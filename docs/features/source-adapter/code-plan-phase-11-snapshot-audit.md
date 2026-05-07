# Phase 11: Snapshot service + Audit access writer

**Цель:** выделить snapshot-операции в самостоятельный сервис (тонкая обёртка над repository — для удобства DI и тестов) и реализовать audit-writer как Fiber middleware/post-hook, который пишет ВСЕ запросы к `/admin/*` в `audit_access`. Запросы к `/v1/*` НЕ аудитятся (см. ADR-014).

**Commit:** `feat(data_export/snapshot): сервис атомарного flip + audit-writer для /admin/*`

---

## Files to CREATE

### Snapshot service

- `internal/features/data_export/snapshot/service.go`:
  - `type Service struct{ repo SnapshotRepoAPI; logger *slog.Logger }`.
  - `func (s *Service) Current(ctx) (models.SnapshotPointer, error)` — возвращает `errorspkg.ErrSnapshotNotReady` если не было успешного load-а.
  - `func (s *Service) Flip(ctx, loadID uuid.UUID) (models.SnapshotPointer, error)` — открывает транзакцию, вызывает `repo.Flip`, COMMIT.
  - `type SnapshotRepoAPI interface { ... }` — узкий интерфейс для DI/тестов.
- `internal/features/data_export/snapshot/service_test.go` — unit:
  - `TestSnapshotService_Current_HappyPath`
  - `TestSnapshotService_Current_NotReady_ReturnsSentinel`
  - `TestSnapshotService_Flip_DelegatesToRepo`
  - `TestSnapshotService_Flip_RepoError_Propagates`

### Audit writer

- `internal/features/data_export/audit/writer.go`:
  - `type Writer struct{ repo AuditRepoAPI; logger *slog.Logger }`.
  - `func (w *Writer) Middleware() fiber.Handler` — после `c.Next()` определяет, что путь начинается с `/admin/`, берёт claims из Locals, формирует `AuditAccessEntry` и пишет в БД (best-effort; при ошибке — log, не валит запрос).
  - `func (w *Writer) Insert(ctx, entry models.AuditAccessEntry) error` — экспортируется для прямых вызовов из admin-handler-ов (если нужно).
- `internal/features/data_export/audit/writer_test.go` — unit:
  - `TestAuditMiddleware_AdminPath_WritesEntry`
  - `TestAuditMiddleware_PublicPath_DoesNotWrite` (проверяет `/v1/*` не записывает).
  - `TestAuditMiddleware_DBError_DoesNotFailRequest`
  - `TestAuditMiddleware_NoClaims_StillWrites_RoleEmpty` (если до middleware каким-то образом дошёл анонимный — пишем `actor_role=""`, `actor_sub=""`).

## Files to MODIFY

- `internal/features/data_export/repository/snapshot.go` — `Flip` уже принимает `pgx.Tx` (фаза 08); проверить, что `Service.Flip` корректно открывает/закрывает tx.
- `internal/features/data_export/repository/audit_access.go` — без изменений (фаза 08 уже создала).

## SQL/Migrations

— нет.

## Run after

```bash
make build
make test-unit
make lint
```

## Tests in this phase

- 4 теста в `snapshot/service_test.go`
- 4 теста в `audit/writer_test.go`

## Definition of Done

- [ ] Snapshot.Service инкапсулирует транзакционный flip.
- [ ] Sentinel `ErrSnapshotNotReady` корректно возвращается при отсутствии current.
- [ ] Audit-middleware пишет ТОЛЬКО `/admin/*`.
- [ ] Audit-middleware never-fails request (best-effort).
- [ ] Все unit-тесты зелёные.
- [ ] `make build` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/snapshot): ... + audit ...`.
