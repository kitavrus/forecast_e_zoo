# Planner: forecast-engine (Module 5)

**Дата:** 2026-05-07
**Tier:** L (compressed to ≤10 phases)

## Phases

| # | Фаза | Файлы | Status |
|---|---|---|---|
| 1 | Migration 3001 — schema forecast + 4 таблицы + indexes | `internal/features/forecast/sqls/migrations/3001_forecast_schema.{up,down}.sql` + embed.go | completed |
| 2 | Sentinel errors + DTO + models + constants | `pkg/errorspkg/errors_forecast.go`, `support_codes.go`, `internal/features/forecast/{models,models/dto,constants}/*` | completed |
| 3 | SQL queries + Repository (forecast.* + marts read) + integration test | `internal/features/forecast/{sqls/queries,repository}/*` | completed |
| 4 | Forecaster interface + MovingAverageForecaster impl + unit tests | `internal/features/forecast/forecaster/*` | completed |
| 5 | Calculator (reorder_point, reorder_qty) + unit tests | `internal/features/forecast/calculator/*` | completed |
| 6 | Constructor (group by supplier, MOQ, build plans) + unit tests | `internal/features/forecast/constructor/*` | completed |
| 7 | ForecastEngine service (orchestration: marts → Forecaster → Calculator → Constructor → write) + unit test | `internal/features/forecast/engine/*` | completed |
| 8 | Scheduler (gocron 05:00 + advisory lock) + tests | `internal/features/forecast/scheduler/*` | completed |
| 9 | Service + Handlers (6 endpoints) + mappers + validators + router + DI in app | `internal/features/forecast/{service,handler,mappers,validators,router}/*` + `internal/app/app.go` + `internal/routers/routers.go` + `internal/config/*` | completed |
| 10 | Prometheus metrics + final validation (build + lint + test) | `internal/observability/metrics.go` + `internal/features/forecast/engine/metrics_adapter.go` | completed |

## Hard invariants
- Каждая фаза → атомарный git commit `feat(forecast): phase N <название>`
- После каждой фазы: `go build ./...` зелёный
- Integration test уровня repository — `postgres:18-alpine` через dockertest
- Все queries — go:embed `*.sql` файлы

## Quality gates
```bash
go build ./...
go test -race ./internal/features/forecast/...
golangci-lint run ./...  # (опционально, если конфигурация прогнозируема)
```
