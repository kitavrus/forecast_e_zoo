# Design Infrastructure — etl-validation

> Все артефакты деплоя — `Dockerfile.etl` (или multi-target `Dockerfile`), новый сервис `etl` в `docker-compose*.yml`, env-vars (префикс `ETL_*`), make-таргеты `migrate-up-etl`/`migrate-down-etl`. Grafana дашборд X-Flow расширяется метриками `etl_*`.

---

## 1. Dockerfile

### Вариант A — отдельный `Dockerfile.etl` (рекомендуем)

```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /out/etl ./cmd/etl

FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/etl /app/etl
COPY configs/etl_validation_rules.yaml /app/configs/etl_validation_rules.yaml
ENV TZ=Europe/Kyiv
EXPOSE 8081 9091
ENTRYPOINT ["/app/etl"]
```

### Вариант B — multi-target `Dockerfile` (один Dockerfile для двух бинарей)

```dockerfile
# syntax=docker/dockerfile:1.7
ARG TARGET=source-adapter

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /out/${TARGET} ./cmd/${TARGET}

FROM alpine:3.20 AS runtime
ARG TARGET
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/${TARGET} /app/app
COPY configs/etl_validation_rules.yaml /app/configs/etl_validation_rules.yaml
ENV TZ=Europe/Kyiv
ENTRYPOINT ["/app/app"]
```

> Решение между A/B — мета-ADR-101. Default: вариант A (отдельный Dockerfile).

---

## 2. Env-vars (полный список)

| ENV | Default | Описание |
|---|---|---|
| `ETL_HTTP_ADDR` | `:8081` | HTTP API listen |
| `ETL_METRICS_ADDR` | `:9091` | Prometheus exposition |
| `ETL_LOG_LEVEL` | `info` | slog level |
| `ETL_DB_DSN` | (required) | PG connection string (либо общий `DB_DSN`, см. ADR-106) |
| `ETL_API_BASE_URL` | `http://source-adapter:8080` | URL source-adapter API |
| `ETL_HTTP_TIMEOUT` | `30s` | timeout HTTP-клиента |
| `ETL_RETRY_MAX` | `5` | количество retry для extractor |
| `ETL_RETRY_BACKOFF_CAP` | `30s` | cap exponential backoff |
| `ETL_CRON_SCHEDULE` | `30 2 * * *` | cron expr (06 fields with seconds NOT used) |
| `ETL_TZ` | `Europe/Kyiv` | таймзона scheduler-а |
| `ETL_STALE_RUN_TIMEOUT` | `1h` | порог abort для зависших running runs |
| `ETL_QUALITY_THRESHOLD_PCT` | `1.0` | порог fail (lines_failed/lines_total) |
| `ETL_VALIDATION_RULES_PATH` | `/app/configs/etl_validation_rules.yaml` | путь к YAML-правилам |
| `ETL_JWT_ALG` | `HS256` | алгоритм подписи (HS256 \| RS256) |
| `ETL_JWT_SIGNING_KEY` | — | ключ для HS256 (либо ниже два) |
| `ETL_JWT_PUBLIC_KEY_PATH` | — | для RS256 verify |
| `ETL_JWT_PRIVATE_KEY_PATH` | — | для RS256 sign |
| `ETL_JWT_ISSUER` | `x-flow-etl` | `iss` claim |
| `ETL_JWT_AUDIENCE` | `source-adapter` | `aud` claim |

> Required: `ETL_DB_DSN` + (`ETL_JWT_SIGNING_KEY` ИЛИ оба `ETL_JWT_*_KEY_PATH`).

---

## 3. docker-compose

