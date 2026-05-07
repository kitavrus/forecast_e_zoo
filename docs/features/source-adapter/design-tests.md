# Design Tests — source-adapter

Стратегия тестирования: unit (моки) + integration (`dockertest/v3` + `postgres:18-alpine`).

---

## 1. Уровни тестирования

| Уровень | Что тестируем | Что мокируем | Инструмент |
|---|---|---|---|
| Unit (handler) | Парсинг query, форматирование ответа, маппинг ошибок | Service интерфейс | `testify/mock` |
| Unit (service) | Бизнес-логика loader, snapshot flip, validator engine | Repository, SourceReader, Validator | `testify/mock` |
| Unit (validator) | Каждое правило (negative_qty, future_event_time, fk_exists, ...) | — | table-driven tests |
| Unit (mappers) | ERP DTO ↔ domain | — | golden files |
| Integration (repository) | SQL: UPSERT, SELECT с cursor, partition routing, advisory lock | Реальная PG18 (dockertest) | dockertest + golang-migrate |
| Integration (loader e2e) | Полный pipeline: cron tick → in-memory ERP → PG → snapshot flip | Реальная PG18 + in-memory `SourceReader` | dockertest |
| Integration (HTTP API) | JWT, NDJSON streaming, ETag, error mapping | Реальная PG18 + Fiber `app.Test()` | Fiber test + dockertest |
| Concurrency | Параллельные `POST /admin/loads` → 409, snapshot consistency под нагрузкой | Реальная PG18 + 2 goroutines | dockertest + sync |

## 2. Базовый Suite (dockertest)

```go
// internal/features/data_export/repository/integration_suite_test.go
package repository_test

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"
    "testing"
    "time"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    "github.com/golang-migrate/migrate/v4/source/iofs"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/ory/dockertest/v3"
    "github.com/ory/dockertest/v3/docker"

    "github.com/Kitavrus/e_zoo/internal/features/data_export/sqls"
)

type Suite struct {
    Pool *pgxpool.Pool
    DSN  string
    pgRes *dockertest.Resource
    pool  *dockertest.Pool
}

func NewSuite(t *testing.T) *Suite {
    t.Helper()
    pool, err := dockertest.NewPool("")
    if err != nil { t.Fatalf("dockertest pool: %v", err) }
    pool.MaxWait = 60 * time.Second

    res, err := pool.RunWithOptions(&dockertest.RunOptions{
        Repository: "postgres",
        Tag:        "18-alpine",
        Env: []string{
            "POSTGRES_USER=test",
            "POSTGRES_PASSWORD=test",
            "POSTGRES_DB=adapter_test",
        },
    }, func(c *docker.HostConfig) {
        c.AutoRemove   = true
        c.RestartPolicy = docker.RestartPolicy{Name: "no"}
    })
    if err != nil { t.Fatalf("run pg: %v", err) }

    dsn := fmt.Sprintf("postgres://test:test@%s/adapter_test?sslmode=disable", res.GetHostPort("5432/tcp"))

    var pgxPool *pgxpool.Pool
    if err := pool.Retry(func() error {
        var err error
        pgxPool, err = pgxpool.New(context.Background(), dsn)
        if err != nil { return err }
        return pgxPool.Ping(context.Background())
    }); err != nil {
        t.Fatalf("connect pg: %v", err)
    }

    // golang-migrate с iofs (миграции из go:embed)
    src, err := iofs.New(sqls.Migrations, ".")
    if err != nil { t.Fatalf("iofs: %v", err) }

    db, err := sql.Open("postgres", dsn)
    if err != nil { t.Fatalf("sql.Open: %v", err) }
    drv, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil { t.Fatalf("driver: %v", err) }

    m, err := migrate.NewWithInstance("iofs", src, "postgres", drv)
    if err != nil { t.Fatalf("migrate new: %v", err) }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        t.Fatalf("migrate up: %v", err)
    }

    s := &Suite{Pool: pgxPool, DSN: dsn, pgRes: res, pool: pool}
    t.Cleanup(s.Close)
    return s
}

func (s *Suite) Close() {
    if s.Pool != nil { s.Pool.Close() }
    if s.pool != nil && s.pgRes != nil { _ = s.pool.Purge(s.pgRes) }
}

func (s *Suite) Truncate(t *testing.T, tables ...string) {
    t.Helper()
    for _, tbl := range tables {
        if _, err := s.Pool.Exec(context.Background(),
            fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", tbl)); err != nil {
            t.Fatalf("truncate %s: %v", tbl, err)
        }
    }
}

func TestMain(m *testing.M) {
    os.Exit(m.Run())
}
```

