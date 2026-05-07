# Design — etl-validation (Модуль 2 «X-Flow ETL»)

> **Источники истины:**
> - `docs/features/etl-validation/research/output.md`
> - `docs/features/etl-validation/spec-interview/output.md` (Q-001..Q-025, все Принято или Отложено)
> - `docs/tmp/data-marts/contract-2026-05-06.md` (контракт 5 mart-таблиц)
> - Паттерн Модуля 1 — `docs/features/source-adapter/design*.md`
>
> Каждое решение этого Design имеет ADR в `design-adr.md` (1:1 с Q-NNN или мета-ADR-100+).

---

## 0. Резюме

Модуль 2 строится поверх готового Модуля 1 (`source-adapter`). Это отдельный binary `cmd/etl/main.go`, deploy-unit и docker-compose-сервис. Он:

1. По cron `02:30 Europe/Kyiv` (configurable, ADR-003) делает PG advisory lock на ключ `etl-run`.
2. Создаёт запись `marts.etl_runs(id=uuidv4, status='running', source_load_id=NULL)`.
3. Читает `GET /v1/snapshots/current` API source-adapter (JWT role `x-flow-etl`) — фиксирует `current_load_id` как `source_load_id` на весь run (atomic read, ADR-005).
4. Скачивает все master/fact/доп. сущности через `GET /v1/{entity}?snapshot=<load_id>` (NDJSON streaming, ETag, JWT) во временные staging-таблицы внутри транзакции ETL.
5. Прогоняет `configs/etl_validation_rules.yaml` через severity-engine (переиспользует движок Модуля 1, ADR-011).
6. Строит 5 mart-таблиц (`marts.mart_*`) через SQL `INSERT … SELECT` с агрегациями и резолюцией `applicable_rule_id` (ADR-013/Q-024 priority `order_rule > supply_spec`).
7. Atomic flip: `INSERT` (append-семантика contract §3.1) + UPDATE «текущей» партиции в одной транзакции; на commit — `etl_runs.status='committed', committed_at=now()`. Освобождает advisory lock.
8. Если `lines_failed/lines_total > 1%` (Q-015) — `etl_runs.status='failed'`, marts не flip-аются, старые витрины остаются актуальными.

Бинарь предоставляет HTTP-API `/admin/etl-*` (роль `admin-cli`), `/admin/reject-log`, `/admin/marts/{name}/refresh` (ondemand для `mart_supplier_scorecard`, Q-021), `/healthz`, `/metrics`.

---

## 1. Scope (что делаем / что не делаем)

### Делаем (MVP)
- Один отдельный бинарь `cmd/etl/main.go` (graceful shutdown, аналог `cmd/source-adapter/main.go`).
- Feature `internal/features/etl_validation/` (ADR-002).
- Schema `marts` в той же БД (ADR-006), 5 mart-таблиц + `etl_runs` + `reject_log` + `audit_access`.
- Append-only ETL run (recompute прошлых партиций — отложено, ADR-012).
- Severity-engine reuse (как библиотечный пакет переиспользуем `internal/features/data_export/validation` — оборачиваем в новые builtin-чеки `fk_exists`/`unique_business_key`/`aggregate_sum_matches`/`referential_integrity`/`null_required_field`).
- HTTP-клиент к API source-adapter (`extractor/`) с ETag, JWT, retry+backoff (cap 30s).
- Scheduler (`gocron/v2`) + advisory lock (ADR-104).
- Admin-API (Fiber v3): `POST /admin/etl-runs`, `POST /admin/etl-runs/{id}/retry`, `GET /admin/etl-runs/{id}`, `GET /admin/etl-runs`, `GET /admin/reject-log`, `POST /admin/marts/{name}/refresh`.
- Observability: `slog` JSON + Prometheus metrics + Grafana дашборд расширяется новыми `etl_*` метриками.
- dockertest интеграционные тесты, общий Suite через `pkg/dockertestpkg`.
- Migrations внутри feature (`internal/features/etl_validation/sqls/migrations/*.sql`), ops запускает `make migrate-up-etl`.

