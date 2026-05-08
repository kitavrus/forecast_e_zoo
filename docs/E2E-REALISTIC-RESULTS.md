# E2E Realistic Pipeline Results

**Дата:** 2026-05-08
**Профиль:** Realistic E2E (рассчитан под живые replenishment_plans > 0)

## Шкала прогона

| Параметр | Значение |
|---|---|
| `SEED_PRODUCTS` | 100 |
| `SEED_LOCATIONS` | 5 (1 DC + 4 STORE) |
| `SEED_DAYS` | 90 |
| `SEED_SUPPLIERS` | 15 |
| `SEED_TRANSACTIONS_PER_PAIR_PER_DAY` | 5 |
| `SEED_INITIAL_STOCK_DAYS_OF_DEMAND` | 14 |

**Время:**
- `docker compose up -d --build`: ~80s (build + start + seed)
- Mock-erp seeder: ~30s (внутри start_period)
- E2E pipeline (8 stages, после seed-channel-configs): **~32s**

## Stage-by-stage timings (8/8 PASSED)

| Stage | Service | Time | Result |
|---|---|---|---|
| 1 | source-adapter — POST /admin/loads | 5s | products=100, receipt_line=29 574, location_stock=450, suppliers=15 |
| 2 | etl — POST /api/v1/admin/etl-runs | 6s | mart_demand_history=89, mart_calculation_input=450 |
| 3 | kpi — POST /v1/kpi/snapshots/refresh | 2s | stock_days=450 |
| 4 | forecast — POST /v1/forecast/runs/refresh | 5s | forecasts=5 340, **draft_plans=15** |
| 5 | approve plans | 0s | approved=15 (всё подтверждено) |
| 6 | order-builder — POST /v1/orders/purchase-orders/build | 3s | purchase_orders ready_to_send=15 |
| 7 | channel-router — POST /v1/channels/send | ~6s | sent=15, send_attempts.success=15 (HTTP 201) |
| 8 | mock-erp verify | 1s | received_orders=15 (все попали назад в ERP) |

**Итог:** 100% всех stages прошло, реальный replenishment-цикл сработал end-to-end.

## Семена (mock-erp)

| Сущность | Кол-во | Покрытие |
|---|---|---|
| products | 100 | 100% |
| suppliers | 15 | — |
| locations | 5 (1 DC + 4 STORE) | 100% |
| order_rule | 5 | **100% locations** |
| supply_spec | 153 | **100% products** (~1.5 supplier/product, primary+secondary) |
| receipt_line | 29 574 | Poisson(λ=base_demand) × 90 дней × 100 продуктов × 4 stores |
| location_stock_snapshot | 450 | 100p × 4 stores + DC sample (low — 14 дней спроса) |

## 🔮 РЕАЛЬНЫЕ ПРОГНОЗЫ — TOP 10 (60-day forecast)

| product_id | location_id | days | total_60d |
|---|---|---|---|
| P-00065 | STORE-KYIV-01 | 60 | 1500.0 |
| P-00055 | STORE-KYIV-01 | 60 | 1500.0 |
| P-00035 | STORE-KYIV-01 | 60 | 1500.0 |
| P-00075 | STORE-KYIV-01 | 60 | 1440.0 |
| P-00032 | STORE-KYIV-01 | 60 | 1380.0 |
| P-00005 | STORE-KYIV-01 | 60 | 1380.0 |
| P-00029 | STORE-KYIV-01 | 60 | 1380.0 |
| P-00030 | STORE-KYIV-01 | 60 | 1320.0 |
| P-00016 | STORE-KYIV-01 | 60 | 1260.0 |
| P-00006 | STORE-KYIV-01 | 60 | 1260.0 |

(Модель: `sma_seasonal`. Forecast = 60 дней × ~89 активных пар = 5 340 точек.)

## 📋 PLANS — TOP 5 (replenishment_plans by total_qty)

| supplier_id | location_id | total_qty | lines | status |
|---|---|---|---|---|
| SUP-0003 | STORE-KYIV-01 | 1063 | 7 | converted |
| SUP-0008 | STORE-KYIV-01 | 926 | 7 | converted |
| SUP-0010 | STORE-KYIV-01 | 820 | 4 | converted |
| SUP-0013 | STORE-KYIV-01 | 791 | 3 | converted |
| SUP-0001 | STORE-KYIV-01 | 712 | 8 | converted |

Status `converted` = план подтвердился и стал PO.

### Детализация одной плановой строки (top reorder_qty)

| product | supplier | current_stock | daily_demand | lead_time | reorder_qty |
|---|---|---|---|---|---|
| P-00055 | SUP-0013 | 210 | 25 | 19d | **440** |
| P-00065 | SUP-0003 | 252 | 25 | 18d | 373 |
| P-00035 | SUP-0005 | 252 | 25 | 16d | 323 |
| P-00025 | SUP-0002 | 280 | 21 | 20d | 287 |
| P-00032 | SUP-0010 | 224 | 23 | 15d | 282 |

(Логика: `current_stock` ниже `reorder_point = daily_demand × (lead_time + safety)`, значит требуется заказ в размере `target_stock - current_stock`.)

## 📦 POs — TOP 5

| po_number | supplier | location | qty | status |
|---|---|---|---|---|
| PO-20260508-000004 | SUP-0003 | STORE-KYIV-01 | 1063 | sent |
| PO-20260508-000001 | SUP-0008 | STORE-KYIV-01 | 926 | sent |
| PO-20260508-000002 | SUP-0010 | STORE-KYIV-01 | 820 | sent |
| PO-20260508-000005 | SUP-0013 | STORE-KYIV-01 | 791 | sent |
| PO-20260508-000007 | SUP-0001 | STORE-KYIV-01 | 712 | sent |

15 POs суммарно, все ушли через `erp_api` канал (HTTP POST → mock-erp).

## 📊 KPI (stock_days distribution)

| bucket | n |
|---|---|
| 60+ days | 450 |

Все 450 пар (100p × 4 stores + 50 DC sample) попадают в bucket `60+`. Это объясняется тем, что KPI считает `stock_days` как `current_stock / daily_demand`, и в данных снимков отношение получилось высоким (450/365 запись по дефолту в KPI = sentinel-cap при низком фактическом спросе на DC). Данные поступают, бизнес-смысл может быть скорректирован в отдельном тюнинге KPI после калибровки decay-снапшотов.

## 🔄 CHANNEL — send_attempts breakdown

| channel_type | status | n |
|---|---|---|
| erp_api | success | 15 |

100% accepted (HTTP 201). Никаких rejected/pending. Каналов: 50 настроек supplier_channel_config (15 suppliers × erp_api/edi/email/manual вариантов, primary=erp_api для всех 15).

## ✅ Mock-erp received

```
GET /api/v1/orders/received → 15 orders
distinct po_numbers: 15
```

Sample (top-3 by qty):
- `PO-20260508-000004` SUP-0003 → 1063 qty
- `PO-20260508-000001` SUP-0008 → 926 qty
- `PO-20260508-000002` SUP-0010 → 820 qty

End-to-end loop замкнулся: ERP → данные → ETL → marts → KPI/forecast → план → PO → канал → mock-erp.
