# Research: etl-validation

## Module
- Имя модуля: `github.com/Kitavrus/e_zoo`
- Стек: Go 1.26, Fiber v3 (`v3.0.0-beta.4`), pgx v5 + pgxpool, golang-migrate v4, gocron v2, golang-jwt v5, prometheus/client_golang, validator/v10, dockertest v3, testify v1.11, yaml.v3, envconfig.
- Один бинарник на момент исследования: `cmd/source-adapter/main.go`. Модуль etl-validation в коде отсутствует.

## Что уже есть в коде (после реализации Модуля 1)

### Backend (паттерны для повторного использования)
Структура директории `internal/features/data_export/` (из Модуля 1):
- `audit/` — `writer.go` (admin audit log writer + тест).
- `exports/` — `service.go` (async export pipeline).
- `exports_storage/` — `storage.go` интерфейс + `local_fs.go` реализация.
- `handler/` — Fiber handlers по сущности: `admin_loads.go`, `etag.go`, `exports.go`, `products.go`, `snapshots.go`, `not_implemented.go`. Подпапки `mappers/` и `validators/` (handler-level).
- `loader/` — оркестратор daily-load (Read → Map → Validate → UPSERT → Flip).
- `mappers/` — domain ↔ ERP DTO (`master.go`, `facts.go`).
- `models/dto/` — DTO-слой (`admin.go`, request/response, entity-DTO с тегами `db:` + `json:`).
- `repository/` — pgxpool + `go:embed` SQL.
- `router/` — `Register(app, deps)` и `Deps`. Регистрация в `internal/routers/routers.go`.
- `scheduler/` — gocron scheduler + `scheduler_test.go`.
- `snapshot/` — atomic flip + advisory lock.
- `sqls/migrations/` (с `embed.go`, `0001_master_and_service`, `0002_facts_partitioned`) и `sqls/queries/` (`select_*.sql`, `loads_*.sql`, `snapshot_*.sql`, `reject_log_*.sql`, `audit_*.sql`, `advisory_lock_try.sql`, `advisory_unlock.sql`).
- `validation/` — `engine.go`, `builtin.go`, `engine_test.go`. Severity-движок с `SeverityCritical` / `SeveritySoft`, YAML-конфиг, `Violation{RuleID, Entity, Field, Severity, Message}`, `Engine.Check(entity, payload, state) []Violation`, поддержка `entity_optional`.

Общие пакеты:
- `internal/app/app.go`, `internal/config/config.go` (envconfig).
- `internal/logger/logger.go` — slog JSON handler.
- `internal/middleware/` — `jwt.go`, `role.go` (constants `RoleXFlowETL = "x-flow-etl"`, `RoleAdminCLI = "admin-cli"`, `RoleITRead = "it-read"`; helpers `RequireRole`, `RequireAnyOf`), `request_id.go`.
- `internal/observability/` — Prometheus metrics + healthz.
- `internal/routers/routers.go` — feature-router aggregator (`Register(app, dataExportDeps)`).
- `pkg/errorspkg/` — `errors.go`, `response.go` (`WriteJSON`), `support_codes.go`. Sentinel set из 23 ошибок.
- `cmd/source-adapter/main.go` — единственный entrypoint.

### Contracts (API source-adapter — вход ETL)
- `GET /v1/{master_entity}` (products, product_barcodes, category, location, supplier) с `since`, `cursor`, `limit`. Заголовки запроса: `Authorization: Bearer <jwt>` (роль `x-flow-etl`). Ответ: `Content-Type: application/x-ndjson`, `X-Snapshot-Id`, `X-Load-Id`, `ETag`, `Cache-Control`. Тело — NDJSON stream.
- `GET /v1/snapshots/current` → `{current_load_id, previous_load_id, committed_at}`.
- `GET /v1/snapshots?limit=N` — список committed загрузок.
- Когда snapshot отсутствует → 503 `{code:"snapshot_not_ready"}` + `Retry-After: 60`.
- 401 при отсутствии JWT, 403 при неверной роли.
- Atomic flip: `snapshot_pointer.current_load_id` обновляется одной транзакцией только после успеха ВСЕХ сущностей и quality-threshold (`lines_failed/lines_total <= 1%`). При неудаче — `loads.status='failed'`, `failure_reason='flip_failed'`. Чтение `/v1/*` фиксирует `current_load_id` в начале запроса (ADR-014, ADR-102).
- Bi-temporal коррекции пробрасываются через `master_change_log` и `receipt_line` (см. контракт витрин).

