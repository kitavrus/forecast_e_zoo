# Phase 15 — Admin handlers + Router + DI

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Реализовать admin endpoints (Fiber v3) с JWT/role middleware (`admin-cli`) и audit_access middleware. Регистрация всех роутов и финальная DI в `internal/etlapp/app.go`. Coverage пакета `handler` через integration-тесты ≥90% (CLAUDE.md mandate).

## Commit

```
feat(etl_validation): admin handlers, router with JWT/audit middleware, full DI wiring
```

## Files to CREATE

### Handlers (`internal/features/etl_validation/handler/`)

- `handler.go` — `Handler` struct + `NewHandler(svcRun service.EtlRun, svcRefresh service.MartRefresh, svcRejectLog RejectLogService, validator validators.Validator) *Handler`.
- `admin_etl_runs_post.go` — `POST /admin/etl-runs` → `service.EtlRun.TriggerRun` → 202 + JSON; mapping ошибок через mappers.
- `admin_etl_runs_retry.go` — `POST /admin/etl-runs/:id/retry`.
- `admin_etl_runs_get_by_id.go` — `GET /admin/etl-runs/:id`.
- `admin_etl_runs_list.go` — `GET /admin/etl-runs?status=&kind=&cursor=&limit=`.
- `admin_marts_refresh.go` — `POST /admin/marts/:name/refresh`.
- `admin_reject_log.go` — `GET /admin/reject-log?etl_run_id=&entity=&severity=&cursor=&limit=`.
- `healthz.go` — `GET /healthz` → 200 + `{"status":"ok","db":"up","scheduler":"up"}`.
- `metrics.go` — `GET /metrics` (если используется promhttp в Fiber v3 через `adaptor.HTTPHandler` — финализация в фазе 16).

### Mappers (`internal/features/etl_validation/mappers/`)

- `helpers.go`:
  - `MapServiceError(err error) error` — common dispatcher: `errorspkg.WriteJSON` обёртка.
  - `MapTriggerRunError(err error)` — особый case `ErrEtlRunAlreadyRunning` → 409 + дополнительное поле `current_run_id`.
  - `MapRetryError(err error)` — `ErrCannotRetryEtl` → 409, `ErrEtlRunNotFound` → 404.
  - `MapMartRefreshError(err error)` — `ErrMartRefreshNotSupported` → 400.

### Router (`internal/features/etl_validation/router/`)

- `router.go`:
  - `Register(group fiber.Router, h *Handler, mw Middlewares)`:
    - `g := group.Group("/admin", mw.JWT, mw.Role("admin-cli"), mw.Audit)`
    - `g.Post("/etl-runs", h.PostEtlRun)`
    - `g.Post("/etl-runs/:id/retry", h.RetryEtlRun)`
    - `g.Get("/etl-runs/:id", h.GetEtlRun)`
    - `g.Get("/etl-runs", h.ListEtlRuns)`
    - `g.Post("/marts/:name/refresh", h.RefreshMart)`
    - `g.Get("/reject-log", h.ListRejectLog)`
    - public:
      - `group.Get("/healthz", h.Healthz)`
      - `group.Get("/metrics", promhttp.Handler())` (через `adaptor.HTTPHandler`).
- `middleware.go`:
  - `Middlewares` struct — `JWT fiber.Handler`, `Role func(string) fiber.Handler`, `Audit fiber.Handler`.
- `audit.go`:
  - Audit middleware — после ответа INSERT в `marts.audit_access` (method, path, sub, role, request_id, status_code, latency_ms).

### DI (`internal/etlapp/app.go` — финализация)

- `New(ctx, cfg, logger)`:
  - pgxpool из `config/db`.
  - Repository.
  - Extractor (TokenSource, Client, Snapshots, Entities).
  - Validation engine (load YAML).
  - Transformer registry.
  - Loader.
  - EtlPipeline service.
  - EtlRun service.
  - MartRefresh service.
  - RejectLog service (read-only repository wrapper).
  - Validators.
  - Handler.
  - Middleware (JWT — using `internal/middleware/jwt`, Role, Audit).
  - Fiber app + router.Register.
  - Scheduler.
- `Run(ctx)`:
  - go scheduler.Start(ctx).
  - app.Listen.
  - на ctx.Done — graceful shutdown (scheduler.Stop, app.Shutdown, pool.Close).

### Tests (integration)

- `internal/features/etl_validation/handler/admin_etl_runs_integration_test.go`:
  - `TestPostEtlRun_Happy` → 202 + run_id.
  - `TestPostEtlRun_AlreadyRunning` → 409 + supportMessage=`EV-001` + current_run_id.
  - `TestPostEtlRun_NoJWT` → 401.
  - `TestPostEtlRun_WrongRole` → 403.
  - `TestRetry_NotFound` → 404, `EV-002`.
  - `TestRetry_StatusNotFailed` → 409, `EV-003`.
  - `TestGetByID_Success` → 200 + полный EtlRunResponse.
  - `TestList_FilterByStatus` → only running.
- `internal/features/etl_validation/handler/admin_marts_integration_test.go`:
  - `TestRefresh_NotSupported` → 400, `EV-005`.
  - `TestRefresh_Scorecard_Happy` → 202.
- `internal/features/etl_validation/handler/admin_reject_log_integration_test.go` — list filter.
- `internal/features/etl_validation/handler/healthz_test.go` — 200 + JSON.
- `internal/features/etl_validation/handler/middleware_test.go` — 401/403 покрытие.
- `internal/features/etl_validation/handler/audit_test.go` — после успешного запроса в `marts.audit_access` появляется запись.

> Все integration-тесты используют общий suite (`pkg/dockertestpkg`), Fiber `app.Test()`, JWT helper генерирует tokens с ролью `admin-cli` или `viewer` для negative cases.

## Files to MODIFY

- `internal/etlapp/app.go` — финальная DI как описано выше.
- `internal/etlapp/deps/deps.go` — реальная инициализация (вместо stub-ов фазы 01).

## SQL / Migrations

Нет.

## Run after

```bash
go build ./...
go test -tags=integration ./internal/features/etl_validation/handler/... -race -count=1 -coverprofile=handler.out
go tool cover -func=handler.out | grep total
# expected: ≥ 90.0%
golangci-lint run ./...
make swagger   # перегенерация swagger.json после новых @Param/@Success
```

## Tests

| Test | Status code | supportMessage |
|---|---|---|
| Post happy | 202 | — |
| Post already_running | 409 | EV-001 |
| Post no JWT | 401 | — |
| Post wrong role | 403 | — |
| Retry not found | 404 | EV-002 |
| Retry status committed | 409 | EV-003 |
| Refresh not supported | 400 | EV-005 |
| Refresh scorecard happy | 202 | — |
| List reject_log filter | 200 | — |
| Healthz | 200 | — |
| Metrics | 200 | — |

## Definition of Done

- [ ] Все 7 endpoint-ов реализованы и подключены в router.
- [ ] JWT/role/audit middleware применены к `/admin/*`.
- [ ] Audit middleware пишет в `marts.audit_access` (verified в integration-тесте).
- [ ] Coverage пакета `handler` ≥90% (CLAUDE.md gate).
- [ ] Полная DI в `internal/etlapp/app.go` — `cmd/etl` собирается, стартует, обрабатывает запросы, gracefully завершается.
- [ ] `make swagger` отработал без ошибок, swagger.json обновлён.
- [ ] `go test -tags=integration ./... -race -count=1` зелёный.
- [ ] `golangci-lint run ./...` зелёный.

## Зависимости

Финальная фаза подключения. Требует все предыдущие.