> **Reuse:** один контейнер на test-package (через `TestMain` или `sync.Once`). Каждый тест
> запускает `Truncate` нужных таблиц. Это сильно ускоряет, если тестов много.

## 3. Сценарии integration-тестов

### 3.1. Snapshot atomic flip

```go
func TestSnapshotFlip_AtomicAndConsistent(t *testing.T) {
    suite := NewSuite(t)
    repo := NewRepository(suite.Pool)
    ctx := context.Background()

    // Создаём load1, UPSERT 100 products, flip
    load1 := uuid.New()
    require.NoError(t, repo.InsertLoad(ctx, dto.Load{ID: load1, Status: "running", Source: "manual"}))
    require.NoError(t, repo.UpsertProductsBatch(ctx, load1, gen100Products(load1)))
    require.NoError(t, repo.FlipSnapshotPointer(ctx, load1))

    cur, _ := repo.SelectSnapshotPointer(ctx)
    require.Equal(t, load1, *cur.CurrentLoadID)

    // Создаём load2, UPSERT 50 products, flip
    load2 := uuid.New()
    require.NoError(t, repo.InsertLoad(ctx, dto.Load{ID: load2, Status: "running", Source: "manual"}))
    require.NoError(t, repo.UpsertProductsBatch(ctx, load2, gen50Products(load2)))
    require.NoError(t, repo.FlipSnapshotPointer(ctx, load2))

    cur2, _ := repo.SelectSnapshotPointer(ctx)
    require.Equal(t, load2, *cur2.CurrentLoadID)
    require.Equal(t, load1, *cur2.PreviousLoadID)

    // Чтение по current — видим строки load2
    rows, _ := repo.SelectProducts(ctx, *cur2.CurrentLoadID, time.Time{}, "", 1000)
    require.Len(t, rows, 50)
}
```

### 3.2. Advisory lock — 409 при параллельном POST /admin/loads

```go
func TestAdvisoryLock_ConcurrentLoadFails(t *testing.T) {
    suite := NewSuite(t)
    repo := NewRepository(suite.Pool)
    ctx := context.Background()

    ok1, release1, err := repo.TryAdvisoryLock(ctx, "source-adapter:daily-load")
    require.NoError(t, err)
    require.True(t, ok1)
    defer release1()

    ok2, _, err := repo.TryAdvisoryLock(ctx, "source-adapter:daily-load")
    require.NoError(t, err)
    require.False(t, ok2, "second acquire must return false")
}
```

### 3.3. Cron e2e (in-memory ERP → PG → committed snapshot)

```go
func TestLoaderE2E_HappyPath(t *testing.T) {
    suite := NewSuite(t)
    repo := NewRepository(suite.Pool)
    erp := NewInMemoryReader(seed.Sample())          // products + facts
    val := validators.NewEngine(testRulesYAML())
    snap := snapshot.NewService(repo)
    loader := service.NewLoader(repo, erp, val, snap, slog.Default())

    loadID := uuid.New()
    require.NoError(t, loader.Run(context.Background(), loadID))

    cur, _ := repo.SelectSnapshotPointer(context.Background())
    require.Equal(t, loadID, *cur.CurrentLoadID)

    products, _ := repo.SelectProducts(context.Background(), loadID, time.Time{}, "", 10000)
    require.Equal(t, len(seed.Sample().Products), len(products))
}

func TestLoaderE2E_QualityThresholdExceeded(t *testing.T) {
    // Inject 2 corrupt из 100 строк receipt_line (negative qty) → 2% > 1% → load failed
    // Snapshot НЕ должен flip-нуться
}

func TestLoaderE2E_SupplierStockMissing_Skips(t *testing.T) {
    // Q-009: ERP не отдаёт supplier_stock → load committed успешно, supplier_stock_snapshot пуст
}
```

