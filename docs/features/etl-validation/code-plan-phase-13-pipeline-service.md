# Phase 13 — EtlPipeline service (orchestration)

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Реализовать orchestrator `service.EtlPipeline.Run(ctx)`: Extract → Stage → Validate → Transform → Load → Flip + lifecycle `etl_runs` (running → committed/failed/aborted) + quality threshold gate (1%, ADR-015) + публикация метрик. Также `service.EtlRun` (admin CRUD-API) и `service.MartRefresh` (ondemand).

## Commit

```
feat(etl_validation): EtlPipeline service orchestration + EtlRun + MartRefresh services
```

## Files to CREATE

### Production code

- `internal/features/etl_validation/service/etl_pipeline.go`:
  - `EtlPipeline` struct — поля: pool, repo, extractor (Snapshots+Entities), validator, transformerRegistry, loader, lockKey int64, qualityThreshold float64, metrics MetricsRecorder, logger.
  - `TryStart(ctx, trigger string, requester *string, parentRunID *string) (*models.EtlRun, error)`:
    1. `pg_try_advisory_xact_lock` (через repository.Lock).
    2. Если занят → `ErrEtlRunAlreadyRunning` + текущий running run.
    3. INSERT etl_runs (status='running', kind='full', trigger, requester).
    4. Вернуть run + запустить `runAsync(ctx, run)` в goroutine с detached context (чтобы HTTP-handler сразу отвечал 202).
  - `runAsync(ctx, run)`:
    1. `extractor.SnapshotsClient.GetCurrent` → fix `source_load_id`.
    2. `repository.Staging.CreateTempTables(tx)`.
    3. Для каждой entity: `extractor.EntitiesClient.Stream(ctx, entity, sourceLoadID, etag)` → `repository.Staging.BulkLoad(tx, table, rows)`.
    4. `validation.Engine.Run(ctx, dataset)` → `Report` + bulk insert reject_log.
    5. Quality gate: `failed/total > 1%` → `ErrQualityThresholdExceeded` → markFailed.
    6. `loader.Apply(ctx, run, builders, summary)` → atomic commit.
    7. metrics.RecordSuccess.
  - `markFailed(ctx, runID string, reason string)` — UPDATE etl_runs SET status='failed', failure_reason, finished_at.
  - Mock-friendly: все зависимости через интерфейсы.
- `internal/features/etl_validation/service/etl_run.go`:
  - `Service` интерфейс — `TriggerRun`, `Retry`, `GetByID`, `List`.
  - `Retry(ctx, runID string, requester string)`:
    - GetByID → если status NOT IN ('failed','aborted') → `ErrCannotRetryEtl`.
    - Создать новый run с `parent_run_id=runID`, `trigger='retry'`, передать в `EtlPipeline.TryStart`.
- `internal/features/etl_validation/service/mart_refresh.go`:
  - `MartRefresh.Refresh(ctx, name string, requester string)`:
    - Validate name через `validators.ValidateMartRefresh`.
    - INSERT etl_runs (kind='mart_refresh', target_mart=name, trigger='admin').
    - Запустить только указанный builder (`registry.BuilderByName(name)`).
    - Loader.Apply для одного builder-а.
- `internal/features/etl_validation/service/mocks.go` (test-only):
  - Mock-ы интерфейсов extractor, validator, transformer, loader для unit-тестов.

### Tests (unit)

- `internal/features/etl_validation/service/etl_pipeline_test.go`:
  - `TestPipeline_TryStart_LockBusy` → `ErrEtlRunAlreadyRunning`.
  - `TestPipeline_TryStart_SnapshotNotReady` → `ErrSnapshotNotReady` (через mock extractor).
  - `TestPipeline_TryStart_HappyPath` → run committed, marts_summary populated.
  - `TestPipeline_QualityThresholdExceeded` → run failed, failure_reason set.
  - `TestPipeline_BuilderError_Rollback` → run failed.
  - `TestPipeline_ExtractorError_RetryThenFail` → run failed с `ErrSourceUnavailable`.
- `internal/features/etl_validation/service/etl_run_test.go`:
  - `TestRetry_StatusNotFailed` → `ErrCannotRetryEtl`.
  - `TestRetry_Failed_StartsNewRun` → новый run с `parent_run_id`.
  - `TestGetByID_NotFound` → `ErrEtlRunNotFound`.
- `internal/features/etl_validation/service/mart_refresh_test.go`:
  - `TestRefresh_UnsupportedMart` → `ErrMartRefreshNotSupported`.
  - `TestRefresh_HappyPath_Scorecard`.

### Tests (integration)

- `internal/features/etl_validation/service/etl_pipeline_integration_test.go`:
  - end-to-end на dockertest с mock source-adapter (`httptest.Server`):
    - mock возвращает snapshot + NDJSON streams.
    - Pipeline запускается, отрабатывает, в `marts.mart_*` появляются строки.
    - `marts.etl_runs` имеет committed run.

## Files to MODIFY

- `internal/features/etl_validation/transformer/registry.go` — экспортировать `BuildersForFullRun()` и `BuilderByName()`, если ещё не публичны.

## SQL / Migrations

Нет.

## Run after

```bash
go build ./internal/features/etl_validation/service/...
go test ./internal/features/etl_validation/service/... -race -count=1
go test -tags=integration ./internal/features/etl_validation/service/... -race -count=1
golangci-lint run ./internal/features/etl_validation/service/...
```

## Tests

См. список выше. Coverage ≥85%, integration end-to-end через mock source-adapter обязателен.

## Definition of Done

- [ ] `EtlPipeline.TryStart` + `runAsync` реализованы с goroutine + detached context.
- [ ] Lifecycle `etl_runs` (running → committed/failed) корректно обновляется в любом исходе.
- [ ] Quality threshold 1% реализован и протестирован.
- [ ] `Retry` отказывает для статусов отличных от `failed`/`aborted`.
- [ ] `MartRefresh` поддерживает только `mart_supplier_scorecard`.
- [ ] All sentinel matrix (см. design-tests.md §4) покрыта тестами.
- [ ] Integration test через mock source-adapter проходит.
- [ ] Coverage ≥85%.
- [ ] `golangci-lint`/`go vet` зелёные.

## Зависимости

Требует фаз 06 (repository), 09 (validation engine), 10 (extractor), 11 (transformer), 12 (loader).
