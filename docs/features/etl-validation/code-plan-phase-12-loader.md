# Phase 12 — Loader + atomic flip

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Реализовать loader, который атомарно (в одной транзакции):
1. Выполняет UPSERT/INSERT-FROM-SELECT через builder-ы (фаза 11).
2. Обновляет `marts.etl_runs.status='committed'`, `committed_at=NOW()`, `marts_summary={mart_name: rows}`, `lines_total`, `lines_failed`.
3. (Опционально) удаляет «висящие» строки от прерванных предыдущих runs (`unmark previous etl_run_id`) — формально не требуется при append-семантике, но snapshot-flip pattern сохраняется как hook для будущего bi-temporal recompute.

## Commit

```
feat(etl_validation): loader with atomic flip in single transaction
```

## Files to CREATE

### Production code

- `internal/features/etl_validation/loader/loader.go`:
  - `Loader` интерфейс — `Apply(ctx, run *models.EtlRun, builders []transformer.Builder, summary BuildSummary) error`.
  - `Impl` — оборачивает `*pgxpool.Pool`, `repository.EtlRuns`, `repository.Marts`.
  - В одной `pool.BeginTx`:
    - для каждого builder-а вызвать `Build(ctx, tx, runID, sourceLoadID)` и накопить `BuildSummary` (rows per mart).
    - Вызвать `repo.EtlRuns.UpdateStatus(ctx, tx, runID, StatusPatch{Status: committed, CommittedAt, FinishedAt, MartsSummary, LinesTotal, LinesFailed})`.
    - `tx.Commit()` — атомарно.
  - При любой ошибке в tx: `tx.Rollback()` + возвращаем error → pipeline помечает run как failed (см. фаза 13).
- `internal/features/etl_validation/loader/flip.go`:
  - `Flip` extension point — для будущего bi-temporal recompute (сейчас no-op, hook на интерфейс).
- `internal/features/etl_validation/loader/summary.go`:
  - `BuildSummary` struct — `MartName → RowsWritten`. Marshal в JSONB для `marts_summary`.

### Tests (integration)

- `internal/features/etl_validation/loader/loader_integration_test.go`:
  - `TestLoader_Apply_Success` — все 4 full-run builder-а отрабатывают, etl_run.status='committed', marts_summary заполнен.
  - `TestLoader_Apply_BuilderError_Rollback` — один builder возвращает ошибку → tx.Rollback() → mart-таблицы пусты, etl_run.status НЕ committed (остаётся running, pipeline помечает failed).
  - `TestLoader_Apply_AtomicCommit` — параллельный SELECT во время Apply не видит частично загруженные строки до Commit.

## Files to MODIFY

- (нет)

## SQL / Migrations

Использует существующие queries из фазы 07.

## Run after

```bash
go build ./internal/features/etl_validation/loader/...
go test -tags=integration ./internal/features/etl_validation/loader/... -race -count=1
golangci-lint run ./internal/features/etl_validation/loader/...
```

## Tests

| Test | Что проверяет |
|---|---|
| `TestLoader_Apply_Success` | happy path, atomic commit |
| `TestLoader_Apply_BuilderError_Rollback` | rollback при ошибке |
| `TestLoader_Apply_AtomicVisibility` | snapshot isolation работает |
| `TestLoader_BuildSummary_JSONB` | `marts_summary` корректно сериализован |

## Definition of Done

- [ ] `Loader.Apply` в одной транзакции делает все INSERT + UPDATE etl_runs.
- [ ] При ошибке любого builder-а — полный rollback.
- [ ] `marts_summary` JSONB содержит счётчики по каждому mart-у.
- [ ] Integration-тесты подтверждают атомарность.
- [ ] Coverage ≥85%.
- [ ] `golangci-lint` зелёный.

## Зависимости

Требует фаз 06 (repository), 07 (SQL), 11 (transformer/builders).
