# Phase 14: ExportsStorage (local FS) + POST /v1/exports + GET /v1/exports/{id} + cleanup-cron

**Цель:** для ответов > 50 MB (или по явному запросу формата `parquet`) — async-export. Принимаем POST → 202 с `exportId` → пишем файл в local FS (`/var/exports/{id}.<format>` + `{id}.meta.json`). GET отдаёт через Fiber static (с JWT) или 202 если ещё в работе. Cleanup — отдельный cron-job (TTL из env, default 24h).

**Commit:** `feat(data_export/exports): local FS storage + POST/GET /v1/exports + cleanup-cron`

---

## Files to CREATE

### Storage интерфейс и реализация

- `internal/features/data_export/exports_storage/storage.go`:
  - `type ExportsStorage interface { Put(ctx, id uuid.UUID, format string, r io.Reader, meta Meta) (path string, err error); Get(ctx, id uuid.UUID) (path string, meta Meta, err error); Delete(ctx, id uuid.UUID) error; ListExpired(ctx, before time.Time) ([]uuid.UUID, error) }`.
  - `type Meta struct{ Entity, Format, SnapshotID string; Requester string; CreatedAt time.Time; SizeBytes int64 }`.
- `internal/features/data_export/exports_storage/local_fs.go`:
  - `type LocalFSStorage struct{ root string }`.
  - Layout: плоский — `{root}/{id}.{format}` + `{root}/{id}.meta.json` (см. design-integrations §4).
  - Реализация всех методов.
- `internal/features/data_export/exports_storage/local_fs_test.go` — unit:
  - `TestLocalFS_PutGet_Roundtrip`
  - `TestLocalFS_Delete_RemovesBoth`
  - `TestLocalFS_ListExpired_ByCreatedAt`
  - `TestLocalFS_Get_NotFound_ReturnsErrExportNotFound`

### Service и handlers

- `internal/features/data_export/exports/service.go`:
  - `type Service struct{ storage ExportsStorage; repo RepoAPI; engine ValidatorEngine; logger }`.
  - `func (s *Service) StartExport(ctx, req PostExportRequest, requester string) (uuid.UUID, error)` — валидирует request, генерирует `exportId`, стартует горутину с записью в FS, сразу возвращает id.
  - `func (s *Service) GetExportPath(ctx, id) (string, Meta, error)` — `errorspkg.ErrExportNotFound`.
- `internal/features/data_export/handler/exports.go`:
  - `POST /v1/exports` — 202 + `{exportId, status:'pending', location: '/v1/exports/{id}'}`. Только role `x-flow-etl`.
  - `GET /v1/exports/{id}` — если `meta.status='ready'` → редиректит на Fiber static `/files/exports/{id}.{format}` (или streamит сам). 404 если нет. 202 если pending.

### Cleanup-cron

- `internal/features/data_export/scheduler/exports_cleanup.go`:
  - `RegisterCleanupJob(s gocron.Scheduler, storage ExportsStorage, ttl time.Duration)` — каждые 30 минут вызывает `ListExpired(now-ttl)` → `Delete(...)`.
- `internal/features/data_export/scheduler/exports_cleanup_test.go`:
  - `TestCleanup_RemovesExpired`
  - `TestCleanup_KeepsFresh`

### Тесты handler-ов

- `internal/features/data_export/handler/exports_test.go` — unit:
  - `TestPostExport_BadFormat_400_ErrInvalidExportFormat`
  - `TestPostExport_HappyPath_Returns202`
  - `TestGetExport_Pending_Returns202`
  - `TestGetExport_Ready_Returns200OrRedirect`
  - `TestGetExport_NotFound_404`

## Files to MODIFY

- `internal/config/config.go` — добавить `ExportsDir` (default `/var/exports`), `ExportsRetentionH` (default 24).
- `pkg/errorspkg/errors.go` — sentinel `ErrExportNotFound` (404, code `not_found`) — уже определён в фазе 08.
- `Makefile` — таргет `clean-exports` для локального тестирования.

## SQL/Migrations

— нет (без БД-таблицы для экспортов в MVP; meta хранится в JSON-файле рядом).

## Run after

```bash
mkdir -p /tmp/test-exports
EXPORTS_DIR=/tmp/test-exports make test-unit
make build
make lint
```

## Tests in this phase

- 4 теста LocalFSStorage
- 5 тестов handler-ов
- 2 теста cleanup-job

Итого: 11.

## Definition of Done

- [ ] LocalFSStorage реализует Put/Get/Delete/ListExpired идемпотентно.
- [ ] `POST /v1/exports` валидирует формат (`ndjson`, `parquet`).
- [ ] `GET /v1/exports/{id}` отдаёт 200/202/404 корректно.
- [ ] Cleanup-job удаляет expired файлы (TTL из env).
- [ ] Sentinel `ErrExportNotFound` мапится в 404.
- [ ] `make build` / `make test-unit` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/exports): ...`.