### 3.4. HTTP API — JWT + NDJSON streaming + ETag

```go
func TestAPI_GetProducts_HappyPath(t *testing.T) {
    suite := NewSuite(t)
    repo := NewRepository(suite.Pool)
    seedSnapshot(t, repo)

    app := buildFiberApp(t, repo)

    req := httptest.NewRequest(http.MethodGet, "/v1/products?since=2026-01-01", nil)
    req.Header.Set("Authorization", "Bearer "+signTestJWT("x-flow-etl"))

    res, err := app.Test(req, -1)
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, res.StatusCode)
    require.Equal(t, "application/x-ndjson", res.Header.Get("Content-Type"))
    require.NotEmpty(t, res.Header.Get("X-Snapshot-Id"))
    require.NotEmpty(t, res.Header.Get("ETag"))

    sc := bufio.NewScanner(res.Body)
    var got int
    for sc.Scan() {
        var p dto.Product
        require.NoError(t, json.Unmarshal(sc.Bytes(), &p))
        got++
    }
    require.Equal(t, 100, got)
}

func TestAPI_NoJWT_401(t *testing.T) { ... }
func TestAPI_WrongRole_403(t *testing.T) { ... } // x-flow-etl → /admin/loads
func TestAPI_NoSnapshot_503(t *testing.T) { ... }
```

### 3.5. Validator engine

```go
func TestValidator_Rules(t *testing.T) {
    cases := []struct {
        name     string
        entity   string
        row      any
        wantSev  string  // critical | soft | ""
    }{
        {"qty_zero_critical",       "receipt_line", dto.ReceiptLine{Qty: 0}, "critical"},
        {"qty_negative_critical",   "receipt_line", dto.ReceiptLine{Qty: -5}, "critical"},
        {"future_event_critical",   "receipt_line", dto.ReceiptLine{EventTime: time.Now().Add(1 * time.Hour)}, "critical"},
        {"future_15min_ok",         "receipt_line", dto.ReceiptLine{EventTime: time.Now().Add(10 * time.Minute)}, ""},
        {"qty_on_hand_negative",    "location_stock_snapshot", dto.LocationStockSnapshot{QtyOnHand: -1}, "critical"},
    }
    eng := validators.NewEngine(testRulesYAML())
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            sev := eng.Validate(c.entity, c.row)
            require.Equal(t, c.wantSev, sev)
        })
    }
}
```

### 3.6. Partition routing

```go
func TestReceiptLine_PartitionRouting(t *testing.T) {
    // Create monthly partitions for 2026-05 and 2026-06
    // INSERT events from both months
    // Verify rows landed in correct partitions (pg_inherits)
}
```

## 4. Что мокируем vs не мокируем

| Что | Решение |
|---|---|
| `pgxpool.Pool` | НЕ мокируем. Используем dockertest на postgres:18-alpine. Реальный SQL гарантирует diff с прод. |
| `SourceReader` | Мокируем через `inmem.Reader` (детерминированные seed-данные). |
| `ERP HTTP-клиент` | В unit — `httptest.NewServer`. В integration — пропускаем (через `inmem.Reader`). |
| `gocron.Scheduler` | Мокируем интерфейсом `Scheduler` (Register, Start, Stop). |
| `ExportsStorage` | В тестах handler/service — мок. В integration `local_fs` — реальный временный каталог `t.TempDir()`. |
| `time.Now` | Через `clock.Clock` (jonboulle/clockwork) для тестов с относительными deadline-ами. |
| `slog.Logger` | НЕ мокируем; используем `slog.New(slog.NewTextHandler(io.Discard))`. |

## 5. Структура каталога с тестами

