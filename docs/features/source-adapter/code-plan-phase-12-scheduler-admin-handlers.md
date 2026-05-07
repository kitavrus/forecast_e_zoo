# Phase 12: Scheduler (gocron) + admin handlers + integration test advisory-lock

**Цель:** обернуть Loader в (1) внутрипроцессный gocron-scheduler с PG advisory-lock для исключения параллельных запусков и (2) admin-handlers `POST /admin/loads`, `POST /admin/loads/{id}/retry`, `GET /admin/loads/{id}`, `GET /admin/reject-log`. Дополнительно — pre-step cron-tick-а: создание следующих месячных партиций (см. note из фазы 04). Integration-тест проверяет, что параллельные cron-tick-и не запускают два load-а одновременно.

**Commit:** `feat(data_export/scheduler): gocron + admin handlers + advisory-lock integration test`

---

## Files to CREATE

### Scheduler

- `internal/features/data_export/scheduler/scheduler.go`:
  - `type Scheduler struct{ cron gocron.Scheduler; loader *loader.Loader; repo RepoAPI; cfg Config; logger *slog.Logger }`.
  - `func New(cfg Config, loader, repo, logger) (*Scheduler, error)` — настраивает gocron с TZ из `SOURCE_ADAPTER_TZ`, добавляет job по cron-expression `SOURCE_ADAPTER_CRON_SCHEDULE` с `WithSingletonMode`.
  - `func (s *Scheduler) Start(ctx) error`, `func (s *Scheduler) Stop(ctx) error`.
  - `func (s *Scheduler) tick(ctx)` — главный обработчик:
    1. Внутри короткой транзакции `pg_try_advisory_lock(daily-load-key)`. Если lock не взят — `return` (другой инстанс работает). Lock держится ВСЁ время load-а — на отдельном долгоживущем conn.
    2. **Pre-step:** `EnsureNextPartitions(ctx)` — создаёт месячные партиции на текущий+2 мес. вперёд (для всех 4 partitioned-таблиц), идемпотентно.
    3. `MarkAborted` стейл-записей (>1ч `started_at` — env `SOURCE_ADAPTER_STALE_AFTER`, default 1h).
    4. `loader.Run(ctx, "erp_e_zoo")`.
    5. Освободить lock.
- `internal/features/data_export/scheduler/lock_key.go` — FNV-64 хеш строки `daily-load` → bigint.
- `internal/features/data_export/scheduler/partitions.go`:
  - `EnsureNextPartitions(ctx, pool, monthsAhead int) error` — для каждой из 4 partitioned-таблиц делает `CREATE TABLE IF NOT EXISTS <parent>_yYYYYmMM PARTITION OF ...` на текущий + N мес. вперёд.

### Admin handlers

- `internal/features/data_export/handler/admin_loads.go`:
  - `POST /admin/loads` → 202 + `{loadId}`. Проверка: уже есть running → 409 `ErrLoadAlreadyRunning` + `currentLoadId`. Иначе — асинхронно стартует через `scheduler.RunNow(ctx)` (новая функция `Scheduler.TriggerOnce(ctx)`).
  - `POST /admin/loads/{id}/retry` → берёт оригинальный load (404 если нет), стартует новый load (NOT возобновляет — ADR §«Retry = рестарт всего load-а»).
  - `GET /admin/loads/{id}` → 200 `GetLoadResponse`.
- `internal/features/data_export/handler/admin_reject_log.go`:
  - `GET /admin/reject-log?load_id=&entity=&severity=&cursor=&limit=` → `GetRejectLogResponse`.

### Mappers

- `internal/features/data_export/handler/mappers/error_mapper.go` — `MapServiceError(err) errorspkg.ErrorResponseJSON` — single-place преобразование sentinel → HTTP-ответ.

### Тесты

- `internal/features/data_export/scheduler/scheduler_test.go` — unit:
  - `TestLockKey_FNV_Stable` (детерминирован).
  - `TestEnsureNextPartitions_GeneratesSQL` (без БД, проверяет SQL-строки).
- `internal/features/data_export/scheduler/scheduler_integration_test.go` — integration (tag `integration`):
  - `TestScheduler_AdvisoryLock_PreventsParallel` — две горутины зовут `tick` параллельно; одна берёт lock, вторая выходит без работы. Проверяется по `loads` — создана ровно одна running-запись.
  - `TestScheduler_StaleLoad_GetsAborted` — seed `loads.status=running, started_at=2h_ago` → tick → status=aborted.
  - `TestScheduler_EnsureNextPartitions_Idempotent` — двойной вызов не падает.
- `internal/features/data_export/handler/admin_loads_test.go` — integration:
  - `TestAdminLoads_PostStartsNew_Returns202`
  - `TestAdminLoads_PostWhileRunning_Returns409WithCurrent`
  - `TestAdminLoads_GetByID_HappyPath`
  - `TestAdminLoads_GetByID_NotFound_404`
  - `TestAdminLoads_Retry_StartsNewLoad`
  - `TestAdminLoads_Retry_NotFound_404`
- `internal/features/data_export/handler/admin_reject_log_test.go`:
  - `TestRejectLog_FilterByEntity`
  - `TestRejectLog_Pagination`

## Files to MODIFY

- `internal/features/data_export/loader/loader.go` — гарантировать, что `Run` принимает `loadID` извне (или возвращает `loadID` и НЕ создаёт его сам — чтобы scheduler мог логировать заранее). Уточнить контракт.
- `pkg/errorspkg/errors.go` — sentinel `ErrCannotRetry` уже определён (фаза 08).
- `go.mod` / `go.sum` — `github.com/go-co-op/gocron/v2`.

## SQL/Migrations

— нет (DDL партиций строится в коде через `CREATE TABLE IF NOT EXISTS`).

## Run after

```bash
go mod tidy
make build
make test-unit
make test-integration
make lint
```

## Tests in this phase

- 2 unit-теста scheduler-а
- 3 integration-теста scheduler-а (включая advisory lock parallelism)
- 6 integration-тестов admin_loads
- 2 integration-теста admin_reject_log

Итого: ~13.

## Definition of Done

- [ ] gocron запускает tick по cron-expression из env.
- [ ] PG advisory lock исключает параллельные tick-и (integration test зелёный).
- [ ] Stale loads (>SOURCE_ADAPTER_STALE_AFTER) автоматом помечаются aborted.
- [ ] EnsureNextPartitions создаёт партиции на 2 мес. вперёд при каждом tick.
- [ ] `POST /admin/loads` 202 / 409.
- [ ] `POST /admin/loads/{id}/retry` стартует новый load.
- [ ] `GET /admin/loads/{id}` и `GET /admin/reject-log` отдают данные.
- [ ] `make build` / `make test-unit` / `make test-integration` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/scheduler): ...`.