### НЕ делаем (out of scope MVP)
- Bi-temporal recompute прошлых дней (Q-012/ADR-012, отложено).
- FDW / отдельный кластер для marts (ADR-006).
- Webhook от source-adapter / LISTEN-NOTIFY (ADR-003 — выбран own cron).
- DSL-формулы калькулятора и сам Replenishment (Модуль 5).
- Cold-storage (Parquet/S3) для marts.
- Multi-tenant.

---

## 2. Контекст и пользователи

| Тип | Что делает | Через что |
|---|---|---|
| **Replenishment калькулятор (Модуль 5)** | Читает `mart_calculation_input`, `mart_master_current`, `mart_demand_history` | прямой SQL (read-only role `mart_reader`, ADR-023) |
| **KPI-модуль (Модуль 4)** | Читает `mart_kpi_daily`, `mart_supplier_scorecard` | прямой SQL (`mart_reader`) |
| **DevOps / on-call X-Flow** | Перезапуск failed run, ondemand refresh `mart_supplier_scorecard` | `POST /admin/etl-runs`, `POST /admin/etl-runs/{id}/retry`, `POST /admin/marts/{name}/refresh`, `GET /admin/etl-runs/{id}` (JWT role `admin-cli`) |
| **IT E-Zoo** | audit просмотр (read-only) | `GET /admin/etl-runs`, `GET /admin/etl-runs/{id}` (JWT role `it-read`) |

**Owner модуля:** команда X-Flow.

---

## 3. Структура feature `internal/features/etl_validation/`

```
internal/features/etl_validation/
├── handler/                         # Fiber-handlers (admin-API)
│   ├── admin_etl_runs.go            # POST /admin/etl-runs, /retry, GET /admin/etl-runs/{id}, GET /admin/etl-runs
│   ├── admin_marts.go               # POST /admin/marts/{name}/refresh
│   ├── admin_reject_log.go          # GET /admin/reject-log
│   └── healthz.go                   # GET /healthz
├── service/
│   ├── etl_run.go                   # бизнес-логика create/retry/get
│   ├── etl_pipeline.go              # ETL pipeline: Extract → Stage → Validate → Transform → Load → Flip
│   └── mart_refresh.go              # ondemand refresh mart_supplier_scorecard
├── repository/                      # pgxpool + go:embed SQL
│   ├── etl_runs.go
│   ├── reject_log.go
│   ├── audit_access.go
│   ├── marts.go                     # UPSERT/INSERT в mart_*
│   └── staging.go                   # CREATE TEMP TABLE + COPY-style load
├── extractor/                       # HTTP-клиент к API source-adapter
│   ├── client.go                    # net/http Client с retry+backoff cap 30s
│   ├── snapshots.go                 # SnapshotsClient.GetCurrent()
│   └── entities.go                  # EntitiesClient.Stream(entity, snapshot, etag)
├── transformer/                     # SQL-driven построение mart_*
│   ├── demand_history.go
│   ├── calculation_input.go         # priority order_rule > supply_spec (ADR-024)
│   ├── kpi_daily.go
│   ├── master_current.go
│   └── supplier_scorecard.go
├── loader/                          # UPSERT в mart_* + atomic flip
│   ├── upsert.go
│   └── flip.go                      # один tx: INSERT + commit etl_runs
├── validation/                      # ETL-engine (reuse Модуля 1)
│   ├── engine_adapter.go            # обёртка над data_export/validation
│   ├── builtin_fk_exists.go
│   ├── builtin_unique_bkey.go
│   ├── builtin_aggregate_sum.go
│   ├── builtin_ref_integrity.go
│   └── builtin_null_required.go
├── validators/                      # формат-валидаторы request DTO
│   └── admin_request.go
├── scheduler/                       # gocron + advisory lock
│   ├── cron.go
│   └── lock.go                      # pg_try_advisory_lock(hash('etl-run'))
├── models/
│   ├── etl_run.go
│   ├── reject.go
│   ├── mart_*.go                    # 5 типов
│   └── dto/
│       ├── admin_etl_run_request.go
│       ├── admin_etl_run_response.go
│       └── admin_mart_refresh.go
├── mappers/
│   ├── source_dto_to_domain.go
│   └── domain_to_mart.go
├── router/
│   └── router.go                    # Register(app, deps)
├── sqls/
│   ├── queries/
│   │   ├── etl_runs_insert.sql
│   │   ├── etl_runs_update_status.sql
│   │   ├── etl_runs_get_by_id.sql
│   │   ├── etl_runs_list.sql
│   │   ├── reject_log_insert.sql
│   │   ├── reject_log_select.sql
│   │   ├── mart_demand_history_insert.sql
│   │   ├── mart_calculation_input_insert.sql
│   │   ├── mart_kpi_daily_insert.sql
│   │   ├── mart_master_current_insert.sql
│   │   ├── mart_supplier_scorecard_insert.sql
│   │   └── audit_access_insert.sql
│   └── migrations/
│       ├── 1001_marts_schema.up.sql
│       ├── 1001_marts_schema.down.sql
│       ├── 1002_etl_runs.up.sql
│       └── 1002_etl_runs.down.sql
└── configs/
    └── etl_validation_rules.yaml    # стартовый набор правил (см. §6)
```

