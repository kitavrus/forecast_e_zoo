# Research: kpi-calibration (Модуль 4)
**Дата:** 2026-05-07
**Mode:** compact

## Контекст
Из draft-plan: «KPI и калибровка OSA, OTIF, Stock days. Этот модуль уже работает с витриной, в нем заданы в отдельных таблицах необходимые калибровки, необходимые KPI.»

## KPI definitions (используем индустриальные стандарты)

### OSA (On-Shelf Availability)
Доля time-points, когда товар доступен на полке (qty > target_threshold). Считается per (product_id, location_id, date).
Формула: `OSA = sum(time_in_stock) / total_observation_time`
Требует: `mart_kpi_daily` или `mart_demand_history` с timestamp-snapshot'ами остатков.

### OTIF (On-Time In-Full)
Доля заказов, доставленных в срок и в полном объёме. Считается per supplier per period.
Формула: `OTIF = (заказы доставленные в срок И в полном объёме) / (общее число заказов)`
Требует: `mart_supplier_scorecard` с deliveries и order completeness.

### Stock Days
Days of supply: на сколько дней хватит текущих остатков при текущей скорости продаж.
Формула: `stock_days = current_stock / avg_daily_demand_30d`
Требует: текущий остаток (mart_calculation_input) + продажи за 30 дней (mart_demand_history).

## Калибровки
Каждый KPI имеет калибровочные параметры — правила, пороги, веса:
- OSA: target_threshold (минимальный остаток считающийся «доступным», по умолчанию 1)
- OTIF: tolerance window (на сколько часов опоздание ещё считается on-time), полнота (full_threshold_pct)
- Stock Days: avg_window_days (период для расчёта среднего спроса, default 30), seasonality_adjustment

Калибровки конфигурируемы per supplier / category / location / globally — иерархическая система с overrides. Хранятся в БД (`kpi_calibrations` table).

## Архитектура
- Cron tick (например 04:00 Europe/Kyiv, после ETL run и витрин)
- Читает marts.* через GET /v1/marts/:name (Модуль 3) ИЛИ напрямую из БД (тот же pgxpool)
- Применяет калибровки, считает KPI
- Сохраняет результат в `kpi.kpi_snapshots` table (новая schema kpi)
- API endpoints: GET /v1/kpi/snapshots, GET /v1/kpi/snapshots/:id, GET /v1/kpi/calibrations, PUT /v1/kpi/calibrations/:id (admin), POST /v1/kpi/snapshots/refresh (admin ondemand)

## Что уже есть (после Модулей 1-3)
- pgxpool, JWT, role middleware (admin-cli, x-flow-etl, it-read)
- pkg/errorspkg, mappers паттерн
- Scheduler (gocron), advisory lock pattern
- mart-таблицы готовы (наполняются ETL)
- /v1/marts/:name endpoint (Модуль 3) для read-side
- migrate-up таргеты в Makefile

## Tier: M-L
- Endpoints: 5
- Новых сущностей: 2 (kpi_snapshots, kpi_calibrations)
- Миграции: 1 (CREATE SCHEMA kpi + 2 таблицы + индексы)
- Внешние интеграции: можем читать marts напрямую из БД (внутренний доступ, без HTTP к Модулю 3 — для скорости)
- Cron: 04:00 Europe/Kyiv
- Diff: ~1000-1500 LOC

→ M-tier (с лёгким уклоном в L). Делаем compact (без swimlane), ≤ 8 фаз.

## Открытые вопросы (defaulted в spec)
- Q-001: Cron schedule → 04:00 Europe/Kyiv (после ETL 02:30, mart_kpi_daily обновлен)
- Q-002: Reading marts → напрямую из БД (быстрее, нет HTTP overhead). HTTP /v1/marts/:name остаётся для внешних потребителей.
- Q-003: Calibration overrides hierarchy → global → category → supplier → location (resolution от specific к generic)
- Q-004: KPI snapshot retention → 365d (rolling)
- Q-005: KPI snapshot partitioning → RANGE по as_of_date месячно (как mart_kpi_daily)
- Q-006: Recompute previous days → manual via POST /v1/kpi/snapshots/refresh?from_date=...
- Q-007: Где живёт код → новая feature internal/features/kpi/, в составе cmd/source-adapter binary

## Non-goals MVP
- Web UI для калибровки — только REST/admin
- Real-time KPI updates — только daily snapshot
- Multi-tenant — один клиент
- Custom KPI definitions — только OSA/OTIF/Stock Days
- ML-based forecasting влияния калибровок — только статика
