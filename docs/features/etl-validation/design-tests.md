# Design Tests — etl-validation

> Тестовая стратегия — повторяет Модуль 1: testify suite + dockertest + golang-migrate + Fiber `app.Test()`. Общий «корневой» Suite живёт в `pkg/dockertestpkg` (создаёт PG-контейнер, прогоняет миграции, отдаёт `*pgxpool.Pool`).

---

## 1. Уровни тестов

| Уровень | Где | Что проверяем | Время |
|---|---|---|---|
| **Unit** | `*_test.go` рядом с пакетом | чистая логика без I/O: validators, mappers, builtin-checks, transformer-helpers | sec |
| **Integration (DB)** | `internal/features/etl_validation/*_integration_test.go` (build tag `integration`) | repository против реальной PG, через `pkg/dockertestpkg` | min |
| **Integration (HTTP)** | `internal/features/etl_validation/handler/*_test.go` (build tag `integration`) | Fiber + service + repository + dockertest, через `app.Test()` + JWT helpers | min |
| **Pipeline (e2e in-process)** | `internal/etlapp/pipeline_test.go` | полный ETL run против fake source-adapter (httptest) + dockertest PG | min |
| **Contract** | `internal/features/etl_validation/extractor/contract_test.go` | контракт NDJSON клиента к стабам source-adapter API | sec |

---

## 2. Общий Suite через `pkg/dockertestpkg`

Импортируется как библиотека:

```go
package etl_test

import (
    "testing"
    "github.com/Kitavrus/e_zoo/pkg/dockertestpkg"
)

func TestMain(m *testing.M) {
    dockertestpkg.RunSuite(m, dockertestpkg.Options{
        // Прогоняем миграции Модуля 1 + Модуля 2.
        Migrations: []string{
            "internal/features/data_export/sqls/migrations",
            "internal/features/etl_validation/sqls/migrations",
        },
        SeedFn: func(pool *pgxpool.Pool) error {
            // Базовые fixtures: products, locations, suppliers — минимальный валидный seed.
            return seed.LoadGolden(pool, "testdata/seed/baseline.sql")
        },
    })
}
```

> Преимущество: всем интеграционным тестам Модуля 2 передаётся один pool через `dockertestpkg.Pool()`. Между тестами — `TRUNCATE marts.* CASCADE; TRUNCATE public.* CASCADE` через helper.

---

## 3. Golden фикстуры для агрегаций

Строим витрины из детерминированных raw данных, проверяем хеш результата.

```
testdata/golden/
├── demand_history/
│   ├── input/
│   │   ├── stg_receipt_line.json
│   │   ├── stg_promo.json
│   │   ├── stg_store_assortment.json
│   │   └── stg_stock_on_hand.json
│   └── expected.json                  # ожидаемые строки mart_demand_history
├── calculation_input/
│   ├── input/
│   │   ├── stg_order_rule.json
│   │   ├── stg_supply_spec.json
│   │   └── stg_stock_on_hand.json
│   └── expected.json                  # проверка priority order_rule > supply_spec (ADR-024)
├── kpi_daily/
│   ├── input/.../*.json
│   └── expected.json
├── master_current/
│   ├── input/.../*.json
│   └── expected.json
└── supplier_scorecard/
    ├── input/.../*.json
    └── expected.json
```

Шаблон теста:

```go
func TestTransformer_BuildDemandHistory_Golden(t *testing.T) {
    pool := dockertestpkg.Pool()
    ctx := context.Background()
    seed.LoadJSONIntoTemp(t, pool, "testdata/golden/demand_history/input")
    runID, sourceLoadID := uuid.New(), uuid.New()

    tx, _ := pool.Begin(ctx)
    defer tx.Rollback(ctx)
    err := transformer.New(repo).BuildDemandHistory(ctx, tx, runID, sourceLoadID)
    require.NoError(t, err)

    actual := readMart(t, tx, "mart_demand_history")
    expected := loadJSON(t, "testdata/golden/demand_history/expected.json")
    require.Equal(t, expected, actual, "mart_demand_history mismatch")
}
```

---

## 4. Матрица sentinel ↔ test

| Sentinel (errorspkg) | Где возникает | Test файл | HTTP statuses проверяем |
|---|---|---|---|
| `ErrEtlRunAlreadyRunning` | `service.EtlPipeline.TryStart` если advisory lock busy | `service/etl_pipeline_test.go`, `handler/admin_etl_runs_test.go` | 409 |
| `ErrSnapshotNotReady` | `extractor.SnapshotsClient.GetCurrent` если 503 | `extractor/snapshots_test.go`, `service/etl_pipeline_test.go` | 503 |
| `ErrQualityThresholdExceeded` | `service.EtlPipeline.Run` если critical/total > 1% | `service/etl_pipeline_test.go` | run.status='failed' (внутрипроцессно) |
| `ErrSourceUnavailable` | `extractor.Client.Do` после исчерпания retry | `extractor/client_test.go` | 502 (admin endpoint) |
| `ErrEtlRunNotFound` | `repository.EtlRunRepository.GetByID` | `repository/etl_runs_test.go`, `handler/admin_etl_runs_test.go` | 404 |
| `ErrCannotRetryEtl` | `service.EtlRun.Retry` если status NOT IN ('failed','aborted') | `service/etl_run_test.go` | 409 |
| `ErrMartRefreshNotSupported` | `service.MartRefresh.Refresh(name)` если name != mart_supplier_scorecard | `service/mart_refresh_test.go`, `handler/admin_marts_test.go` | 400 |
| `ErrUnauthorized`, `ErrForbidden` (reuse) | JWT/role middleware | `handler/middleware_test.go` | 401/403 |
| `ErrBadRequest` (reuse) | validators | `handler/admin_etl_runs_test.go` | 400 |

