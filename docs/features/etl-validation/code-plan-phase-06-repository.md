# Phase 06 — Repository (pgx + go:embed) + integration test

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Реализовать слой `repository/` для всех операций с `marts.etl_runs`, `marts.reject_log`, `marts.audit_access`, `marts.mart_*` и staging-таблицами. SQL — только через `go:embed` (см. фазу 07). Integration test через dockertest + `postgres:18-alpine` + общий Suite (`pkg/dockertestpkg`).

## Commit

```
feat(etl_validation): repository layer with pgx + integration suite (postgres:18-alpine)
```

## Files to CREATE

### Production code

- `internal/features/etl_validation/repository/repository.go` — `Repository` aggregate-интерфейс + `New(pool *pgxpool.Pool) *Impl` конструктор.
- `internal/features/etl_validation/repository/etl_runs.go`:
  - `Insert(ctx, run *models.EtlRun) error`
  - `GetByID(ctx, id string) (*models.EtlRun, error)` — `errors.Is(err, errorspkg.ErrEtlRunNotFound)` при miss.
  - `List(ctx, filter ListFilter) ([]models.EtlRun, error)` — cursor pagination.
  - `UpdateStatus(ctx, id string, patch StatusPatch) error` — обновляет `status`, `finished_at`, `committed_at`, `marts_summary`, `failure_reason`, `lines_total`, `lines_failed`.
  - `GetCurrentRunning(ctx) (*models.EtlRun, error)` — последний с status='running' (для error-response 409).
- `internal/features/etl_validation/repository/reject_log.go`:
  - `BulkInsert(ctx, entries []models.RejectLogEntry) error` — `pgx.CopyFrom` или batch insert.
  - `List(ctx, filter RejectFilter) ([]models.RejectLogEntry, error)` — `etl_run_id`, `entity`, `severity`, `from`, `to`, cursor, limit.
- `internal/features/etl_validation/repository/audit_access.go`:
  - `Insert(ctx, entry models.AuditAccessEntry) error`.
- `internal/features/etl_validation/repository/marts.go`:
  - `UpsertDemandHistory(ctx, tx pgx.Tx, runID, sourceLoadID string) (int64, error)` — INSERT … SELECT FROM staging.
  - аналогичные методы для `mart_calculation_input`, `mart_kpi_daily`, `mart_master_current`, `mart_supplier_scorecard`.
  - Каждый метод выполняет соответствующий `.sql` из фазы 07.
- `internal/features/etl_validation/repository/staging.go`:
  - `CreateTempTables(ctx, tx pgx.Tx) error` — `CREATE TEMP TABLE` для всех `stg_*`.
  - `BulkLoad(ctx, tx pgx.Tx, table string, rows <-chan []any) (int64, error)` — `pgx.CopyFrom`.
- `internal/features/etl_validation/repository/lock.go`:
  - `TryAdvisoryLock(ctx, tx pgx.Tx, key int64) (bool, error)` — `SELECT pg_try_advisory_xact_lock($1)`. Используется scheduler-ом и админ-handler-ом.
- `internal/features/etl_validation/sqls/sqls.go` — `embed.FS` объявление: `//go:embed *.sql migrations/*.sql`.

### Tests (integration)

- `internal/features/etl_validation/repository/integration_suite_test.go` — testify/suite поднимает `postgres:18-alpine` через `pkg/dockertestpkg.NewSuite`. Применяет миграции из `internal/features/etl_validation/sqls/migrations/` через `iofs` driver. `TearDownSuite` — pool.Purge.
- `internal/features/etl_validation/repository/etl_runs_integration_test.go` — happy path: Insert → GetByID → UpdateStatus → List. Тест на 409 (повторный insert с тем же `id` через UNIQUE constraint? Если PK — конфликт). Тест на `ErrEtlRunNotFound`.
- `internal/features/etl_validation/repository/reject_log_integration_test.go` — BulkInsert (1000 строк) + List filter (`severity='critical'`).
- `internal/features/etl_validation/repository/audit_access_integration_test.go` — Insert + чтение.
- `internal/features/etl_validation/repository/lock_integration_test.go` — два goroutine одновременно вызывают `TryAdvisoryLock(ctx, tx, sameKey)`: первый получает true, второй — false. После Commit/Rollback первой tx — второй goroutine получает true.
- `internal/features/etl_validation/repository/marts_integration_test.go` — наполнение staging → UpsertDemandHistory → SELECT * FROM marts.mart_demand_history, проверка `etl_run_id` и `source_load_id`.

> Build tag: все integration-файлы с `//go:build integration`.

## Files to MODIFY

- `pkg/dockertestpkg/suite.go` (если требуется) — добавить опцию для `migrations`-пути из feature (если ещё не поддерживается). Либо новый конструктор `NewSuiteWithMigrations(t, fsys fs.FS, dir string)`.

## SQL / Migrations

SQL-файлы создаются в фазе 07. На фазе 06 — только пустые stub-файлы для go:embed (или ссылка на already existing если фаза 07 идёт раньше — но порядок: 06 раньше 07 потому что repository нужен для compile-смокa последующих фаз; фактически SQL вынесен в фазу 07, но пустые `.sql`-файлы можно создать в фазе 06 как `SELECT 1;` placeholders, либо объединить — оставляем разделение). Фактически упрощение: в фазе 06 создаём SQL-стабы (просто чтобы embed не падал), полное наполнение — в фазе 07.

> Решение: SQL-файлы заполняются в фазе 07. В фазе 06 создаём в `sqls/` файлы с минимально валидным SQL (`SELECT 1;`) только для тех queries, которые нужны репозиторию для компиляции. Финальное наполнение — в 07.

Альтернатива: переименовать фазу 07 в "SQL queries (полное наполнение)" и в фазе 06 положить минимально работающие версии. Используем эту схему.

## Run after

```bash
go build ./internal/features/etl_validation/repository/...
go test -tags=integration ./internal/features/etl_validation/repository/... -count=1 -race
go vet ./internal/features/etl_validation/repository/...
golangci-lint run ./internal/features/etl_validation/repository/...
```

## Tests

| Test | Что проверяет |
|---|---|
| `TestEtlRunsRepository_InsertGetUpdateList` | happy path |
| `TestEtlRunsRepository_GetByID_NotFound` | `ErrEtlRunNotFound` |
| `TestEtlRunsRepository_UpdateStatus_Committed` | committed_at заполняется |
| `TestRejectLog_BulkInsert_1000Rows` | производительность + integrity |
| `TestRejectLog_ListByFilter` | severity, entity filtering |
| `TestAuditAccess_Insert` | базовая |
| `TestLock_TryAdvisoryLock_Contention` | concurrency: lock contention |
| `TestMarts_UpsertDemandHistory` | etl_run_id и source_load_id корректно |

## Definition of Done

- [ ] Все интерфейсы и реализации в `repository/` созданы.
- [ ] go:embed SQL подключён (`sqls/sqls.go`).
- [ ] Integration suite поднимает `postgres:18-alpine` и применяет миграции из feature.
- [ ] Все integration-тесты проходят (`go test -tags=integration ./internal/features/etl_validation/repository/... -race -count=1`).
- [ ] Lock contention test проходит детерминированно.
- [ ] `golangci-lint` зелёный (включая `sqlclosecheck`, `rowserrcheck`, `noctx`).
- [ ] Coverage пакета `repository` ≥80%.

## Зависимости

Требует фаз 02 (sentinel errors), 03 (schema marts), 04 (etl_runs/reject_log/audit_access).