`docker-compose.yml` (production-like локальная сборка):

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: e_zoo
      POSTGRES_USER: e_zoo
      POSTGRES_PASSWORD: e_zoo
    volumes: ["pgdata:/var/lib/postgresql/data"]
    ports: ["5432:5432"]

  source-adapter:
    build: { context: ., dockerfile: Dockerfile }
    environment:
      DB_DSN: postgres://e_zoo:e_zoo@postgres:5432/e_zoo?sslmode=disable
      JWT_SIGNING_KEY: ${SHARED_JWT_KEY}
    depends_on: [postgres]
    ports: ["8080:8080", "9090:9090"]

  etl:
    build: { context: ., dockerfile: Dockerfile.etl }
    environment:
      ETL_DB_DSN: postgres://e_zoo:e_zoo@postgres:5432/e_zoo?sslmode=disable
      ETL_API_BASE_URL: http://source-adapter:8080
      ETL_JWT_SIGNING_KEY: ${SHARED_JWT_KEY}
      ETL_CRON_SCHEDULE: "30 2 * * *"
      ETL_TZ: Europe/Kyiv
      ETL_QUALITY_THRESHOLD_PCT: "1.0"
    depends_on:
      postgres: { condition: service_started }
      source-adapter: { condition: service_started }
    ports: ["8081:8081", "9091:9091"]

  prometheus:
    image: prom/prometheus:v2.55.0
    volumes: ["./deploy/prometheus.yml:/etc/prometheus/prometheus.yml:ro"]
    ports: ["9099:9090"]

  grafana:
    image: grafana/grafana:11.3.0
    volumes: ["./deploy/grafana/dashboards:/var/lib/grafana/dashboards:ro"]
    ports: ["3000:3000"]

volumes:
  pgdata:
```

> Дев-вариант (`docker-compose.dev.yml`) пробрасывает source-код через bind-mount + air для hot-reload (out of scope этого design).

---

## 4. Makefile (новые таргеты)

```makefile
ETL_DSN ?= postgres://e_zoo:e_zoo@localhost:5432/e_zoo?sslmode=disable
MIG_PATH = internal/features/etl_validation/sqls/migrations

migrate-up-etl:
	migrate -database "$(ETL_DSN)" -path $(MIG_PATH) up

migrate-down-etl:
	migrate -database "$(ETL_DSN)" -path $(MIG_PATH) down 1

migrate-version-etl:
	migrate -database "$(ETL_DSN)" -path $(MIG_PATH) version

build-etl:
	go build -o bin/etl ./cmd/etl

run-etl:
	go run ./cmd/etl

docker-build-etl:
	docker build -f Dockerfile.etl -t e_zoo/etl:dev .

test-integration-etl:
	go test ./internal/features/etl_validation/... -tags=integration

# Существующие (Модуль 1) — оставляем как есть.
# migrate-up / migrate-down — для source-adapter.
```

---

## 5. Prometheus config (расширение)

`deploy/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: source-adapter
    static_configs: [{ targets: ["source-adapter:9090"] }]
  - job_name: etl
    static_configs: [{ targets: ["etl:9091"] }]
```

---

## 6. Grafana дашборд (расширение)

JSON-дашборд X-Flow получает новый row «X-Flow ETL»:

| Panel | Query (PromQL) |
|---|---|
| ETL run duration p95 (7d) | `histogram_quantile(0.95, sum by (le) (rate(etl_run_duration_seconds_bucket[7d])))` |
| ETL success rate (1d) | `sum(rate(etl_run_success_total[1d])) / (sum(rate(etl_run_success_total[1d])) + sum(rate(etl_run_failed_total[1d])))` |
| ETL lag seconds | `etl_lag_seconds` |
| Lines failed % | `sum(rate(etl_lines_failed_total[1d])) / sum(rate(etl_lines_processed_total[1d]))` |
| Mart rows total | `mart_rows_total` (per mart) |

Алерты:
- `etl_lag_seconds > 4 * 3600` за 5m → critical.
- `up{job="etl"} == 0` 5m → critical.
- `increase(etl_run_failed_total{reason="quality"}[1d]) > 0` → warning.

---

## 7. CI / CD (out of scope MVP, но фиксируем форму)

- Lint: `golangci-lint run ./...`
- Test: `make test-unit`, при наличии Docker — `make test-integration-etl`
- Build: docker buildx, push в registry.
- Deploy: уже задача IT E-Zoo (отложено до фазы pre-prod).

---

## 8. Backup / DR

- БД одна (марты + raw в одном кластере PG18) — backup существующего PG покрывает marts.
- Recovery: восстановить PG → запустить ETL вручную через `POST /admin/etl-runs` (re-build из source-adapter raw данных).

---

## 9. Network model

| От | К | Порт | Зачем |
|---|---|---|---|
| etl | source-adapter | 8080 | extractor: GET /v1/* |
| etl | postgres | 5432 | pgxpool |
| prometheus | etl | 9091 | scrape /metrics |
| devops (host) | etl | 8081 | admin API |
| etl | (egress) | — | нет внешних HTTP-вызовов кроме source-adapter |