```
internal/features/data_export/
├── handler/
│   ├── products.go
│   └── products_test.go              # unit (mock service)
├── service/
│   ├── loader.go
│   ├── loader_test.go                # unit (mocks)
│   └── loader_integration_test.go    # integration (dockertest)
├── repository/
│   ├── products.go
│   ├── products_test.go              # integration (dockertest)
│   └── integration_suite_test.go     # Suite, NewSuite, Truncate
├── validators/
│   ├── engine.go
│   ├── engine_test.go                # unit table-driven
│   └── builtin_test.go               # unit per-rule
└── mappers/
    ├── erp_to_domain.go
    └── erp_to_domain_test.go         # unit golden files

# E2E на уровне приложения
test/
├── e2e/
│   ├── api_test.go                   # Fiber app.Test() + dockertest
│   └── concurrency_test.go           # параллельные load-ы
└── seed/
    └── sample.go                     # фикстуры
```

## 6. Покрытие sentinel errors (S-7)

Каждая sentinel-ошибка из [design-errors.md](design-errors.md) §3 должна иметь как минимум один
тест, явно покрывающий её путь. Матрица — обязательный артефакт code-review.

| Sentinel error | Уровень теста | Файл теста (планируемый) |
|---|---|---|
| `ErrBadRequest` | unit (validators) | `internal/features/data_export/handler/validators_test.go` |
| `ErrInvalidCursor` | unit (handler) | `internal/features/data_export/handler/products_test.go` |
| `ErrInvalidQuery` | unit (handler) | `internal/features/data_export/handler/products_test.go` |
| `ErrInvalidExportFormat` | unit (handler) | `internal/features/data_export/handler/exports_test.go` |
| `ErrAuthMissingToken` | unit (middleware) | `internal/middleware/jwt_middleware_test.go` |
| `ErrAuthInvalidToken` | unit (middleware) | `internal/middleware/jwt_middleware_test.go` |
| `ErrAuthForbidden` | unit (middleware) | `internal/middleware/role_test.go` |
| `ErrNotFound` | integration | `internal/features/data_export/repository/repository_test.go` |
| `ErrLoadNotFound` | integration | `internal/features/data_export/repository/loads_test.go` |
| `ErrExportNotFound` | integration | `internal/features/data_export/repository/exports_test.go` |
| `ErrSnapshotNotFound` | integration | `internal/features/data_export/repository/snapshot_test.go` |
| `ErrLoadAlreadyRunning` | integration | `internal/features/data_export/service/admin_loads_integration_test.go` |
| `ErrCannotRetry` | integration | `internal/features/data_export/service/admin_loads_integration_test.go` |
| `ErrEntityValidation` | unit (handler) | `internal/features/data_export/handler/validators_test.go` |
| `ErrSnapshotNotReady` | integration | `internal/features/data_export/service/snapshot_test.go` + `test/e2e/api_test.go` (503 + Retry-After) |
| `ErrDBUnavailable` | integration | `test/e2e/healthz_test.go` (kill PG container) |
| `ErrInternal` | unit (catch-all) | `internal/middleware/error_handler_test.go` |

**Internal-only sentinels** (не доходят до клиента, проверяются по логам/метрикам):

| Sentinel | Уровень теста | Файл |
|---|---|---|
| `ErrERPUnavailable` | integration | `service/loader_integration_test.go` (httptest 5xx) |
| `ErrQualityThresholdExceeded` | integration | `service/loader_integration_test.go` (corrupt rows) |
| `ErrShutdownSignal` | integration | `service/loader_integration_test.go` (ctx cancel) |
| `ErrPartitionMissing` | integration | `repository/facts_test.go` (event_date вне диапазона партиций) |

**Базовое требование для code-review:** ни одна sentinel из `design-errors.md` §3 не остаётся без
теста (минимум — unit). 401/403/409/503 + `Retry-After` дополнительно проверяются на уровне
`test/e2e/api_test.go` через `Fiber app.Test()`.

## 7. CI placeholder

CI **не настраивается в MVP** (Q-012). Однако `Makefile` должен содержать:

```makefile
test-unit:
	go test ./internal/... ./pkg/... -short -race

test-integration:
	go test ./internal/... ./test/e2e/... -race -tags=integration

test-all: test-unit test-integration

lint:
	golangci-lint run --timeout 5m
```

Локальный гейт перед коммитом: `make test-all && make lint`.