### Infrastructure
- PostgreSQL 18 (`postgres:18-alpine` в dockertest, `infra/pg/postgresql.conf`).
- Prometheus (`infra/prometheus/prometheus.yml`).
- Cron: `gocron.NewScheduler(gocron.WithLocation(loc))` (loc = Europe/Kyiv), `WithSingletonMode(LimitModeReschedule)` + PG advisory lock через `pg_try_advisory_lock(hash('daily-load'))`. Lock skip → `load_skipped_total++`.
- Migrations: `golang-migrate/v4` + `iofs` + `go:embed` из `sqls/migrations/embed.go`.
- Audit retention 90 дней, ежедневный cleanup-job.

### Tests
- Уровни: unit (handler/service/validator/mappers), integration repository (dockertest+golang-migrate), integration loader e2e (in-memory `SourceReader`), integration HTTP API (Fiber `app.Test()`), concurrency (parallel POST /admin/loads → 409).
- Базовый Suite: `internal/features/data_export/repository/integration_suite_test.go` поднимает `postgres:18-alpine` через `dockertest.NewPool`, mounts migrations через `iofs`, raw DSN `postgres://test:test@host/adapter_test`.
- Validator: table-driven tests на правила (`negative_qty`, `future_event_time`, `fk_exists` и т.д.).
- Mappers: golden files.
- HTTP fixtures: `testdata/fixtures/*.json` для каждой entity.

## Паттерны в коде (что ETL должен соблюдать)
- Feature-folder: `internal/features/<feature_name>/{handler,service,repository,sqls,models/dto,mappers,validators,router,scheduler,...}`. Регистрация фичи через единый `Register(app, deps)` из `internal/routers/routers.go`.
- SQL только через `go:embed` (`sqls/queries/*.sql`, `sqls/migrations/*.sql`), без ORM.
- Pgxpool как единственный DB-handle (`*pgxpool.Pool`).
- DTO имеют теги `db:` + `json:`; mappers разделены на `master.go`/`facts.go`.
- Severity-валидация: YAML-driven `validation.Engine`, две severity (`critical`, `soft`), `entity_optional` whitelist.
- Snapshot semantics: atomic flip в одной транзакции; `loads.status` ∈ {`running`, `committed`, `failed`, `aborted`}; `snapshot_pointer.current_load_id` источник правды для читателей.
- Advisory lock per-job: `pg_try_advisory_lock(hash(jobname))`, скрытый skip, метрика `*_skipped_total`.
- Scheduler: gocron singleton + advisory lock как защита второго уровня. TZ Europe/Kyiv.
- Logger slog JSON c контекстными полями: `request_id`, `load_id`, `entity`, `status_code`, `duration_ms`, `requester` (JWT sub).
- Errors: sentinel-set + `errorspkg.WriteJSON(c, err)` (HTTP-mapping inline в handler-ах, отдельного `mappers/errors.go` нет).
- JWT middleware: `Authorization: Bearer`, claims проверяются на `role` ∈ {`x-flow-etl`, `admin-cli`, `it-read`}.
- Тесты: testify suite + dockertest + golang-migrate + Fiber `app.Test()`.
- Quality threshold: `lines_failed/lines_total > 1%` блокирует commit.

## Чего НЕТ (потребуется создать)
- Все слои фичи `etl_validation` (или `data_etl`) в `internal/features/`: `handler/`, `service/`, `repository/`, `models/dto/`, `router/`, `sqls/{migrations,queries}/`, `validators/` (cross-entity), `mappers/`, `scheduler/`.
- Entry-point: либо отдельный `cmd/etl/main.go`, либо подключение в существующий `cmd/source-adapter/main.go` (не определено).
- Mart-таблицы (миграции) для `mart_demand_history`, `mart_calculation_input`, `mart_kpi_daily`, `mart_master_current`, `mart_supplier_scorecard` (контракт §3.1–3.5). Схема расположения (`marts.*` schema, отдельная БД, FDW/реплика) в коде не задана.
- ETL pipeline:
  - Extract: HTTP-клиент к `/v1/{entity}` source-adapter (NDJSON streaming + ETag + cursor + JWT с ролью `x-flow-etl`). Текущий код HTTP-клиента к `v1/*` отсутствует.
  - Transform: денормализация (JOIN products+category+brand+primary_supplier), агрегации `SUM/COUNT DISTINCT` по `receipt_line` с группировкой `(location, product, as_of_date)`, формирование флагов `had_promo`, `was_in_assortment`, `lifecycle_state_at_date`, `was_oos`.
  - Load: append/full-rebuild стратегии (`mart_demand_history` append, `mart_calculation_input` full rebuild, `mart_kpi_daily` append, `mart_master_current` full rebuild, `mart_supplier_scorecard` rolling weekly).