---

## 5. Тесты pipeline (e2e in-process)

`internal/etlapp/pipeline_test.go`:

1. Стартуем `httptest.Server`, отдающий стабовые `GET /v1/snapshots/current` и `GET /v1/{entity}` (NDJSON).
2. Поднимаем dockertest PG, накатываем миграции Модуля 1 + Модуля 2, заливаем seed.
3. Создаём `etlapp.App` с подменённым `extractor.client.baseURL` на адрес `httptest.Server`.
4. Триггерим pipeline через `app.RunEtlOnce(ctx)`.
5. Проверяем:
   - `marts.etl_runs` row `status='committed'`.
   - 5 mart-таблиц непустые.
   - `marts_summary` JSONB содержит `rows` для каждой mart.
   - `reject_log` пуст (или содержит ровно ожидаемые soft-violations).
6. Negative: меняем стабы так, чтобы было > 1% critical-rows → проверяем `status='failed'` и витрины НЕ изменились.
7. Negative: 503 от стаба — проверяем `status='aborted'` и метрику `etl_skipped_no_snapshot_total`.

---

## 6. Тесты extractor

| Сценарий | Файл | Ассерт |
|---|---|---|
| 200 OK + ETag → возвращаем reader | `client_test.go` | reader.Next() поэлементно |
| 304 Not Modified → возвращаем «no changes» | `entities_test.go` | специальный `ErrNotModified` (или nil reader) |
| 503 snapshot_not_ready | `snapshots_test.go` | `ErrSnapshotNotReady` |
| 5xx + retry с backoff cap 30s | `client_test.go` | проверяем число попыток + интервалы (через fakeclock) |
| JWT signing HS256 | `token_source_test.go` | подписан корректно, claims `role=x-flow-etl` |

---

## 7. Тесты validation builtin-checks

`validation/*_test.go`:

| Builtin | Сценарии |
|---|---|
| `fk_exists` | (a) row.fk in ref → ok; (b) row.fk NOT in ref → critical violation |
| `unique_business_key` | (a) уникальные → ok; (b) дубль → critical |
| `aggregate_sum_matches` | (a) разница ≤ tolerance → ok; (b) > tolerance → critical |
| `referential_integrity` | (a) row.applicable_rule_id в master_current → ok; (b) нет → critical |
| `null_required_field` | (a) поле not null → ok; (b) null → soft (или critical в зависимости от severity) |

---

## 8. Тесты scheduler / advisory lock

| Сценарий | Файл | Ассерт |
|---|---|---|
| TryLock первый раз → acquired=true | `scheduler/lock_test.go` | возвращён release-callback |
| TryLock второй раз пока не release → acquired=false | `scheduler/lock_test.go` | acquired=false |
| Stale detect: run started_at < now()-1h, status='running' | `scheduler/stale_test.go` | DetectStale возвращает runID |

---

## 9. Тесты handler (Fiber app.Test)

```go
func TestHandler_AdminEtlRuns_TriggerOK(t *testing.T) {
    app := newTestApp(t) // poll dockertest pool, регистрирует router
    token := newJWT(t, "admin-cli")
    req := httptest.NewRequest("POST", "/admin/etl-runs", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, _ := app.Test(req)
    require.Equal(t, 202, resp.StatusCode)
    // ассертим, что в БД появилась etl_runs row со status='running'
}

func TestHandler_AdminEtlRuns_RetryNotFound(t *testing.T) { ... } // 404
func TestHandler_AdminEtlRuns_RetryConflict(t *testing.T) { ... } // 409 — running
func TestHandler_AdminMarts_NotSupported(t *testing.T) { ... }    // 400
func TestHandler_AdminRejectLog_Pagination(t *testing.T) { ... }  // 200 + cursor
```

---

## 10. CI — make-таргеты

```
make test-unit         # go test ./... -short
make test-integration  # go test ./... -tags=integration
make test-golden       # выделенный набор golden-тестов
make test-all          # unit + integration + golden
```

В CI сначала `test-unit`, затем при наличии Docker — `test-integration` + `test-golden`. dockertest пропускается, если `DOCKER_HOST` не задан (skip с сообщением, не fail).
