# Design Infrastructure — source-adapter

Локальная инфраструктура: docker-compose, Dockerfile, env-vars, healthcheck, метрики, логи.

> **CI/CD не входит в MVP** — зафиксировано как Q-012 (см. [design-adr.md](design-adr.md) ADR-012).

---

## 1. docker-compose.yml

```yaml
# docker-compose.yml
version: "3.9"

services:
  postgres:
    image: postgres:18-alpine
    container_name: ezoo_pg
    environment:
      POSTGRES_USER: adapter
      POSTGRES_PASSWORD: adapter
      POSTGRES_DB: source_adapter
      # PG18 tuning
      POSTGRES_INITDB_ARGS: "--locale=C --encoding=UTF8"
    volumes:
      - pg_data:/var/lib/postgresql/data
      - ./infra/pg/postgresql.conf:/etc/postgresql/postgresql.conf:ro
    command: ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U adapter -d source_adapter"]
      interval: 5s
      timeout: 3s
      retries: 10

  migrate:
    image: migrate/migrate:v4
    profiles: ["migrate"]
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./internal/features/data_export/sqls/migrations:/migrations:ro
    command: [
      "-path=/migrations",
      "-database=postgres://adapter:adapter@postgres:5432/source_adapter?sslmode=disable",
      "up"
    ]

  source-adapter:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: ezoo_source_adapter
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DB_DSN: "postgres://adapter:adapter@postgres:5432/source_adapter?sslmode=disable&pool_max_conns=20"
      HTTP_ADDR: ":8080"
      LOG_LEVEL: INFO
      JWT_ALG: HS256
      JWT_SECRET: "dev-secret-change-in-prod"
      JWT_ADMIN_ROLE: "admin-cli"
      JWT_READ_ROLES: "x-flow-etl,it-read"
      SOURCE_ADAPTER_CRON_SCHEDULE: "0 2 * * *"
      SOURCE_ADAPTER_TZ: "Europe/Kyiv"
      QUALITY_THRESHOLD_PCT: "1.0"
      ERP_BASE_URL: ""              # пусто = in-memory dev backend
      ERP_AUTH_MODE: "none"
      ERP_HTTP_TIMEOUT: "30s"
      ERP_RETRY_MAX: "3"
      ERP_RETRY_BACKOFF_CAP: "30s"
      EXPORTS_BASE_DIR: "/var/exports"
      EXPORTS_RETENTION: "24h"
      EXPORTS_INLINE_MAX_MB: "50"
      AUDIT_RETENTION: "2160h"
      REJECT_LOG_RETENTION: "2160h"
      VALIDATION_RULES_PATH: "/etc/source-adapter/validation_rules.yaml"
      MASTER_TRACKED_FIELDS_PATH: "/etc/source-adapter/master_tracked_fields.yaml"
      APP_ENV: "dev"
      SOURCE_ADAPTER_STALE_LOAD_TIMEOUT: "1h"
    ports:
      - "8080:8080"
    volumes:
      - ./config/validation_rules.yaml:/etc/source-adapter/validation_rules.yaml:ro
      - ./config/master_tracked_fields.yaml:/etc/source-adapter/master_tracked_fields.yaml:ro
      - exports_data:/var/exports
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/v1/healthz"]
      interval: 10s
      timeout: 3s
      retries: 5

  prometheus:
    image: prom/prometheus:latest
    container_name: ezoo_prometheus
    profiles: ["observability"]
    volumes:
      - ./infra/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports:
      - "9090:9090"

volumes:
  pg_data:
  exports_data:
```

**Использование:**

```bash
# 1. Запустить PG
docker compose up -d postgres

# 2. Применить миграции (явный шаг, не auto-apply)
docker compose run --rm migrate

# 3. Запустить адаптер
docker compose up -d source-adapter

# 4. Опционально — Prometheus
docker compose --profile observability up -d prometheus
```

## 2. Dockerfile (multi-stage)

