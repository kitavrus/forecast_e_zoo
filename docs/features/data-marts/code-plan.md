# Code Plan: data-marts (Модуль 3) — 6 фаз

```yaml
tier: M
risk: reversible
phases: 6
commit_per_phase: true
```

## Phase 1 — Foundations: constants + DTO + models
**Files:**
- `internal/features/data_marts/constants/marts.go`
- `internal/features/data_marts/models/mart.go` (MartInfo, MartVersion, MartField, MartSchema, MartRow, Cursor)
- `internal/features/data_marts/models/dto/mart_response.go` (response DTOs)

**Definition of Done:**
- [x] constants содержит 5 mart names + LimitDefault/Max + CacheTTL
- [x] models/mart.go компилируется (`go build ./internal/features/data_marts/...`)
- [x] DTO без cycle imports
- [x] git commit `feat(data_marts): phase 1 constants + models + dto`

## Phase 2 — SQL queries (go:embed) + Repository + integration test
**Files:**
- `internal/features/data_marts/sqls/queries/embed.go`
- `internal/features/data_marts/sqls/queries/list_marts_versions.sql`
- `internal/features/data_marts/sqls/queries/current_version.sql`
- `internal/features/data_marts/sqls/queries/select_mart_demand_history.sql`
- `internal/features/data_marts/sqls/queries/select_mart_calculation_input.sql`
- `internal/features/data_marts/sqls/queries/select_mart_kpi_daily.sql`
- `internal/features/data_marts/sqls/queries/select_mart_master_current.sql`
- `internal/features/data_marts/sqls/queries/select_mart_supplier_scorecard.sql`
- `internal/features/data_marts/repository/repository.go` (pgxpool + queries.MustGet)
- `internal/features/data_marts/repository/repository_integration_test.go` (build tag integration; postgres:18-alpine)

**Definition of Done:**
- [x] queries.MustGet работает (тест-смок)
- [x] repository.New(pool) + методы: GetCurrentVersion, ListMartVersions, SelectMartRows
- [x] Integration test: insert etl_run + 1 mart row → SelectMartRows возвращает её (smoke test, можно пропустить локально через SKIP_DOCKER=1)
- [x] git commit `feat(data_marts): phase 2 sqls + repository + integration test`

## Phase 3 — MartReader interface + PG implementation + cache
**Files:**
- `internal/features/data_marts/service/reader.go` (MartReader interface)
- `internal/features/data_marts/service/reader_pg.go` (PG implementation)
- `internal/features/data_marts/service/cache.go` (in-memory cache 60s TTL)
- `internal/features/data_marts/service/schemas.go` (hardcoded Schema map)
- `internal/features/data_marts/service/cache_test.go` (TTL eviction test)

**Definition of Done:**
- [x] MartReader interface: List, Read, GetVersion, GetSchema
- [x] reader_pg.go реализует все 4 метода
- [x] cache.go: get/put with TTL, sync.RWMutex
- [x] schemas.go: 5 mart schemas hardcoded
- [x] go test -race ./internal/features/data_marts/service/... passes
- [x] git commit `feat(data_marts): phase 3 MartReader + PG impl + cache + schemas`

## Phase 4 — Service + Handlers (4 endpoints) + Mappers
**Files:**
- `internal/features/data_marts/service/service.go` (Service struct + NewService)
- `internal/features/data_marts/handler/handler.go` (struct + NewHandler)
- `internal/features/data_marts/handler/list_marts.go` (GET /v1/marts)
- `internal/features/data_marts/handler/get_mart.go` (GET /v1/marts/:name — NDJSON streaming)
- `internal/features/data_marts/handler/get_version.go` (GET /v1/marts/:name/version)
- `internal/features/data_marts/handler/get_schema.go` (GET /v1/marts/:name/schema)
- `internal/features/data_marts/mappers/helpers.go` (MapServiceError)

**Definition of Done:**
- [x] Service.List/Read/GetVersion/GetSchema корректно делегируют в MartReader
- [x] 4 handlers компилируются, c.Bind() где нужно, mart name validation, cursor decode
- [x] mappers.MapServiceError → errorspkg sentinel
- [x] git commit `feat(data_marts): phase 4 service + handlers + mappers`

## Phase 5 — Router + DI Registration
**Files:**
- `internal/features/data_marts/router/router.go` (Deps + Register)
- `internal/routers/routers.go` (добавляем data_marts deps)
- `internal/app/app.go` (DI: построить repo → service → handler → router)

**Definition of Done:**
- [x] router.Register: 4 routes под /v1/marts с JWT + RequireAnyOf(RoleXFlowETL, RoleITRead)
- [x] internal/app/app.go: создаём data_marts.Repository, Service, Handler, передаём в routers.Register
- [x] internal/routers/routers.go: новый параметр dataMartsDeps
- [x] go build ./... проходит
- [x] git commit `feat(data_marts): phase 5 router + DI registration`

## Phase 6 — Tests (handler + service unit) + Validation
**Files:**
- `internal/features/data_marts/handler/handler_test.go` (4 endpoints через app.Test())
- `internal/features/data_marts/service/service_test.go` (mock MartReader, cache hit/miss)

**Definition of Done:**
- [x] Unit tests handler: happy path для 4 endpoints + bad cursor (400) + unknown mart (404) + JWT/role guard
- [x] Unit tests service: name validation, cache hit/miss
- [x] go test -race -count=1 ./internal/features/data_marts/... passes
- [x] go build ./... passes
- [x] golangci-lint run ./internal/features/data_marts/... passes
- [x] git commit `feat(data_marts): phase 6 unit tests + validation`
