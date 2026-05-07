# Code Plan: kpi-calibration (Module 4)

8 фаз, атомарные коммиты, quality gates после каждой.

| Phase | Name | Files | Commit |
|---|---|---|---|
| 1 | Migration 2001 | `internal/features/kpi/sqls/migrations/2001_kpi_schema.{up,down}.sql`, `Makefile` | `feat(kpi): phase 1 migration 2001 — kpi schema` |
| 2 | Sentinel errors + DTO + models | `pkg/errorspkg/{errors,support_codes}.go`, `internal/features/kpi/{constants,models,models/dto}/*.go` | `feat(kpi): phase 2 sentinel errors + dto + models` |
| 3 | SQL queries + Repository + integration test | `internal/features/kpi/sqls/queries/*.sql`, `internal/features/kpi/repository/*.go` | `feat(kpi): phase 3 repository + sql queries` |
| 4 | Calibration resolver | `internal/features/kpi/calibration/resolver.go(_test)?` | `feat(kpi): phase 4 calibration resolver` |
| 5 | KPI calculators | `internal/features/kpi/calculators/{osa,otif,stock_days}.go(_test)?` | `feat(kpi): phase 5 kpi calculators` |
| 6 | Engine + Scheduler | `internal/features/kpi/engine/*.go`, `internal/features/kpi/scheduler/*.go` | `feat(kpi): phase 6 engine + scheduler` |
| 7 | Service + Handlers + Mappers + Router + DI | `internal/features/kpi/{service,handler,mappers,router,validators}/*.go`, `internal/routers/routers.go` | `feat(kpi): phase 7 service + handlers + router + DI` |
| 8 | Prometheus metrics + validation | `internal/observability/metrics.go`, `internal/features/kpi/engine/engine.go` | `feat(kpi): phase 8 prometheus metrics + validation` |

## Detail per phase

### Phase 1 — Migration 2001
- `2001_kpi_schema.up.sql`: schema kpi, kpi_calibrations, kpi_snapshots (partitioned), seed-row dla 3 default global калибровок.
- `2001_kpi_schema.down.sql`: DROP SCHEMA kpi CASCADE.
- Makefile: `migrate-up-kpi`, `migrate-down-kpi`, `migrate-version-kpi` (отдельный target, общий DSN с marts).

### Phase 2 — Errors + DTO + models
- `pkg/errorspkg/errors.go`: добавить ErrKpiSnapshotNotFound, ErrKpiCalibrationNotFound, ErrInvalidKpiName.
- `pkg/errorspkg/support_codes.go`: SupportKpiSnapshotNotFound="KPI-001", SupportKpiCalibrationNotFound="KPI-002", SupportInvalidKpiName="KPI-003" + добавить в SupportMessageCodes.
- `internal/features/kpi/constants/kpi.go`: KpiOSA, KpiOTIF, KpiStockDays + ScopeTypeGlobal/Category/Supplier/Location/ProductLocation + AdvisoryLockKey.
- `internal/features/kpi/models/{snapshot.go,calibration.go}`: domain types.
- `internal/features/kpi/models/dto/*.go`: SnapshotItem, ListSnapshotsResponse, CalibrationItem, UpdateCalibrationRequest, RefreshSnapshotsRequest, RefreshSnapshotsResponse.

### Phase 3 — Repository + SQL
- `sqls/queries/*.sql` (9 файлов из design §4).
- `sqls/queries/embed.go`: go:embed FS.
- `repository/repository.go`: struct + New(*pgxpool.Pool).
- `repository/snapshots.go`: ListSnapshots, GetSnapshotByID, InsertSnapshot, DeleteSnapshotsForRange.
- `repository/calibrations.go`: ListCalibrations, GetCalibrationByID, UpdateCalibration.
- `repository/marts_reader.go`: ReadDemandHistoryAggregates, ReadCalculationInput, ReadSupplierScorecard.
- `repository/repository_integration_test.go`: dockertest postgres:18-alpine, apply migrations 1001 (marts) + 2001 (kpi), CRUD.

### Phase 4 — Calibration resolver
- `calibration/resolver.go`: hierarchical resolver (location > supplier > category > global). 1 round-trip — все calibrations для KPI грузятся в memory, in-process matching.
- `calibration/resolver_test.go`: 5 happy paths (each level of hierarchy), fallback chain, no calibration for KPI → defaults.

### Phase 5 — Calculators
- `calculators/osa.go`: ComputeOSA(rows, params) → []models.Snapshot.
- `calculators/otif.go`: ComputeOTIF(rows, params) → []models.Snapshot.
- `calculators/stock_days.go`: ComputeStockDays(rows, params, supplierResolver) → []models.Snapshot.
- Tests: `*_test.go` — happy + zero-deliveries + division-by-zero + cap.

### Phase 6 — Engine + Scheduler
- `engine/engine.go`: Engine struct (repo, resolver, calculators, logger, metrics interface). Run(ctx, runID, asOfDate, kpis) — orchestration, quality threshold check, write snapshots.
- `engine/engine_test.go`: mock repo + readers, проверить quality threshold (>5% errors → fail) и happy.
- `scheduler/scheduler.go`: gocron + advisory lock pattern (как в data_export). TryTrigger(ctx) — sync acquire, async run, returns acquired bool.
- `scheduler/scheduler_test.go` — basic init + TryTrigger contract.

### Phase 7 — Service + Handlers + Router + DI
- `service/service.go`: KpiService struct. List/Get/Update/Refresh.
- `validators/validators.go`: ValidateRefreshRequest, ValidateUpdateCalibration, ValidateKpiName.
- `mappers/helpers.go`: MapServiceError.
- `handler/handler.go`: struct + NewHandler.
- `handler/{list_snapshots,get_snapshot,list_calibrations,update_calibration,refresh_snapshots}.go`.
- `router/router.go`: Deps struct + Register.
- `internal/routers/routers.go`: добавить kpiRouter в Register сигнатуру (slim, conditional).

### Phase 8 — Prometheus + validation
- `internal/observability/metrics.go`: добавить KpiEngineRunTotal, KpiEngineRunDuration, KpiSnapshotCountTotal, KpiEngineErrorsTotal в allMetrics.
- `engine/engine.go`: использовать observability метрики.
- Validation: go build, go test, golangci-lint.

## Definition of Done (per phase)

- [ ] Файлы созданы.
- [ ] `go build ./...` проходит.
- [ ] Unit-тесты зелёные (где применимо).
- [ ] Атомарный коммит создан.
- [ ] Status в `code-plan-status.md` обновлён на `completed`.
