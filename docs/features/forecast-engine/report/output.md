# Pipeline Report: forecast-engine (Модуль 5)
**Дата:** 2026-05-07
**Tier:** L (compact mode)

## Что сделано

Реализован forecast-engine: cron 05:00 Europe/Kyiv → читает marts → moving average + seasonality forecast → calculator (reorder point, qty) → constructor (group by supplier, apply MOQ) → пишет в `forecast.*`. 6 endpoints + admin refresh/approve.

## Endpoints

| Method | Path | Auth | Описание |
|---|---|---|---|
| GET | /v1/forecast/runs | admin/etl/it-read | List + filter + cursor |
| GET | /v1/forecast/runs/:id | admin/etl/it-read | Single run |
| POST | /v1/forecast/runs/refresh | admin-cli | Ondemand recompute (202/409/503) |
| GET | /v1/replenishment/plans | admin/etl/it-read | List plans |
| GET | /v1/replenishment/plans/:id | admin/etl/it-read | Plan + lines |
| POST | /v1/replenishment/plans/:id/approve | admin-cli | draft → approved (для Модуля 6) |

## Архитектура

Forecaster pluggable interface → MovingAverageForecaster (SMA30 × DOW × WOY multipliers).
Calculator: `safety = z×stddev×√LT, RP = safety + LT×demand, target = RP + cycle×demand, qty = max(0, target − stock − transit)`.
Constructor: group by (supplier, location), MOQ skip, ceil-multiplier rounding.

## Метрики прогона
- 11 git коммитов (10 фаз + docs)
- 1 миграция (3001_forecast_schema) с partitioning
- 7 новых sentinel + supportMessage коды (FCT-001..007)
- ~50 unit + 1 integration test

## Quality gates
- `go build ./...` — OK
- `go test -race ./internal/features/forecast/...` — OK (5 packages)
- `go test ./...` — full project OK

## Известные ограничения
- Простой moving average forecaster. Pluggable interface готов для ML-замены (Prophet, LightGBM).
- Без A/B testing моделей.
- Без multi-location optimization (transfer between locations).
- Без promo lift modeling.
- Forecast horizon фиксирован 14 дней (configurable через env).

## Артефакты
- design.md, spec-interview, code-plan + status, 10 phase-файлов
- internal/features/forecast/* — реализация
- pkg/errorspkg/errors_forecast.go — sentinel
- forecast.* schema (forecast_runs, forecasts, calculation_lines, replenishment_plans)