- Trigger механизм: контракт говорит «после `committed` нашего load» — конкретный механизм (cron / pull `GET /v1/snapshots/current` / webhook от source-adapter / событие в БД) не реализован.
- Cross-entity validators (FK consistency между сущностями: `receipt_line.product_id ∈ products`, `supply_spec.supplier_id ∈ supplier`, дедупликация по бизнес-ключу).
- Bi-temporal коррекции: пересчёт исторических дней по `master_change_log` / corrections в `receipt_line`. Логика идемпотентности витрин не реализована.
- ETL-snapshot semantics: понятие `etl_run_id` существует только как поле в контракте витрин, таблица `etl_runs` / `etl_run_pointer` отсутствует.
- ETL метрики Prometheus (`etl_run_duration_seconds`, `etl_lines_processed_total`, `etl_lines_failed_total`, `mart_rows_total`, `etl_lag_seconds` относительно `committed_at` source-adapter).
- Конфиг идентификации `source_load_id` → `etl_run_id` provenance в каждой строке витрины.
- Tests: integration suite для marts, golden-fixtures для агрегаций.

## Зависимости (что затронет изменение)
- API Модуля 1 (потребитель): `GET /v1/{entity}`, `GET /v1/snapshots/current`, NDJSON streaming, ETag/X-Snapshot-Id, JWT-роль `x-flow-etl`. ETL обязан учитывать `snapshot_not_ready` 503 + `Retry-After`. Дополнительных endpoints в Модуле 1 для ETL не описано.
- Контракт витрин Модуля 3 (производитель): таблицы `mart_demand_history`, `mart_calculation_input`, `mart_kpi_daily`, `mart_master_current`, `mart_supplier_scorecard` (контракт §3) с обязательными полями `source_load_id`, `etl_run_id`. Replenishment (Модуль 3) читает их read-only из схемы `marts` той же БД либо через FDW (spec replenishment §4.5).
- БД: `replenishment_plans`, `forecasts`, `runs_queue`, `audit_events` — таблицы Replenishment, могут быть в той же БД (только PostgreSQL 18 как ограничение).
- Open MQ-вопросы из контракта витрин (MQ-1…MQ-5) частично пересекаются с областью этого модуля (особенно MQ-5: владение `applicable_rule_id`).

## Frontend: применимо или нет
Не применимо. Модуль 1 — backend-only (Fiber HTTP API + cron). Модуль 2 (ETL) — backend-only по природе: pull данных из API Модуля 1, трансформация, запись в `mart_*` таблицы PostgreSQL. UI отсутствует и в контракте витрин, и в spec Replenishment §4.5 (внешние зависимости — только PostgreSQL).

