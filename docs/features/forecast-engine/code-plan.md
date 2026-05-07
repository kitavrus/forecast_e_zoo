# Code Plan: forecast-engine (Module 5)

**Дата:** 2026-05-07
**Tier:** L (compressed to 10 phases)
**Mode:** compact

| # | Фаза | Status |
|---|---|---|
| 1 | Migration 3001 — schema forecast + 4 таблицы + indexes | completed |
| 2 | Sentinel errors + DTO + models + constants | completed |
| 3 | SQL queries + Repository (forecast.* + marts read) | completed |
| 4 | Forecaster interface + MovingAverageForecaster + unit tests | completed |
| 5 | Calculator (RP, safety, target, qty) + unit tests | completed |
| 6 | Constructor (group by supplier, MOQ, multiplier) + unit tests | completed |
| 7 | ForecastEngine orchestration (marts→F→C→K→write) + unit test | completed |
| 8 | Scheduler (gocron 05:00 + advisory lock) + tests | completed |
| 9 | Service + Handlers (6 endpoints) + mappers/validators/router/DI | completed |
| 10 | Prometheus metrics + final validation | completed |

## Quality gates (passed)

```
go build ./...                                                # OK
go test -race ./internal/features/forecast/...                # OK (5 packages)
go test ./...                                                  # OK (full project)
```

## Definition of Done

- [x] Все 10 фаз реализованы и проходят quality gates
- [x] 6 endpoints зарегистрированы в `/v1/forecast/*` и `/v1/replenishment/*`
- [x] Forecaster pluggable (interface + MovingAverageForecaster impl)
- [x] Schema forecast с partitioning по forecast_date RANGE month
- [x] Advisory lock 0x4643544552474E45 ("FCTERGNE")
- [x] Cron 05:00 Europe/Kyiv (configurable)
- [x] Prometheus метрики: run_total/duration, forecasts/lines/plans counters, errors
- [x] Sentinel errors FCT-001..FCT-007 с support codes
- [x] DI wired в `internal/app/app.go` + `internal/routers/routers.go`
- [x] Конфиг `FORECAST_CRON_SCHEDULE`, `FORECAST_CRON_TZ`, `FORECAST_HORIZON_DAYS`
