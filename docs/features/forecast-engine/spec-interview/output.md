# Spec Interview: forecast-engine (Module 5)

**Дата:** 2026-05-07
**Mode:** compact (defaulted из research, без интерактивного интервью — auto-mode)
**Tier:** L

> Все ответы ниже — defaulted из `research/output.md` (Q-001..Q-008) + spec-2026-05-06.md.
> Эскалация к юзеру по любому из ответов — на следующей итерации.

## 1. Проблема и цель (PRD inline)

Автоматический ежедневный расчёт прогноза спроса, точек заказа и черновых планов пополнения для всех (location, sku) пар. Снимает ручную работу категорийного менеджера, повышает OSA и снижает overstock. Хранит прогноз и план в БД, отдаёт через REST API. Module 6 (Order/EDI dispatch) подхватывает approved-планы.

**Success metrics (post-launch, 6 мес):**
- доля автозаказов ≥ 80% (planов с verdict=auto без manual override)
- OSA ≥ 95% B-класс (см. spec §1)
- p95 run duration ≤ 10 минут (~150 локаций × 30k SKU)

## 2. Сценарии (defaulted)

- **Happy path:** cron 05:00 → engine.Run → forecasts/lines/plans записаны → API exposed.
- **Refresh manually:** admin-cli POST /v1/forecast/runs/refresh → 202 либо 409.
- **Approve plan:** admin-cli POST /v1/replenishment/plans/:id/approve → status='approved', timestamp.
- **Race condition:** 2 cron tick'a → второй получает busy lock → skip.

## 3. Ответы на defaulted Q

| Q | Ответ |
|---|---|
| Q-001 cron | `0 5 * * *` Europe/Kyiv (configurable `FORECAST_CRON_SCHEDULE`) |
| Q-002 модель | SMA30 + dow_multiplier × woy_multiplier; pluggable `Forecaster` interface |
| Q-003 lead_time | из `mart_calculation_input.lead_time_days`; fallback 7д |
| Q-004 approval | draft → approved (admin); cancelled через future endpoint |
| Q-005 horizon | 14 дней default, configurable `FORECAST_HORIZON_DAYS` (1..60) |
| Q-006 confidence | `lower_bound = forecast × (1 - 0.2)`, `upper_bound × 1.2`, confidence=0.8 (MVP placeholder) |
| Q-007 marts | direct pgxpool, same БД |
| Q-008 idempotency | advisory lock 0x4643544552474E45 + run registry |

## 4. Открытые вопросы (для будущих итераций)

- Backfill historical runs при rule change — explicitly не делаем (§8.5).
- Cancel plan endpoint — отложено до Module 6 hand-off.
- Per-supplier override horizon — пока не нужно, single global config.
- ML upgrade path — interface заложен, реализация в v2.

## 5. Принятые компромиссы

- Confidence interval — placeholder ±20%; реальные квантили из остатков SMA — post-MVP.
- Single forecaster per run — нет model A/B, нет ensemble.
- Plans пишутся даже если `total_qty=0` — фильтр happens client-side.
