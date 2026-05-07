# Phase 08: Repository (pgx + go:embed) + integration tests (dockertest)

**Цель:** реализовать слой `repository`, который на pgx/v5 + pgxpool выполняет SQL из фазы 06 и возвращает доменные модели из фазы 05. Все методы — `(ctx, ...) (..., error)`. Sentinel-ошибки маппятся через `pkg/errorspkg`. Покрытие — **5+ интеграционных сценариев** через dockertest postgres:18-alpine.

**Commit:** `feat(data_export/repository): pgx repository + integration tests (dockertest postgres:18-alpine)`

---

## Files to CREATE

### Repository код

- `internal/features/data_export/repository/repository.go` — `type Repository struct{ pool *pgxpool.Pool }`. Конструктор `New(pool)`.
- `internal/features/data_export/repository/loads.go`:
  - `InsertRunning(ctx, source string) (Load, error)`
  - `MarkCommitted(ctx, loadID, linesTotal, linesFailed int64, entityStats jsonb) error`
  - `MarkFailed(ctx, loadID uuid.UUID, reason string) error`
  - `MarkAborted(ctx, staleAfter time.Duration) (int, error)` — возвращает кол-во помеченных стейл-записей.
  - `GetByID(ctx, loadID) (Load, error)` → `errorspkg.ErrLoadNotFound`.
  - `GetRunning(ctx) (*Load, error)` (nil если нет).
- `internal/features/data_export/repository/snapshot.go`:
  - `Seed(ctx) error` (идемпотентен)
  - `GetCurrent(ctx) (SnapshotPointer, error)` → `errorspkg.ErrSnapshotNotReady` если `current_load_id IS NULL`.
  - `Flip(ctx, tx pgx.Tx, loadID uuid.UUID) (SnapshotPointer, error)` — выполняется внутри транзакции loader-а.
- `internal/features/data_export/repository/locks.go`:
  - `TryAdvisoryLock(ctx, tx pgx.Tx, key int64) (bool, error)`
  - `AdvisoryUnlock(ctx, tx pgx.Tx, key int64) error`
- `internal/features/data_export/repository/reject_log.go`:
  - `Insert(ctx, e RejectEntry) error`
  - `Select(ctx, filter RejectFilter, cursor models.Cursor, limit int) ([]RejectEntry, models.Cursor, error)`
- `internal/features/data_export/repository/audit_access.go`:
  - `Insert(ctx, e AuditAccessEntry) error`
- `internal/features/data_export/repository/master.go` — методы `SelectProducts`, `SelectCategory`, ... (12 master-сущностей). Каждый — пагинация по `(load_id, pk)`. Также UPSERT-методы: `UpsertProduct(ctx, tx, p)`, `UpsertCategory(...)`, ... (вызываются loader-ом фазы 10).
- `internal/features/data_export/repository/facts.go` — `SelectReceiptLine(ctx, dateFrom, dateTo, cursor, limit)` и т.д. (4 факта). UPSERT-методы для loader-а: `InsertReceiptLineBatch(ctx, tx, batch)`.
- `internal/features/data_export/repository/master_change_log.go` — `Insert(ctx, tx, entry)`, `Select(ctx, filter, cursor, limit)`.
- `internal/features/data_export/repository/repository_test_helpers.go` (build tag `integration`) — общая infrastructure для тестов:
  - `setupPG(t) (*pgxpool.Pool, func())` — поднимает dockertest postgres:18-alpine, прогоняет миграции 0001+0002, возвращает teardown.
  - `seedLoad(t, pool, status string) Load`
  - `seedSnapshot(t, pool, loadID uuid.UUID)`
  - `truncateAll(t, pool)` — очистка между тестами.

### Integration-тесты (build tag `integration`)

Минимум **8 сценариев** (>5 как требует план):

- `repository/loads_test.go`:
  - `TestLoads_InsertRunning_GetByID`
  - `TestLoads_MarkCommitted_StatusTransition`
  - `TestLoads_GetByID_NotFound_ReturnsErrLoadNotFound`
  - `TestLoads_MarkAborted_StaleRows`
- `repository/snapshot_test.go`:
  - `TestSnapshot_GetCurrent_NoSeed_ReturnsErrSnapshotNotReady`
  - `TestSnapshot_FlipAtomicInTx`
  - `TestSnapshot_Flip_RollbackOnError` — проверяет, что при rollback `current_load_id` не меняется.
- `repository/locks_test.go`:
  - `TestAdvisoryLock_TryLock_AcquireRelease`
  - `TestAdvisoryLock_TryLock_BusyReturnsFalse` (две параллельные транзакции).
- `repository/reject_log_test.go`:
  - `TestRejectLog_InsertAndSelect`
  - `TestRejectLog_FilterBySeverity`
- `repository/master_test.go`:
  - `TestUpsertProduct_AndSelect` (учитывая partition pruning не нужен).
- `repository/facts_test.go`:
  - `TestReceiptLine_InsertRoutesToPartition`
  - `TestReceiptLine_SelectByEventDateRange_UsesPartitionPruning` (через `EXPLAIN`).

## Files to MODIFY

- `pkg/errorspkg/errors.go` — добавить sentinel: `ErrLoadNotFound`, `ErrSnapshotNotReady` (503, code `snapshot_not_ready`), `ErrSnapshotNotFound` (404), `ErrLoadAlreadyRunning` (409, code `load_already_running`), `ErrCannotRetry`.
- `pkg/errorspkg/errors_test.go` — добавить кейсы.
- `Makefile` — `test-integration` уже есть; убедиться, что `-tags=integration` применяется.

## SQL/Migrations

— нет.

## Run after

```bash
make build
make test-integration
make lint
```

## Tests in this phase

См. список выше — 13+ integration-тестов покрывают:
1. Insert running → get by id
2. Mark committed (transition)
3. Get by id not found → sentinel
4. Mark aborted stale rows
5. Snapshot не готов → sentinel
6. Flip атомарный
7. Flip rollback
8. Advisory lock acquire/release
9. Advisory lock busy
10. Reject log insert/select
11. Reject log filter severity
12. Master upsert/select
13. Facts insert routes to partition + partition pruning

## Definition of Done

- [ ] Все методы Repository покрыты.
- [ ] Sentinel-ошибки `ErrLoadNotFound`, `ErrSnapshotNotReady`, `ErrLoadAlreadyRunning` маппятся в правильных кейсах.
- [ ] dockertest поднимает PG18-alpine, мигрирует, тесты проходят.
- [ ] Минимум один тест проверяет partition pruning через `EXPLAIN`.
- [ ] Минимум один тест проверяет advisory lock параллелизм.
- [ ] `make test-integration` зелёный (15+ тестов).
- [ ] `make build` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/repository): ...`.
