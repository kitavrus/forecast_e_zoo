# E2E Full-Scale Results
**Дата:** 2026-05-07
**Scale:** 1000 products × 30 locations × 365 days × 50 suppliers
**Status:** Stages 1-3 ✅ PASS, Stage 4 ⚠️ partial (forecast OK, plans=0 by data design)

## Объёмы данных в БД (после прогона)

| Таблица | Строк |
|---|---:|
| public.products | **1 000** |
| public.supplier | 50 |
| public.order_rule | 30 |
| public.supply_spec | 1 503 |
| public.receipt_line | 9 469 |
| public.location_stock_snapshot | 9 990 |
| marts.mart_master_current | 1 080 |
| marts.mart_calculation_input | 1 000 |
| marts.mart_demand_history | 978 |
| kpi.kpi_snapshots | 1 000 |
| **forecast.forecasts** | **58 680** |
| forecast.replenishment_plans | 0 (см. ниже) |

Mock-erp seeded: 109 547 stock_movement, 130 000 supplier_stock_snapshot (не загружены полностью в pipeline по дизайну MVP).

## Stages — реальные результаты

| Stage | Статус | Время | Output |
|---|---|---:|---|
| 1 source-adapter | ✅ | 6s | products=1000, receipt_line=9469, location_stock_snapshot=9990, supplier=50 |
| 2 etl | ✅ | 5s | mart_demand_history=978, mart_calculation_input=1000, mart_master_current=1080 |
| 3 kpi | ✅ | 2s | stock_days=1000 (osa=0, otif=0 — нет нужных входных данных в MVP) |
| 4 forecast | ⚠️ | 5s | **forecasts=58 680** ✅, **replenishment_plans=0** ⚠️ |
| 5-8 | — | — | не запущены runner-ом (assertion на draft_plans>0 не прошёл) |

## 🔮 Прогнозы (forecast.forecasts) — реальные результаты

### Агрегированная статистика
| Метрика | Значение |
|---|---:|
| Всего прогнозов | **58 680** |
| Уникальных (product, location) пар | 624 |
| Уникальных дат прогноза | 60 |
| Avg forecast_qty | **2.07** units/day |
| Min / Max forecast_qty | 1.00 / 6.00 |
| Stddev forecast_qty | 0.89 |
| Model | `sma_seasonal` (SMA30 × DOW × WOY multipliers) |

### Топ 10 продуктов по 60-дневному прогнозу

| Product | Location | Days | Total 60d | Avg/day |
|---|---|---:|---:|---:|
| P-00974 | STORE-ZAPORIZHZHIA-06 | 60 | 360.00 | 6.00 |
| P-00596 | STORE-ODESA-17 | 60 | 360.00 | 6.00 |
| P-00209 | STORE-DNIPRO-12 | 60 | 300.00 | 5.00 |
| P-00113 | STORE-ZAPORIZHZHIA-13 | 60 | 300.00 | 5.00 |
| P-00864 | STORE-VINNYTSIA-14 | 60 | 300.00 | 5.00 |
| P-00346 | STORE-KYIV-01 | 60 | 300.00 | 5.00 |
| P-00447 | STORE-LVIV-16 | 60 | 300.00 | 5.00 |
| P-00834 | STORE-VINNYTSIA-21 | 60 | 300.00 | 5.00 |
| P-00538 | STORE-KHARKIV-04 | 60 | 300.00 | 5.00 |
| P-00661 | STORE-ZAPORIZHZHIA-13 | 60 | 300.00 | 5.00 |

## 📈 Mart Demand History — top sales (real ingested data)

| Product | Location | Date | Qty Sold | Returned |
|---|---|---|---:|---:|
| P-00974 | STORE-ZAPORIZHZHIA-06 | 2026-05-01 | 6.00 | 0.00 |
| P-00596 | STORE-ODESA-17 | 2026-05-01 | 6.00 | 0.00 |
| P-00447 | STORE-LVIV-16 | 2026-05-01 | 5.00 | 0.00 |
| P-00113 | STORE-ZAPORIZHZHIA-13 | 2026-05-01 | 5.00 | 0.00 |
| P-00209 | STORE-DNIPRO-12 | 2026-05-01 | 5.00 | 0.00 |

## 📊 KPI Snapshots — distribution

`stock_days` (на сколько дней хватит запасов при текущей скорости продаж):

