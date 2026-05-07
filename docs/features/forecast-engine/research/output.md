# Research: forecast-engine (Модуль 5)
**Дата:** 2026-05-07
**Mode:** compact

## Контекст
Из draft-plan: «Прогноз → калькуляция → конструктор. Этот модуль, учитывая результаты работы четвертого модуля и данных с 3 и 4, уже занимается непосредственно прогнозируем, калькуляцией и подготовкой данных. Для прогноза.»

Существует детальная спека: `docs/tmp/replenishment/spec-2026-05-06.md` (приводим из неё ключевые требования).

## Architecture flow

```
mart_demand_history (Модуль 2-3) ──┐
mart_calculation_input (Модуль 2-3) ─┤
mart_supplier_scorecard (Модуль 2-3) ─┤  → Forecast → Calculation → Constructor → replenishment_plans
kpi_snapshots (Модуль 4)            ─┘                                          → forecasts
                                                                                → calculation_lines
```

## Tier: L (Large)
- Endpoints: 6+
- Новых сущностей: 4 (forecasts, replenishment_plans, calculation_lines, run_log)
- Миграций: 2 (forecast schema + partitioned tables)
- Внешние интеграции: нет (читает marts напрямую)
- Cron: 05:00 Europe/Kyiv (после KPI 04:00)
- Diff: ~2000-3000 LOC

## Forecast model (MVP — простой)
- Метод: moving average + seasonality (week-of-year + day-of-week multiplier)
- Window: 30 дней истории + 4 предыдущих сезонов (если retention позволяет)
- Output: `forecasts(product_id, location_id, forecast_date, forecast_qty, lower_bound, upper_bound, model_name, confidence)`
- Отдельный pluggable interface `Forecaster` для будущей замены на ML (Prophet, LightGBM)

## Calculation
- Input: `mart_calculation_input` (current stock + supply_spec + order_rule + applicable_rule_id)
- Reorder point: `safety_stock + lead_time_demand`
- Reorder quantity: `target_stock - current_stock + expected_demand_during_lead_time`
- Output: `calculation_lines(product_id, location_id, current_stock, reorder_point, reorder_qty, target_stock, supplier_id, lead_time_days, calculated_at)`

## Constructor (preparation for orders)
- Группирует calculation_lines по supplier
- Применяет min order qty, MOQ, multiplier
- Output: `replenishment_plans(plan_id, supplier_id, location_id, plan_date, total_qty, lines_count, status='draft')` + lines reference

## Endpoints
- GET /v1/forecast/runs (list, filter date)
- GET /v1/forecast/runs/:id (single run + summary)
- POST /v1/forecast/runs/refresh (admin-cli, ondemand recompute)
- GET /v1/replenishment/plans (list)
- GET /v1/replenishment/plans/:id (с lines)
- POST /v1/replenishment/plans/:id/approve (admin-cli) — переход draft → approved (готовится к Модулю 6)

## Schema forecast в той же БД (consistent)
```
forecast.forecast_runs(id, started_at, finished_at, status, snapshot_id refs marts.etl_runs.id)
forecast.forecasts(product_id, location_id, forecast_date, forecast_qty, model_name, run_id) PARTITION BY RANGE forecast_date
forecast.calculation_lines(product_id, location_id, current_stock, reorder_point, reorder_qty, supplier_id, run_id, calculated_at)
forecast.replenishment_plans(plan_id, supplier_id, location_id, plan_date, total_qty, lines_count, status, run_id)
```

## Что уже есть (Модули 1-4)
- pgxpool, JWT, role middleware
- Scheduler (gocron), advisory lock
- marts.* (mart_demand_history, mart_calculation_input, mart_supplier_scorecard, mart_master_current)
- kpi.kpi_snapshots
- pkg/errorspkg, mappers паттерн

## Q-NNN (defaulted)
- Q-001: Cron schedule → 05:00 Europe/Kyiv
- Q-002: Forecast model → moving average + seasonality (MVP); pluggable Forecaster interface
- Q-003: Calculation lead_time source → mart_calculation_input.applicable_rule_id resolved + supplier scorecard
- Q-004: Plans approval workflow → draft → approved (admin) → handed off to Модуль 6
- Q-005: Forecast horizon → 14 дней (configurable)
- Q-006: Confidence interval → simple (avg ± 1.96*stddev)
- Q-007: Read marts → напрямую из БД pgxpool (не HTTP)
- Q-008: Run idempotency → advisory lock + forecast_runs registry (как в Module 1/2)

## Non-goals MVP
- ML модели (Prophet, LightGBM) — interface заложен, реализация позже
- A/B testing моделей
- Multi-location optimization (transfer between locations)
- Promo lift modeling
