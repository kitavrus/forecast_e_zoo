# E2E Pipeline Test

End-to-end проверка всего e_zoo репленишмент-пайплайна: mock ERP → 7 микросервисов → mock ERP. Запускается одной командой `bash tests/e2e/run.sh`. Прогоняет 8 стадий, на каждой делает HTTP-вызов админ-эндпоинта и проверяет, что соответствующая таблица в Postgres получила ожидаемые данные.

## Что проверяется

Полный цикл репленишмента:

1. **source-adapter** забирает справочники, движения и снапшоты остатков из mock-ERP, складывает в Postgres (`public.products`, `public.receipt_line`, `public.location_stock_snapshot`, `public.supplier`).
2. **etl** строит data-marts (`marts.mart_demand_history`, `marts.mart_calculation_input`).
3. **kpi** считает OSA / OTIF / stock_days по data-marts (`kpi.kpi_snapshots`).
4. **forecast** строит прогноз спроса и draft-планы пополнения (`forecast.forecasts`, `forecast.replenishment_plans`).
5. Админ **аппрувит draft-планы** через API.
6. **order-builder** превращает approved-планы в purchase orders в статусе `ready_to_send` (`orders.purchase_orders`).
7. **channel-router** забирает `ready_to_send`-PO, отправляет их в mock-ERP по конфигу `channels.supplier_channel_config`, переводит PO в `sent`, фиксирует попытки в `channels.send_attempts` со статусом `accepted`.
8. **mock-ERP** через `GET /api/v1/orders/received` подтверждает, что заказ дошёл.

Каждая стадия проверяет HTTP-ответ + count-проверку в Postgres. Любая ошибка прерывает прогон с `[FAIL]`.

## Prerequisites

- **Docker** + `docker compose` v2 (для compose-проекта `e_zoo`).
- **jq**, **curl** (parsing JSON, HTTP-вызовы).
- **Go 1.26+** (только для разовой генерации JWT через `tests/e2e/cmd/jwtgen`; альтернативно — `python3 -c "import jwt; ..."`).
- Свободные порты `5432`, `8080-8087`, `8090`, `9090`.
- `.env` файл — скопируй `.env.example` или экспортируй переменные `JWT_SECRET`, `MOCK_ERP_API_KEY`, `POSTGRES_USER`, `POSTGRES_DB`, опц. `SEED_PRODUCTS/SEED_LOCATIONS/SEED_DAYS/SEED_SUPPLIERS`.

## Quick start

```bash
# 1. Подготовить .env
cp .env.example .env

# 2. Поднять весь стек (build + start) и прогнать E2E (small-scale seed)
make e2e-up                                  # build + up + seed_channel_configs
bash tests/e2e/run.sh --skip-up              # прогон без повторного up

# Или одной командой (включая up):
bash tests/e2e/run.sh --scale-small --cleanup
```

## Параметры запуска

| Флаг | Назначение |
|---|---|
| `--skip-up` | не делать `make e2e-up` (compose уже запущен) |
| `--cleanup` | после прогона `docker compose down -v` |
| `--scale-small` | мелкий seed: `SEED_PRODUCTS=10 SEED_LOCATIONS=2 SEED_DAYS=7 SEED_SUPPLIERS=3`. По умолчанию full: 1000/30/365/50 |
| `-h`, `--help` | помощь |

ENV vars (читаются из окружения, при необходимости — переопредели в shell перед запуском):

- `JWT_SECRET` — HS256 ключ (default `dev-secret-change-in-prod`).
- `POSTGRES_USER` / `POSTGRES_DB` — для psql exec (default `adapter` / `source_adapter`).
- `MOCK_ERP_API_KEY` — X-API-Key для верификации `/api/v1/orders/received` (default `test-api-key`).

## Архитектура runner-а

- Генерация JWT — `go run ./tests/e2e/cmd/jwtgen -role admin-cli` ⇒ `iss=admin-cli` claim, проверяется `internal/middleware.RequireAdmin()`.
- Запросы к админам — `curl -fsS -H "Authorization: Bearer $JWT"`.
- Проверка counts — `docker exec ezoo_pg psql -U <user> -d <db> -t -A -c "SELECT count(*) FROM ..."`.
- Polling — каждая async-стадия (loads / etl-runs / forecast-runs) опрашивается до `committed`, либо до тайм-аута 600s; для kpi/orders/channels — поллинг по DB (count != 0 + стабильность за 2s).

## Per-stage breakdown

| # | Сервис | Endpoint | Async? | Что проверяется |
|---|---|---|---|---|
| 1 | source-adapter | `POST :8080/admin/loads` → polling `GET .../admin/loads/{id}` до `committed` | да | `products`, `receipt_line`, `location_stock_snapshot`, `suppliers` count > 0 |
| 2 | etl | `POST :8081/api/v1/admin/etl-runs` → polling `GET .../etl-runs/{id}` до `committed` | да | `marts.mart_demand_history`, `marts.mart_calculation_input` count > 0 |
| 3 | kpi | `POST :8083/v1/kpi/snapshots/refresh` (sync 202 + background trigger) | sync trigger / async write | `kpi.kpi_snapshots` появились rows для `osa`/`otif`/`stock_days` |
| 4 | forecast | `POST :8084/v1/forecast/runs/refresh` → polling `GET .../runs/{id}` до `committed` | да | `forecast.forecasts`, draft `forecast.replenishment_plans` count > 0 |
| 5 | (runner) | `GET :8084/v1/replenishment/plans?status=draft` + per-id `POST .../approve` | sync | DB: `replenishment_plans.status='approved'` count > 0 |
| 6 | order-builder | `POST :8086/v1/orders/purchase-orders/build` (async через trigger) | async | `orders.purchase_orders.status='ready_to_send'` стабилизировался |
| 7 | channel-router | `POST :8087/v1/channels/send` (async) | async | `orders.purchase_orders.status='sent'` + `channels.send_attempts.status='accepted'` |
| 8 | mock-ERP verify | `GET :8090/api/v1/orders/received` (X-API-Key) | sync | `len(items) >= sent_count` |