## Открытые вопросы для Spec Interview (Q-NNN)
1. Q-001: Где запускается ETL — отдельный binary `cmd/etl/main.go`, или подключается в существующий `cmd/source-adapter/main.go` как доп.scheduler-job? Влияет на DI и deploy-unit.
2. Q-002: Имя feature-папки: `internal/features/etl_validation/` или `internal/features/data_etl/` (или иное)? Контракт витрин использует имя «X-Flow ETL» (внешнее), draft-plan — «ETL и валидация».
3. Q-003: Триггер запуска: (a) собственный cron (после source-adapter cron 02:00 Kyiv с задержкой), (b) polling `GET /v1/snapshots/current` каждые N минут с проверкой смены `current_load_id`, (c) webhook от source-adapter (`POST /etl/notify`), (d) событие через `LISTEN/NOTIFY` PostgreSQL.
4. Q-004: Идемпотентность ETL run — использовать ту же advisory-lock семантику (`pg_try_advisory_lock(hash('etl-run'))`) и таблицу `etl_runs(id, status, source_load_id, ...)` по аналогии с `loads`?
5. Q-005: ETL читает текущий snapshot через `GET /v1/snapshots/current` и фиксирует `source_load_id` на весь run (atomic read), или допустимы разные `load_id` для разных entity внутри одного run-а?
6. Q-006: Где живут `mart_*` — в той же БД source-adapter (другая schema `marts`), отдельная БД с FDW, отдельный кластер? Контракт витрин §4.5 spec-replenishment допускает «в той же БД либо через FDW/реплику».
7. Q-007: Партиционирование `mart_*` (особенно `mart_demand_history`, `mart_kpi_daily`) — by `as_of_date` RANGE по аналогии с `0002_facts_partitioned`? Размер партиции (день / месяц)?
8. Q-008: Retention для `mart_*` — глубина истории `mart_demand_history` (контракт MQ-3: 1/2/3+ года); политика очистки старых партиций.
9. Q-009: Метрики Prometheus — какой набор обязателен на MVP? (`etl_run_duration_seconds`, `etl_lines_processed_total{entity}`, `etl_lines_failed_total`, `mart_rows_total{mart}`, `etl_lag_seconds`, `etl_runs_skipped_total`, `etl_run_quality_threshold_violated_total`).
10. Q-010: Обработка failed ETL run — повторный запуск автоматически или только вручную? Частичный rollback per-mart (drop append-rows by `etl_run_id`) или полный rebuild? Запоминается ли последний committed `source_load_id` для возобновления?
11. Q-011: Аналог severity для ETL — нужен ли YAML-driven validation engine для cross-entity rules (FK consistency, дедупликация, business-rule агрегаций), и какой набор built-in checks (`fk_exists`, `unique_business_key`, `aggregate_sum_matches`)? Или ETL верит, что severity критики уже отсеяна Модулем 1?
12. Q-012: Bi-temporal recompute — как идентифицируются «переоткрытые» дни в `mart_demand_history`/`mart_kpi_daily`? Через diff `master_change_log` за prev run, или через columns `system_time_*` исходных таблиц (которые контракт §2.5 запрещает пробрасывать в витрины наружу, но ETL читает их внутрь)?
13. Q-013: `applicable_rule_id` (контракт MQ-5) — вычисляется на этапе сборки `mart_calculation_input` ETL-ом, или калькулятор Replenishment вычисляет его сам? Влияет на состав ETL-логики.
14. Q-014: `etl_run_id` — UUID v4, генерируется на старте run, пишется в каждую строку всех витрин; нужна ли таблица-реестр `etl_runs(id, started_at, finished_at, status, source_load_id, marts_summary jsonb)` для аудита?
15. Q-015: Quality threshold для ETL run — переиспользуется ли пороговое значение `lines_failed/lines_total > 1%` из source-adapter (ADR loader), или у ETL свои пороги per-mart?
16. Q-016: Как ETL получает access к API source-adapter — через JWT с ролью `x-flow-etl`, секрет которого хранится в env (`ETL_JWT_SIGNING_KEY` / общий с adapter), или через service-to-service токены?
17. Q-017: Backpressure / cursor: ETL делает full read через NDJSON на каждый run, или сохраняет последний обработанный `cursor` per entity и делает incremental pull (`since=...`)? Контракт §3.1 говорит «append daily», что предполагает incremental.
18. Q-018: Логи и observability — куда складывать ETL-структурные логи (`slog` с context fields `etl_run_id`, `mart`, `source_load_id`); попадают ли они в общий Prometheus/Grafana, описанный в `infra/prometheus/`?
19. Q-019: Тесты — нужен ли отдельный `dockertest` Suite с marts-схемой, или достаточно использовать тот же `adapter_test` DB? Golden-фикстуры для агрегаций (input NDJSON → expected mart rows)?
20. Q-020: Mart-схема ownership — миграции `mart_*` живут в `internal/features/etl_validation/sqls/migrations/` или в общей БД с миграциями source-adapter? Кто запускает migrate-up в проде?
21. Q-021: `mart_supplier_scorecard` (контракт §3.5) обновляется «rolling weekly + ondemand» — какой триггер ondemand, кто его дёргает, через какой API?
22. Q-022: Аутентификация для административных ETL-ручек (если будут `/admin/etl-runs` по аналогии с `/admin/loads`) — роль `admin-cli`?

## Важно — только факты, без рекомендаций.

## Ответ оркестратору
- Кодовая база Модуля 1 уже реализована в `internal/features/data_export/` с полным набором паттернов (handler/service/repository/loader/validation/snapshot/scheduler/sqls), severity-движок (`SeverityCritical`/`SeveritySoft`) — есть; mart-таблицы, ETL-feature, ETL-snapshot и cross-entity валидация — отсутствуют.
- Контракт витрин (`docs/tmp/data-marts/contract-2026-05-06.md`) фиксирует 5 mart-таблиц с refresh-стратегией и требует поля `source_load_id` + `etl_run_id` в каждой строке. Replenishment ожидает их read-only в схеме `marts` (той же БД либо FDW).
- Новые компоненты для создания: feature-folder `etl_validation` со всеми слоями; миграции `mart_*`; HTTP-клиент к API source-adapter (NDJSON+JWT+ETag); pipeline Extract→Transform→Load; trigger-механизм; реестр `etl_runs`; cross-entity validators; bi-temporal recompute; метрики; тесты (dockertest).
- Сформулировано 22 открытых вопроса (Q-001…Q-022).
- Готовность к spec-interview: высокая (паттерны Модуля 1 и контракт Модуля 3 зафиксированы; основные пробелы — деталь триггера, размещения marts, схемы партиционирования и владение `applicable_rule_id`).
