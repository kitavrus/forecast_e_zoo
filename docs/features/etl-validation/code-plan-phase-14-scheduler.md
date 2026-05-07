# Phase 14 — Scheduler (gocron + advisory lock)

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Поднять scheduler через `github.com/go-co-op/gocron/v2`: cron `30 2 * * *` в Europe/Kyiv, pre-step partition maintenance (создать партиции на следующий месяц для `mart_demand_history` и `mart_kpi_daily`), тик вызывает `service.EtlPipeline.TryStart` с `trigger='cron'`. Advisory lock — сторона `service.EtlPipeline` (already из фазы 13), но scheduler сам тоже должен реагировать на «занят» graceful (логировать + skip + инкремент `etl_runs_skipped_total`).

## Commit

```
feat(etl_validation): scheduler (gocron) cron 02:30 Europe/Kyiv + partition maintenance
```

## Files to CREATE

### Production code

- `internal/features/etl_validation/scheduler/scheduler.go`:
  - `Scheduler` struct — `s gocron.Scheduler`, `pipeline service.EtlPipeline`, `partitionMaint PartitionMaintainer`, `logger *slog.Logger`, `metrics MetricsRecorder`.
  - `Start(ctx context.Context) error` — регистрирует job с cron expression и timezone из config, запускает scheduler.
  - `Stop(ctx context.Context) error` — graceful shutdown через `s.Shutdown()`.
  - Job-callback:
    1. `partitionMaint.EnsureNextMonth(ctx)` — создаёт партиции на следующий месяц если ещё нет.
    2. `pipeline.TryStart(ctx, "cron", nil, nil)`.
    3. Если `errors.Is(err, errorspkg.ErrEtlRunAlreadyRunning)` → log warn, `metrics.IncSkipped("already_running")`, return nil.
    4. Если `errors.Is(err, errorspkg.ErrSnapshotNotReady)` → log warn, `metrics.IncSkipped("snapshot_not_ready")`, return nil.
    5. Прочие ошибки → log error, метрика, return error (не блокирует следующий tick).
- `internal/features/etl_validation/scheduler/partition_maintenance.go`:
  - `PartitionMaintainer` интерфейс — `EnsureNextMonth(ctx) error`.
  - Реализация: через repository выполняет `CREATE TABLE IF NOT EXISTS marts.mart_demand_history_yYYYY_mMM PARTITION OF marts.mart_demand_history FOR VALUES FROM ('YYYY-MM-01') TO ('YYYY-MM-01' + INTERVAL '1 month');` для текущего и следующего месяца. То же для `mart_kpi_daily`.
- `internal/features/etl_validation/repository/partitions.go` (если решено вынести SQL):
  - `EnsurePartitions(ctx, tx pgx.Tx, table string, month time.Time) error`.

### Tests (integration)

- `internal/features/etl_validation/scheduler/scheduler_integration_test.go`:
  - `TestScheduler_PartitionMaintenance_CreatesMissing` — на чистой БД без партиций → `EnsureNextMonth` создаёт текущий + следующий.
  - `TestScheduler_AdvisoryLockContention` — два scheduler-а в одной БД одновременно тикают; только один захватывает lock, второй получает skipped.
  - `TestScheduler_TickInvokesPipeline` — gocron `RunNow()` или fake clock; убеждаемся, что `pipeline.TryStart` вызвался ровно один раз.

### Tests (unit)

- `internal/features/etl_validation/scheduler/scheduler_test.go`:
  - mock `pipeline` + `partitionMaint`, проверяем mapping ошибок → метрики/логи.

## Files to MODIFY

- `internal/etlapp/app.go` — wire scheduler в DI после фазы 15 (в фазе 14 — только заготовка `app.scheduler = scheduler.New(...)`, реальный `Start` вызовется в фазе 15 при сборке всей DI). Альтернатива: вызов `scheduler.Start` в `app.Run` уже здесь, и финализация в фазе 15. Используем второй вариант — здесь же вызываем `scheduler.Start` из `app.Run`.

## SQL / Migrations

Нет (DDL для партиций — динамический в Go).

## Run after

```bash
go build ./internal/features/etl_validation/scheduler/...
go test ./internal/features/etl_validation/scheduler/... -race -count=1
go test -tags=integration ./internal/features/etl_validation/scheduler/... -race -count=1
golangci-lint run ./internal/features/etl_validation/scheduler/...
```

## Tests

| Test | Что проверяет |
|---|---|
| `TestScheduler_PartitionMaintenance_CreatesMissing` | партиции на след. месяц создаются |
| `TestScheduler_AdvisoryLockContention` | concurrency: только один scheduler начинает run |
| `TestScheduler_TickInvokesPipeline` | tick вызывает pipeline один раз |
| `TestScheduler_HandlesAlreadyRunning` | mapping в `etl_runs_skipped_total{reason=already_running}` |
| `TestScheduler_HandlesSnapshotNotReady` | mapping в `skipped{reason=snapshot_not_ready}` |

## Definition of Done

- [ ] Scheduler стартует с cron `30 2 * * *` в Europe/Kyiv (из конфига).
- [ ] Partition maintenance создаёт партиции до запуска pipeline-а.
- [ ] Advisory lock contention отрабатывает graceful (skip с метрикой).
- [ ] Graceful shutdown реализован (Stop корректно завершает текущий job).
- [ ] Integration test для concurrency проходит детерминированно.
- [ ] Coverage ≥80%.
- [ ] `golangci-lint`/`go vet` зелёные.

## Зависимости

Требует фаз 06 (repository), 13 (pipeline). Использует `go-co-op/gocron/v2`, добавленный в фазе 01.