> Migrations нумеруются с `1001` чтобы не конфликтовать с миграциями Модуля 1 (`0001..0099`).

---

## 4. cmd/etl/main.go

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/Kitavrus/e_zoo/internal/etlapp"
    "github.com/Kitavrus/e_zoo/internal/etlapp/config"
)

func main() {
    cfg, err := config.Load() // envconfig, prefix ETL_
    if err != nil { slog.Error("config load failed", "err", err); os.Exit(1) }

    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
    slog.SetDefault(logger)

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    app, err := etlapp.New(ctx, cfg, logger)
    if err != nil { slog.Error("etl app init failed", "err", err); os.Exit(1) }

    if err := app.Run(ctx); err != nil { slog.Error("etl app run failed", "err", err); os.Exit(1) }
}
```

> Структура `internal/etlapp` (а не `internal/app`) живёт рядом с `internal/app` Модуля 1 — чтобы каждый бинарь имел свой собственный набор «корневых» зависимостей. Подробности в [design-di.md](design-di.md).

---

## 5. Mart-таблицы (5 шт, ADR-006/007/008)

| Mart | Партиционирование | Retention | PK | Refresh strategy |
|---|---|---|---|---|
| `marts.mart_demand_history` | `RANGE(as_of_date)` месячные партиции (ADR-007) | 365d (ADR-008) | `(product_id, location_id, as_of_date)` | append daily |
| `marts.mart_calculation_input` | не партиционировано | current snapshot (truncate+rebuild) | `(product_id, location_id)` | full rebuild per run |
| `marts.mart_kpi_daily` | `RANGE(as_of_date)` месячные партиции | 365d | `(location_id, kpi_name, as_of_date)` | append daily |
| `marts.mart_master_current` | не партиционировано | current snapshot (truncate+rebuild) | `(entity_type, entity_id)` | full rebuild per run |
| `marts.mart_supplier_scorecard` | не партиционировано | rolling weekly | `(supplier_id, week_start)` | rolling weekly + ondemand (Q-021) |

**Общие колонки во всех mart-таблицах:**
- `etl_run_id UUID NOT NULL`
- `source_load_id UUID NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Полная схема — в [design-sql.md](design-sql.md). Контракт колонок (бизнес-поля) — в `docs/tmp/data-marts/contract-2026-05-06.md`.

---

## 6. Validation rules (стартовый YAML)

`configs/etl_validation_rules.yaml`:

```yaml
version: 1
entity_optional: []
rules:
  # mart_calculation_input ↔ products
  - { id: et-fk-001, entity: mart_calculation_input, check: fk_exists,
      field: product_id, ref_entity: products, ref_field: id, severity: critical }
  # mart_calculation_input ↔ locations
  - { id: et-fk-002, entity: mart_calculation_input, check: fk_exists,
      field: location_id, ref_entity: locations, ref_field: id, severity: critical }
  # mart_demand_history unique business key
  - { id: et-uq-001, entity: mart_demand_history, check: unique_business_key,
      keys: [product_id, location_id, as_of_date], severity: critical }
  # mart_kpi_daily aggregate sum check
  - { id: et-agg-001, entity: mart_kpi_daily, check: aggregate_sum_matches,
      field: revenue_total, source_entity: receipt_line, source_filter: "line_kind='sale'",
      tolerance_pct: 0.01, severity: critical }
  # referential integrity между master_current и calculation_input
  - { id: et-ri-001, entity: mart_calculation_input, check: referential_integrity,
      field: applicable_rule_id, ref_entity: mart_master_current, severity: critical }
  # null_required_field
  - { id: et-nl-001, entity: mart_calculation_input, check: null_required_field,
      field: applicable_rule_id, severity: soft }
```

