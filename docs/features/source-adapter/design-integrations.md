# Design Integrations — source-adapter

Внешние интеграции и обоснование выбора инструментов.

---

## 1. HTTP-клиент к ERP

### Выбор: `net/http` (стандартная библиотека) + кастомный retry-middleware

**Почему не resty/req/etc.:**
- Стандартная библиотека достаточна для pull (REST/SOAP). Меньше зависимостей.
- Тонкая прослойка `transport.RoundTripper` для retry/backoff/auth/тайминга — проще, чем настройка
  чужого фреймворка с overrides.
- SOAP — обычный POST с XML-телом; не нужен fancy DSL.

### Структура

```go
// internal/features/data_export/service/http_client.go

type ERPHTTPClient struct {
    base        string
    httpClient  *http.Client
    auth        SourceAuth
    retryMax    int
    backoffCap  time.Duration
    log         *slog.Logger
}

func NewERPHTTPClient(cfg *config.Config, auth SourceAuth, log *slog.Logger) *ERPHTTPClient {
    transport := &retryTransport{
        base:       http.DefaultTransport,
        retryMax:   cfg.ERPRetryMax,           // default 3
        backoffCap: cfg.ERPRetryBackoffCap,    // default 30s
        jitterPct:  10,
        log:        log,
    }
    return &ERPHTTPClient{
        base: cfg.ERPBaseURL,
        httpClient: &http.Client{
            Timeout:   cfg.ERPHTTPTimeout,     // default 30s per attempt
            Transport: transport,
        },
        auth: auth,
        log:  log,
    }
}

func (c *ERPHTTPClient) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
    if err != nil { return nil, err }
    if err := c.auth.Apply(req); err != nil { return nil, err }
    req.Header.Set("Accept", "application/json")
    return c.httpClient.Do(req)
}
```

### Retry политика (ADR-007 / Q-007)

| Параметр | Значение | ENV |
|---|---|---|
| Max retries | 3 | `ERP_RETRY_MAX` |
| Backoff base | 1s | hardcoded |
| Backoff cap | 30s | `ERP_RETRY_BACKOFF_CAP` |
| Jitter | 10% | hardcoded |
| Retry на коды | 408, 429, 500, 502, 503, 504, network errors | hardcoded |
| НЕ retry | 4xx (кроме 408/429), context.Canceled | hardcoded |

Алгоритм: `delay = min(cap, base * 2^attempt) ± jitter`.

### Auth варианты

```go
type SourceAuth interface {
    Apply(req *http.Request) error
}

func NewSourceAuth(cfg *config.Config) SourceAuth {
    switch cfg.ERPAuthMode {
    case "bearer":  return &bearerAuth{tokenSource: oauth2ClientCredentials(cfg)}
    case "mtls":    return &mtlsAuth{certPath: cfg.ERPMTLSCertPath, keyPath: cfg.ERPMTLSKeyPath}
    case "apikey":  return &apiKeyAuth{header: "X-API-Key", value: cfg.ERPAPIKey}
    case "none":    fallthrough
    default:        return &noAuth{}
    }
}
```

> **Q-001 / Q-002 эскалация:** конкретный режим зависит от ERP клиента. До решения IT E-Zoo —
> используем `noAuth` + in-memory backend для разработки.
>
> **Fallback:** если до DD-MM-YYYY (дата фиксируется PM X-Flow) нет ответа от ИБ E-Zoo (Q-001) /
> IT E-Zoo (Q-002) — реализуем `erp_e_zoo_reader` как in-memory stub с фиктивными данными для
> интеграционных тестов; HTTP-клиент к ERP остаётся за интерфейсом `SourceReader` + `SourceAuth`,
> готовый к подмене на реальную реализацию без изменений в loader/handler.

## 2. Логгер: `log/slog` (stdlib)

**Почему slog, а не zerolog/zap:**
- Стандартная библиотека (Go 1.21+). Нет внешних зависимостей.
- JSON handler из коробки.
- Достаточная производительность для 50–100 RPS API + cron-load.
- Легко мокируется в тестах (`io.Discard`).

### Структура логирования

```go
log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level:     cfg.LogLevel,
    AddSource: false,
}))
```

### Контекстные поля (всегда)

| Ключ | Источник | Пример |
|---|---|---|
| `request_id` | middleware/RequestID, X-Request-Id header | `7e1b...` |
| `load_id` | loader pipeline | `8c1a...` |
| `entity` | loader pipeline | `products` |
| `status_code` | logger middleware | `200` |
| `duration_ms` | logger middleware | `123` |
| `requester` | JWT issuer/sub claim | `x-flow-etl` |

### Уровни

| Уровень | Когда |
|---|---|
| ERROR | load failed, panic recovered, PG unavailable |
| WARN  | severity=soft в reject_log, retry attempt, stale load aborted |
| INFO  | load started/committed, snapshot flipped, scheduler tick |
| DEBUG | per-row trace, SQL params (только в dev) |

### Чувствительные данные

- **Никогда не логируем:** JWT secret, ERP API-key/cert, payment_terms (опционально маскируем).
- **Маскируем PII:** `loyalty_hash` (уже хеш), но НЕ raw client_id.
- **Никогда не логируем целиком raw ERP response** — только ID + код ошибки.

## 3. Метрики: Prometheus

### Endpoint: `GET /metrics` (без JWT)

### Метрики (registry)