```dockerfile
# syntax=docker/dockerfile:1.6

# ---------- builder ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# go:embed подхватит миграции и SQL автоматически — отдельной copy не требуется
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git rev-parse --short HEAD)" \
    -o /out/source-adapter ./cmd/source-adapter

# ---------- runner ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/source-adapter /app/source-adapter
# wget для healthcheck
COPY --from=busybox:1.36-uclibc /bin/wget /usr/bin/wget

USER nonroot:nonroot

EXPOSE 8080

VOLUME ["/var/exports"]

ENTRYPOINT ["/app/source-adapter"]
```

> Distroless даёт минимальный attack surface. Если healthcheck wget не работает — используем
> `gcr.io/distroless/base-debian12` (есть `wget` встроенный).

## 3. Полный список ENV-vars

| ENV | Default | Required | Описание |
|---|---|---|---|
| `HTTP_ADDR` | `:8080` | no | Bind address для Fiber |
| `DB_DSN` | — | **yes** | Postgres DSN |
| `DB_MAX_CONNS` | `20` | no | pgxpool max |
| `DB_MIN_CONNS` | `2` | no | pgxpool min |
| `LOG_LEVEL` | `INFO` | no | DEBUG \| INFO \| WARN \| ERROR |
| `JWT_ALG` | `HS256` | no | HS256 \| RS256 |
| `JWT_SECRET` | — | **yes (HS256)** | HS256 секрет |
| `JWT_PUBLIC_KEY_PATH` | `""` | no | путь к PEM (RS256) |
| `JWT_ADMIN_ROLE` | `admin-cli` | no | claim `iss` или `role` для /admin/* |
| `JWT_READ_ROLES` | `x-flow-etl,it-read` | no | comma-separated роли для /v1/* |
| `SOURCE_ADAPTER_CRON_SCHEDULE` | `0 2 * * *` | no | cron-выражение |
| `SOURCE_ADAPTER_TZ` | `Europe/Kyiv` | no | IANA TZ |
| `QUALITY_THRESHOLD_PCT` | `1.0` | no | % failed → fail load |
| `ERP_BASE_URL` | `""` | no | пусто = in-memory backend (Q-002) |
| `ERP_AUTH_MODE` | `none` | no | none \| bearer \| mtls \| apikey |
| `ERP_API_KEY` | `""` | no | для apikey |
| `ERP_OAUTH_TOKEN_URL` | `""` | no | для bearer |
| `ERP_OAUTH_CLIENT_ID` | `""` | no | для bearer |
| `ERP_OAUTH_CLIENT_SECRET` | `""` | no | для bearer |
| `ERP_MTLS_CERT_PATH` | `""` | no | для mtls |
| `ERP_MTLS_KEY_PATH` | `""` | no | для mtls |
| `ERP_HTTP_TIMEOUT` | `30s` | no | per-attempt timeout |
| `ERP_RETRY_MAX` | `3` | no | max attempts |
| `ERP_RETRY_BACKOFF_CAP` | `30s` | no | exp backoff cap |
| `EXPORTS_BASE_DIR` | `/var/exports` | no | local FS для async exports |
| `EXPORTS_RETENTION` | `24h` | no | через сколько cleanup |
| `EXPORTS_INLINE_MAX_MB` | `50` | no | граница inline vs async |
| `AUDIT_RETENTION` | `2160h` (90d) | no | retention audit_access |
| `REJECT_LOG_RETENTION` | `2160h` (90d) | no | retention reject_log |
| `VALIDATION_RULES_PATH` | `/etc/source-adapter/validation_rules.yaml` | no | путь к YAML правил |
| `MASTER_TRACKED_FIELDS_PATH` | `/etc/source-adapter/master_tracked_fields.yaml` | no | путь к YAML tracked fields |
| `SOURCE_ADAPTER_STALE_LOAD_TIMEOUT` | `1h` | no | через сколько running-load → aborted (ADR-015 / Q-015) |
| `APP_ENV` | `dev` | no | dev \| prod (влияет на startup banner Fiber) |
| `PROMETHEUS_PATH` | `/metrics` | no | endpoint для Prometheus scrape |

## 4. Healthcheck

### `GET /v1/healthz` (public, без JWT)

```json
{
  "status": "ok",
  "version": "1.0.0+a3f8c1",
  "db": "up",
  "current_snapshot_id": "8c1a4d2f-...",
  "snapshot_committed_at": "2026-05-07T03:15:42Z"
}
```

При проблемах с БД — `503` + `{status: "degraded", db: "down"}`.

При отсутствии snapshot — `200` + `current_snapshot_id: null` (это не fail, просто первый запуск
ещё не отработал).

### Docker healthcheck

```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:8080/v1/healthz"]
  interval: 10s
  timeout: 3s
  retries: 5
  start_period: 30s
```

## 5. Prometheus конфиг

```yaml
# infra/prometheus/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: source-adapter
    static_configs:
      - targets: ["source-adapter:8080"]
    metrics_path: /metrics
```

Полный список метрик — [design-integrations.md](design-integrations.md) §3.

## 6. PostgreSQL конфиг (опционально)

```ini
# infra/pg/postgresql.conf — рекомендуемый baseline для PG18
listen_addresses = '*'
max_connections = 100

shared_buffers = 256MB
work_mem = 16MB
maintenance_work_mem = 64MB
effective_cache_size = 1GB

wal_compression = on
wal_level = replica
checkpoint_completion_target = 0.9

# logging для dev
log_min_duration_statement = 200ms
log_lock_waits = on
log_temp_files = 10MB
log_line_prefix = '%t [%p] %u@%d '
```

## 7. Политика логирования

| Среда | Куда | Формат | Уровень default |
|---|---|---|---|
| Dev (compose) | `stdout` контейнера | JSON (slog JSON handler) | `INFO` |
| Prod | `stdout` контейнера → внешний sink (Loki/CloudWatch — Q-012) | JSON | `INFO` |

Шаблон строки:

```json
{"time":"2026-05-07T02:00:01Z","level":"INFO","msg":"daily-load started","load_id":"8c1a...","source":"erp_e_zoo"}
```

Контекст (`request_id`, `load_id`, `entity`) добавляется через `slog.With(...)`.

## 8. Файлы ENV-шаблонов

```
.env.dev          # для compose, безопасные defaults
.env.example      # шаблон со всеми переменными для prod-копии
```

`.gitignore` исключает `.env`, `.env.prod`, `.env.local`.

## 9. Метрики мониторинга (рекомендуемые алерты)

| Алерт | Условие | Severity |
|---|---|---|
| Load failed (репеатедли) | `increase(source_adapter_load_failed_total[24h]) > 0` | critical |
| Snapshot stale | `time() - source_adapter_snapshot_committed_at_seconds > 86400 * 1.5` | critical |
| ERP latency | `histogram_quantile(0.95, sum(rate(source_adapter_erp_request_duration_seconds_bucket[5m])) by (le)) > 10` | warning |
| 5xx surge | `rate(source_adapter_http_requests_total{status_code=~"5.."}[5m]) > 0.1` | warning |
| DB pool exhausted | `source_adapter_db_pool_inuse / source_adapter_db_pool_max > 0.9` | warning |
| Stale running load | `source_adapter_loads_running_oldest_seconds > 3600` | warning |

> Конкретный алертинг (Alertmanager, PagerDuty, Telegram) — Q-012, эскалация IT E-Zoo.

## 10. CI placeholder

CI **не настраивается**. В `Makefile`:

```makefile
.PHONY: build test test-unit test-integration lint migrate-up migrate-down docker-build run

build:
	go build -o bin/source-adapter ./cmd/source-adapter

test-unit:
	go test ./internal/... ./pkg/... -short -race

test-integration:
	go test ./internal/... ./test/e2e/... -race -tags=integration

test: test-unit test-integration

lint:
	golangci-lint run --timeout 5m

migrate-up:
	migrate -path internal/features/data_export/sqls/migrations \
	        -database "$$DB_DSN" up

migrate-down:
	migrate -path internal/features/data_export/sqls/migrations \
	        -database "$$DB_DSN" down 1

docker-build:
	docker build -t source-adapter:dev .

run: docker-build
	docker compose up -d
```