Movement of severity (`critical` → блокирует commit при >1%, `soft` → пишется в `reject_log` без блокировки) — повторяет Модуль 1.

---

## 7. Endpoints

| Метод | Путь | Роли | Назначение |
|---|---|---|---|
| GET | `/healthz` | none | liveness/readiness |
| GET | `/metrics` | none (metrics-port отдельно) | Prometheus exposition |
| POST | `/admin/etl-runs` | `admin-cli` | force-start ETL run (если lock свободен) |
| POST | `/admin/etl-runs/{id}/retry` | `admin-cli` | retry failed run (тот же `source_load_id`) |
| GET | `/admin/etl-runs/{id}` | `admin-cli`, `it-read` | детали run-а + JSON `marts_summary` |
| GET | `/admin/etl-runs` | `admin-cli`, `it-read` | пагинированный список последних runs |
| GET | `/admin/reject-log` | `admin-cli` | пагинированный список reject-rows |
| POST | `/admin/marts/{name}/refresh` | `admin-cli` | ondemand refresh (только `mart_supplier_scorecard`, иначе `ErrMartRefreshNotSupported`) |

Контракт DTO и sequence — [design-sequence-diagrams.md](design-sequence-diagrams.md).

---

## 8. Ключевые решения и ссылки

- **Reuse:**
  - `pkg/errorspkg` — sentinel + `WriteJSON` ([design-errors.md](design-errors.md)).
  - `internal/middleware/jwt` + `internal/middleware/role` — JWT + RBAC.
  - `pkg/logger` — slog wrapper.
  - `pkg/dockertestpkg` — общий suite для интеграционных тестов.
  - `internal/features/data_export/validation` — severity engine (импортируется как библиотека).
- **Новое:**
  - Бинарь `cmd/etl/main.go`.
  - `internal/etlapp` — DI и lifecycle.
  - Schema `marts` (миграции 1001/1002).
  - `configs/etl_validation_rules.yaml`.
  - HTTP-клиент `extractor/` к source-adapter API (новый зависимый сервис).
- **Безопасность:**
  - Все mart-таблицы — read-only role `mart_reader` (`GRANT SELECT ON ALL TABLES IN SCHEMA marts TO mart_reader`).
  - `audit_access` — пишется на каждый admin-запрос (как в Модуле 1).
  - JWT `x-flow-etl` для запросов к source-adapter API; ключ через `ETL_JWT_SIGNING_KEY` (HS256) либо `ETL_JWT_PUBLIC_KEY_PATH` (RS256).

---

## 9. Связанные документы

| Файл | Что внутри |
|---|---|
| `design-c4.md` | C4 levels 1-4 для feature |
| `design-dataflow.md` | cron tick → ETL pipeline → marts |
| `design-sequence-diagrams.md` | sequence для каждого endpoint + cron-tick |
| `design-go-layers.md` | детали папок, интерфейсы, контракты |
| `design-sql.md` | миграции 1001/1002 + queries |
| `design-tests.md` | dockertest, golden, sentinel↔test |
| `design-di.md` | DI отдельного бинаря |
| `design-integrations.md` | HTTP-клиент к source-adapter, JWT, slog, Prometheus |
| `design-errors.md` | новые sentinel + HTTP mapping |
| `design-infrastructure.md` | docker-compose service `etl`, env, Makefile |
| `design-adr.md` | 25 ADR (1:1 c Q-NNN) + мета-ADR-100+ |

> **NB.** `design-swimlane.html` создаётся отдельным subagent.