| Метрика | Тип | Лейблы | Назначение |
|---|---|---|---|
| `source_adapter_load_started_total` | Counter | `source` | Сколько load-ов стартовало |
| `source_adapter_load_success_total` | Counter | — | Успешные load-ы |
| `source_adapter_load_failed_total` | Counter | `reason` (erp_unavailable, quality_threshold_exceeded, shutdown_signal, internal) | Фейлы по причинам |
| `source_adapter_load_skipped_total` | Counter | — | Cron tick пропустил из-за advisory lock |
| `source_adapter_load_duration_seconds` | Histogram | `status` | Длительность load-а |
| `source_adapter_entity_rows_processed` | Counter | `entity`, `result` (upserted, rejected_critical, rejected_soft) | Per-entity счёт строк |
| `source_adapter_validation_soft_total` | Counter | `entity`, `rule_id` | Soft-нарушения по правилам |
| `source_adapter_snapshot_not_ready_total` | Counter | — | 503 на /v1/* |
| `source_adapter_http_requests_total` | Counter | `endpoint`, `method`, `status_code`, `requester` | API requests |
| `source_adapter_http_request_duration_seconds` | Histogram | `endpoint`, `method` | API latency |
| `source_adapter_erp_request_duration_seconds` | Histogram | `entity`, `outcome` | ERP HTTP latency |
| `source_adapter_erp_retries_total` | Counter | `entity`, `attempt` | Сколько retry-попыток |
| `source_adapter_export_size_bytes` | Histogram | `entity`, `format` | Размер async-exports |
| `source_adapter_export_duration_seconds` | Histogram | `entity`, `format` | Длительность async-export |
| `source_adapter_db_pool_inuse` | Gauge | — | Использование pgxpool |

### Library

`github.com/prometheus/client_golang/prometheus` + `promhttp.Handler()`. Registry — глобальный
`prometheus.DefaultRegisterer` (добавляем коллекторы в `init()` каждого пакета через `MustRegister`).

## 4. Локальная FS для async-exports

### Layout

```
/var/exports/
├── 7e1b1a30-.../
│   ├── data.parquet
│   └── meta.json   (entity, snapshot_id, format, requester, created_at)
```

или плоско:

```
/var/exports/{id}.parquet
/var/exports/{id}.meta.json
```

> Решение: **плоский layout** — проще для Fiber static и cleanup.

### Cleanup

Background-job в самом сервисе (cron каждый час):

1. `SELECT id, path FROM exports WHERE status IN ('ready','failed') AND created_at < now() - interval '24 hours'`.
2. `os.Remove(path)`.
3. `DELETE FROM exports WHERE id = ANY(...)`.

### Fiber serving

`/v1/exports/:id/download` → handler делает:
1. `SELECT status, path FROM exports WHERE id=$1`.
2. Если `status != 'ready'` → 404.
3. `c.SendFile(path, fiber.SendFile{Compress: false, ByteRange: true})`.

### Замена на S3 (будущее)

`ExportsStorage` интерфейс уже даёт абстракцию. S3-импл:

```go
type s3Storage struct {
    bucket  string
    prefix  string
    client  *s3.Client
}

func (s *s3Storage) Write(ctx, id, format, src) (path string, size int64, err)  { ... }
func (s *s3Storage) Open(ctx, id) (io.ReadCloser, error)                         { ... }
```

Handler меняет behaviour: вместо `c.SendFile` отдаёт `signed_url` в JSON. Решение об активации
S3 — Q-013, эскалация.

## 5. Cron: `github.com/go-co-op/gocron/v2`

**Почему gocron:**
- Активная поддержка, semver discipline.
- Встроенная поддержка cron-выражений + IANA TZ.
- Singleton mode (защита от двойного запуска внутри процесса) дополняет advisory lock.
- Простой `Job` API без overengineering.

```go
sched, err := gocron.NewScheduler(gocron.WithLocation(loc)) // loc = Europe/Kyiv
_, err = sched.NewJob(
    gocron.CronJob(cfg.SourceAdapterCron, false),
    gocron.NewTask(func() { loader.Run(ctx, uuid.New()) }),
    gocron.WithSingletonMode(gocron.LimitModeReschedule),
)
sched.Start()
```

## 6. JWT: `github.com/golang-jwt/jwt/v5`

- HS256 default; RS256 если задан `JWT_PUBLIC_KEY_PATH`.
- Claims: `iss` (issuer/role), `sub` (caller id), `exp`, `iat`.
- Verify в middleware, claims кладём в `c.Locals("jwt_claims", claims)`.
- Для тестов — генератор `signTestJWT(role string) string` в `test/helpers`.

## 7. Структурированная конфигурация: `kelseyhightower/envconfig`

- Один tag-based парсинг env-vars в struct.
- Default-ы рядом с типами.
- Required-валидация на старте.
- Полный список ENV см. [design-infrastructure.md](design-infrastructure.md) §2.

## 8. Зависимости (go.mod, окончательный список)

```
require (
    github.com/gofiber/fiber/v3                v3.0.0   // или последний v3 stable
    github.com/jackc/pgx/v5                    v5.x
    github.com/golang-migrate/migrate/v4       v4.x
    github.com/go-co-op/gocron/v2              v2.x
    github.com/golang-jwt/jwt/v5               v5.x
    github.com/google/uuid                     v1.x
    github.com/kelseyhightower/envconfig       v1.x
    github.com/prometheus/client_golang        v1.x
    gopkg.in/yaml.v3                           v3.x
    github.com/jonboulle/clockwork             v0.x  // тесты с временем

    // tests
    github.com/ory/dockertest/v3               v3.x
    github.com/stretchr/testify                v1.x
)
```

Никаких ORM (sqlx/gorm/ent), никакого resty, никакого Redis/Kafka.
