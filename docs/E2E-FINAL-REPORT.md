# E2E Pipeline Test — Final Report
**Дата:** 2026-05-07
**Status:** ✅ **8/8 stages PASSED**

## Что сделано

Построен mock-сервис E-Zoo ERP (Python FastAPI + Faker + SQLite) и интегрирован с нашими 7 микросервисами через docker-compose. Полный E2E pipeline проходит зелёным:

```
mock-erp → source-adapter (HTTP pull) → public.* tables
       ↓
       etl → marts.* (4 marts)
       ↓
       kpi → kpi.kpi_snapshots
       ↓
       forecast → forecast.forecasts + replenishment_plans (draft)
       ↓
       admin approve → status=approved
       ↓
       order-builder → orders.purchase_orders (ready_to_send)
       ↓
       channel-router → mock-erp (HTTP push) → status=sent + accepted
```

## E2E Результаты (small-scale: 10 products / 2 locations / 7 days / 3 suppliers)

| Stage | Service | Action | Time | Result |
|---|---|---|---|---|
| 1 | source-adapter | Pull from mock ERP | 6s | ✅ products=10, receipt_line=380, location_stock_snapshot=20, supplier=3 |
| 2 | etl | Build marts | 5s | ✅ demand_history=70, calc_input=20, master_current=10, kpi_daily=28 |
| 3 | kpi | Compute OSA/OTIF/StockDays | 2s | ✅ osa=10, stock_days=20 |
| 4 | forecast | Forecast + plans | 0s | ✅ forecasts=600 (10 products × 60 days), draft_plans=3 |
| 5 | manual approve | PUT approve plans | 0s | ✅ approved=3 |
| 6 | order-builder | Build POs | 3s | ✅ ready_to_send=3 |
| 7 | channel-router | Send to mock ERP | 2s | ✅ sent=3, send_attempts.accepted=3 |
| 8 | mock-erp verify | Received POs | 0s | ✅ received_orders=3 |

**Total: 18 секунд на full pipeline** (small scale).

## Как запустить

```bash
cp .env.example .env
SEED_PRODUCTS=10 SEED_LOCATIONS=2 SEED_DAYS=7 SEED_SUPPLIERS=3 make e2e-up
bash tests/e2e/run.sh --skip-up
```

Для полного scale (1000/30/365/50):
```bash
make e2e-up    # ~10-15 минут на seed mock-erp с 1000 products / 365 days
bash tests/e2e/run.sh --skip-up
```

## Архитектура mock-erp

Python FastAPI + Faker + SQLite (persistent volume).

**Endpoints:**
- READ: 16 GET `/api/v1/{entity}` для всех сущностей source-adapter (master + facts)
- WRITE: POST `/api/v1/orders` принимает PO от channel-router
- VERIFY: GET `/api/v1/orders/received` для E2E-проверки
- HEALTH: GET `/healthz`

Auth: `X-API-Key` (env `MOCK_ERP_API_KEY`). Также поддерживает `Authorization: Bearer` для совместимости.

## Найденные и исправленные баги (10 commits подряд)

| # | Commit | Проблема | Фикс |
|---|---|---|---|
| 1 | 92acb91 | Loader не UPSERT-ил category/supplier/location → FK violation на products | Добавлены реальные UPSERT методы в master.go |
| 2 | 6d4c351 | ETL 500 SA-INT-001 — schema permissions для e_zoo_app + JWT issuer claim не задан | ALTER DEFAULT PRIVILEGES + GRANT USAGE; Issuer в HS256Config |
| 3 | b732439 | snapshot endpoint field names mismatch (snapshot_id vs current_load_id) | Унифицировано на current_load_id + committed_at |
| 4 | ae8651a | URL prefix mismatch (/v1/entities/{e} vs /v1/{e}) + facts требуют event_date_from/to | Прибран /entities/, добавлен date range, 404/501 → graceful skip |
| 5 | b2ad4f0 | 12 staging columns vs DTO field names mismatch | Renamed columns под DTO json tags + relaxed NULL constraints |
| 6 | dc9be49 | Entity name constants (stock_on_hand vs location_stock_snapshot, product vs products) | Унифицировано с source-adapter routes |
| 7 | 29cb944 | Source-adapter не загружал 3 entity (order_rule, supply_spec, location_stock_snapshot) | Реализованы UPSERT/insert + handlers + интегрированы в runPipeline |
| 8 | (включён в #7) | mock-erp не принимал Bearer auth (только X-API-Key) | auth.py поддерживает оба |
| 9 | (включён в #7) | forecast PresetRunID — POST возвращал uuid.New(), engine сохранял свой → 404 | engine.go: PresetRunID при triggered run |
| 10 | (включён в #7) | receipt_line.LineKind пуст → mart_demand_history.qty_sold=0 | LineKind: "sale"/"return" по qty sign |

Плюс предшествующие коммиты:
- 9c89f33 — docker-compose smoke fixes (migration 4001 partition-key, init.sh shell, etl healthcheck)
- 6f9e4e7 — supplier_channel_config seed для 50 поставщиков
- 0031e02 — E2E test runner + jwtgen helper + docs

## Как это помогает проекту

1. **Контракт ERP проверен на практике** — REST + JSON, X-API-Key auth, cursor pagination + ETag. Можно показать клиенту до получения ответа на Q-001..Q-003.
2. **Все 7 микросервисов работают вместе** — больше не изолированные unit/integration тесты, а реальный flow.
3. **Stress-test для partitioning** — 365 дней истории требуют 12+ monthly partitions, миграция 0003 их создаёт.
4. **Demo-сценарий готов** — `make e2e-up && bash tests/e2e/run.sh --skip-up` показывает 8/8 за минуты.
5. **Найдено 10 контрактных багов между модулями** — которые иначе всплыли бы в production.

## Файлы

**Mock ERP:** `mock-erp/` (Python service, ~1500 LOC)
**HTTPSourceReader:** `internal/features/data_export/loader/http_source_reader.go`
**E2E runner:** `tests/e2e/run.sh` + `tests/e2e/cmd/jwtgen/main.go`
**Docs:** `docs/E2E-TEST.md` + `docs/MICROSERVICES.md`
**Compose:** `docker-compose.yml` (9 сервисов: postgres + migrate + mock-erp + 7 микросервисов)
**Migrations:** 9 миграций applied (`0001..5001`)

## Известные ограничения (документированные)

- **Daily-delta эмуляция** в mock-erp архитектурно готова (current_simulated_date в SQLite), но runner использует static seed
- **OAuth2/mTLS** для ERP — interface заложен, реализация только api_key
- **Multi-tenant** — один клиент в mock
- **Bi-temporal recompute** в ETL — Q-012 отложен
- **CI/GitHub Actions** для E2E — Q-012 отложен
- 14 entity master-handlers source-adapter возвращают 501 (не реализованы для E2E через ETL — но 3 критичных реализованы в commit 29cb944)
