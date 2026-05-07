# Phase 15: Router + DI registration

**Цель:** собрать всё вместе. `internal/features/data_export/router/router.go` регистрирует все маршруты с правильными middleware (JWT + role + audit для `/admin/*`). `internal/routers/routers.go` агрегирует фичи. `internal/app/app.go` строит DI: pgxpool → repository → snapshot/audit/loader/scheduler/exports → handlers → router. После этой фазы сервис полностью работоспособен end-to-end.

**Commit:** `feat(app): router + DI registration (full wiring data_export feature)`

---

## Files to CREATE

- `internal/features/data_export/router/router.go`:
  - `type Deps struct{ JWT, RoleXFlow, RoleAdmin, RoleITRead, AuditWriter fiber.Handler; ProductsHandler, ..., AdminLoadsHandler, ExportsHandler ... }`.
  - `func Register(app *fiber.App, deps Deps)`:
    - `app.Get("/healthz", deps.HealthzHandler.Get)` (БЕЗ JWT)
    - `app.Get("/metrics", ...)` (БЕЗ JWT, реальный handler в фазе 16)
    - Группа `v1 := app.Group("/v1", deps.JWT)`:
      - `v1.Get("/snapshots/current", deps.SnapshotsHandler.GetCurrent)` (роль x-flow-etl OR it-read)
      - `v1.Get("/products", deps.RoleXFlowOrITRead, deps.ProductsHandler.Get)` ... (по одному маршруту на каждый handler фазы 13)
      - `v1.Post("/exports", deps.RoleXFlow, deps.ExportsHandler.Post)`
      - `v1.Get("/exports/:id", deps.RoleXFlowOrITRead, deps.ExportsHandler.Get)`
    - Группа `admin := app.Group("/admin", deps.JWT, deps.RoleAdmin, deps.AuditWriter)`:
      - `admin.Post("/loads", deps.AdminLoadsHandler.Post)`
      - `admin.Post("/loads/:id/retry", deps.AdminLoadsHandler.Retry)`
      - `admin.Get("/loads/:id", deps.AdminLoadsHandler.GetByID)`
      - `admin.Get("/reject-log", deps.AdminRejectLogHandler.Get)`

- `internal/routers/routers.go`:
  - `func Register(app *fiber.App, dataExportDeps data_export_router.Deps) { data_export_router.Register(app, dataExportDeps); }`. Сейчас один feature, но интерфейс готов под будущие.

- `internal/app/app.go` (расширить из фазы 01):
  - `New(cfg)` — порядок:
    1. `slog` logger.
    2. `pgxpool.New(cfg.DBDsn)`.
    3. `repository.New(pool)`.
    4. Snapshot.Seed (идемпотентный INSERT id=1).
    5. `validation.Load(cfg.ValidationRulesPath)`.
    6. `loader.New(reader, repo, engine, logger)`.
    7. `scheduler.New(cfg, loader, repo, logger)` — РЕГИСТРАЦИЯ jobs БЕЗ старта.
    8. `exports_storage.NewLocalFS(cfg.ExportsDir)`.
    9. `exports.NewService(storage, repo, engine, logger)`.
    10. Hand-shaking middleware (`jwt`, `role`, `request_id`, `audit_writer`).
    11. Создание handler-ов (передача нужных deps).
    12. Сборка `data_export_router.Deps`.
    13. `routers.Register(fiberApp, deps)`.
  - `Run(ctx)` — `scheduler.Start()` параллельно с `fiber.Listen()`.
  - `Shutdown(ctx)` — `scheduler.Stop()` → `fiber.ShutdownWithContext(ctx)` → `pool.Close()`.

- `test/e2e/e2e_test.go` (build tag `integration`) — sanity end-to-end:
  - `TestE2E_Healthz_OK`
  - `TestE2E_Snapshots_NotReady_503` (свежая БД).
  - `TestE2E_AdminLoads_TriggerAndCommit_Then_GetProducts` — POST /admin/loads → ждём committed → GET /v1/products → 200 + items + ETag.
  - `TestE2E_JWTRequired_401`
  - `TestE2E_WrongRole_403`
  - `TestE2E_AuditAccessRecorded_ForAdminCall` (проверяет, что запись в `audit_access` появилась).

## Files to MODIFY

- `internal/app/app.go` (фаза 01) — расширить.
- `cmd/source-adapter/main.go` — без изменений (всё уже через `app.New` / `app.Run`).

## SQL/Migrations

— нет.

## Run after

```bash
docker-compose up -d postgres
make migrate-up
make build
make run &  # smoke
curl -H "Authorization: Bearer $JWT" http://localhost:8080/v1/snapshots/current
make test-integration
```

## Tests in this phase

- 6 e2e-тестов в `test/e2e/e2e_test.go` (поднимают полный стек с dockertest PG + реальным Fiber listener).

## Definition of Done

- [ ] `internal/app/app.go` строит весь граф зависимостей без panic.
- [ ] `internal/features/data_export/router/router.go` регистрирует все маршруты.
- [ ] JWT обязателен на `/v1/*` и `/admin/*` (e2e зелёный).
- [ ] Role-gating работает (e2e зелёный).
- [ ] Audit пишется для `/admin/*` (e2e зелёный).
- [ ] End-to-end: trigger load → committed → GET /v1/products возвращает реальные данные из stub fixtures.
- [ ] `make build` / `make test-integration` зелёные.
- [ ] Коммит атомарный, сообщение `feat(app): router + DI registration`.