## Expected output (small-scale)

```text
[..] Step 0: waiting for all services healthy (timeout 5 min)...
[OK] Step 0: all 9 services healthy
[OK] Step 0.5: JWT generated (issuer=admin-cli)
[..] Stage 1/8: source-adapter — POST /admin/loads
[OK] Stage 1: load=...-... committed in 12s — products=10 receipt_line=140 location_stock_snapshot=20 suppliers=3
[..] Stage 2/8: etl — POST /api/v1/admin/etl-runs
[OK] Stage 2: etl-run=... committed in 18s — mart_demand_history=70 mart_calculation_input=20
[..] Stage 3/8: kpi — POST /v1/kpi/snapshots/refresh
[OK] Stage 3: kpi run=... in 8s — osa=20 otif=20 stock_days=20
[..] Stage 4/8: forecast — POST /v1/forecast/runs/refresh
[OK] Stage 4: forecast run=... committed in 24s — forecasts=600 draft_plans=6
[..] Stage 5/8: approve all draft replenishment plans
[OK] Stage 5: approved=6 failed=0 in 4s (DB approved=6)
[..] Stage 6/8: order-builder — POST /v1/orders/purchase-orders/build
[OK] Stage 6: purchase_orders total=6 ready_to_send=6 in 11s
[..] Stage 7/8: channel-router — POST /v1/channels/send
[OK] Stage 7: PO sent=6 send_attempts total=6 accepted=6 failed=0 in 9s
[..] Stage 8/8: verify mock-erp received purchase orders
[OK] Stage 8: mock-erp received_orders=6 distinct_po_numbers=6 in 1s

════════════════════════════════════════════════════════════════════════
  E2E Pipeline Test: 8/8 stages PASSED in 87s
════════════════════════════════════════════════════════════════════════
  Stage 1 source-adapter        12s   products=10 receipt_line=140
  Stage 2 etl                   18s   demand_history=70 calc_input=20
  Stage 3 kpi                    8s   osa=20 otif=20 stock_days=20
  Stage 4 forecast              24s   forecasts=600 draft_plans=6
  Stage 5 approve plans          4s   approved=6
  Stage 6 order-builder         11s   PO ready_to_send=6
  Stage 7 channel-router         9s   PO sent=6 attempts accepted=6
  Stage 8 mock-erp verify        1s   received=6 distinct=6
════════════════════════════════════════════════════════════════════════
```

Полный seed (1000/30/365/50) обычно укладывается в 8-15 минут.

## Troubleshooting

| Симптом | Причина / лечение |
|---|---|
| `services did not become healthy` | mock-erp seed > 60s. Увеличить `start_period` в `docker-compose.yml.mock-erp.healthcheck` или взять `--scale-small`. |
| `migrations failed` | проверь `infra/pg/init/01_init.sh` (создание service-роли `e_zoo_app`); удали `var/postgres/` перед `make e2e-up` (volume не перечитает init). |
| Stage 1 zero products | source-adapter не видит mock-erp. Убедись что `ERP_BASE_URL=http://mock-erp:8090` в env. |
| Stage 1 load `failed` с `products_category_id_fkey` | **Pre-existing source-adapter bug** — loader пытается вставить products до того, как загрузит category. Проверь `docker compose logs source-adapter \| grep loader.failed`. Чинится отдельно (приоритет порядка загрузки в `internal/features/data_export/service`). |
| Stage 7 send_attempts.failed > 0 | `channels.supplier_channel_config` не засеян или `endpoint_url` неверный. Перезапусти `make seed-channel-configs`. |
| `401 Unauthorized` от любого админ-эндпоинта | `JWT_SECRET` runner-а ≠ `JWT_SECRET` сервиса (см. `.env`). Они должны совпадать (HS256). |
| `403 Forbidden` от админ-эндпоинта | роль в JWT (`iss`) != `admin-cli`. Проверь `tests/e2e/cmd/jwtgen` параметры. |
| Stage 6 timeout — `ready_to_send=0` | order-builder упал на partial-фазе. Логи: `docker compose logs ezoo_order_builder --tail=200`. |
| Stage 8 received < sent | retry-окно channels-router'а ещё не закрылось. Запусти `curl :8090/api/v1/orders/received` через 30s повторно. |

## Известные ограничения

- **Single-day delta NIY** — runner делает full ingest каждый раз; incremental loads не покрыты.
- **API-key auth** — все channel-configs используют api_key (см. `tests/e2e/seed_channel_configs.sql`); EDI / mTLS варианты не E2E-ятся.
- **No multi-tenant** — single Postgres, single mock-ERP, без RLS.
- **JWT_SECRET shared** — все сервисы используют один HS256 ключ. Production должен использовать RS256 + KMS.

## См. также

- [docs/MICROSERVICES.md](MICROSERVICES.md) — обзор всех 7 микросервисов.
- `tests/e2e/seed_channel_configs.sql` — seed `supplier_channel_config` (50 поставщиков, api_key к mock-ERP).
- `mock-erp/app/routes/orders.py` — реализация `POST /api/v1/orders` и `GET /api/v1/orders/received`.