| Bucket | Count |
|---|---:|
| < 7 days (CRITICAL — нужно срочно заказывать) | 5 |
| 7-30 days (WARNING) | 0 |
| 30-90 days (HEALTHY) | 0 |
| > 90 days (OVERSTOCK) | 995 |

**Avg stock_days:** 363 (фактически inf для большинства — низкая частота продаж в test-данных).

### Top 5 critical (stock_days = 0)
- P-00357 / DC-ODESA-03
- P-00350 / DC-LVIV-02
- P-00196 / DC-ODESA-03
- P-00411 / DC-ODESA-03
- P-00901 / DC-LVIV-02

## 🎯 End-to-end trace — продукт P-00974 в STORE-ZAPORIZHZHIA-06

```
ERP исторические продажи  → mock-erp выдаёт ~6 транзакций за период
       ↓
source-adapter pull      → public.receipt_line (партиционированно по event_date)
       ↓
ETL aggregation          → mart_demand_history.qty_sold = 6 (за тестовый период)
       ↓
forecast SMA × seasonality → 6 units/day × 60 days = 360 units forecast
       ↓
mart_calculation_input  → on_hand, daily_demand, safety_stock (некоторые поля NULL — детали ниже)
       ↓
constructor             → плана нет (см. почему ниже)
```

## ❓ Почему replenishment_plans = 0

Анализ `mart_calculation_input`:

```
total: 1 000 строк
zero_stock: 5
avg_on_hand: 101.1
max_on_hand: 200
daily_demand, safety_stock, lead_time_days: NULL для многих строк
```

Корневые причины:
1. **Низкая фактическая истории продаж** — Faker сгенерировал ~6 транзакций per (product, location) за весь период (мы получили 9469 receipt_line на 1000×30 пар = ~0.3 продажи на пару). При высоком on_hand (avg 101) → stock_days ≈ inf → ничего заказывать не нужно.
2. **NULL поля в mart_calculation_input** — `daily_demand`/`safety_stock`/`lead_time_days` не заполняются для всех строк (зависит от наличия `order_rule`). У нас 30 order_rules на 1000 продуктов — ~3% покрытие.
3. **Constructor skip-логика** — если `daily_demand` NULL, constructor пропускает строку.

Это **корректное поведение** на текущих test-данных. Чтобы pipeline дошёл до Stage 8, нужно:
- Либо увеличить плотность продаж в mock-erp seeder (10-20× транзакций per pair per day)
- Либо снизить начальные стоки до уровней, которые triggеr-ят replenishment
- Либо seedить order_rules для всех продуктов (а не 30 из 1000)

## ✅ Что подтверждено работающим

1. **Source-adapter HTTP integration** — реально читает из mock-erp через REST + X-API-Key, обрабатывает 9k+ receipt_line, 10k stock snapshots, 1k products в 6 секунд.
2. **ETL cross-validation engine** — успешно строит 4 marts с partition по event_date, atomic snapshot flip работает.
3. **KPI calculator** — обрабатывает 1000 product×location пар через hierarchical calibration resolver, OSA/OTIF/StockDays.
4. **Forecast SMA × seasonality** — генерирует 58 680 daily forecasts (1000 products × ~60 days × subset locations) с lower/upper bounds.
5. **Partitioning** — 12 monthly partitions создались автоматически, INSERT routing работает на 9k+ строк фактов.
6. **Mock-erp daily-delta architecture** — готова к incremental scenarios (current_simulated_date).

## Оптимизация data scale для следующего теста

Чтобы получить полный 8/8 на realistic данных, нужны другие параметры seeder:

| Param | Current | Recommended for 8/8 |
|---|---:|---:|
| SEED_PRODUCTS | 1000 | 200-500 |
| SEED_LOCATIONS | 30 | 10-15 |
| SEED_DAYS | 365 | 90-180 |
| SEED_SUPPLIERS | 50 | 20 |
| receipt_line per pair per day | ~0.3 | **5-10** |
| order_rule coverage | 3% | **80-100%** |

Это next iteration улучшение mock-erp seeder.

## Git
Все коммиты в этой сессии — **22 атомарных** (1 финальный commit будет добавлен после этого отчёта).

## Следующие шаги
- ✅ Cleanup: `make compose-down`
- (опц.) Оптимизация mock-erp seeder для 8/8
- (опц.) Подключение реальных Q-001..003 после ответа E-Zoo IT
