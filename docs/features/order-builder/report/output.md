# Pipeline Report: order-builder (Модуль 6)
**Дата:** 2026-05-07
**Tier:** M (compact)

## Что сделано

Реализован order-builder: cron 06:00 Europe/Kyiv → подбирает approved replenishment_plans → конвертирует в purchase orders + lines → пишет в `orders.*`. 5 endpoints + admin actions (build, cancel, regenerate).

## Endpoints

| Method | Path | Auth | Описание |
|---|---|---|---|
| GET | /v1/orders/purchase-orders | admin/etl/it-read | List + filter + cursor |
| GET | /v1/orders/purchase-orders/:id | admin/etl/it-read | Single PO + lines + history |
| POST | /v1/orders/purchase-orders/build | admin-cli | Ondemand build из approved plans |
| POST | /v1/orders/purchase-orders/:id/cancel | admin-cli | Cancel before send |
| POST | /v1/orders/purchase-orders/:id/regenerate | admin-cli | Cancel + create new PO from same plan |

## Status workflow
`draft → ready_to_send (default at build) → sent → confirmed_by_erp → received | cancelled`

## Метрики прогона
- 9 git коммитов (1 design + 8 фаз)
- 1 миграция (4001_orders_schema): orders schema + sequence + 3 таблицы + partitioning по created_at + ALTER plan status enum (+ converted)
- 8 sentinel + supportMessage коды (OB-001..008)
- 5 Prometheus метрик (order_builder_*)
- ~30 unit + 1 integration test (отложен на validation stage)

## Quality gates
- `go build ./...` — OK
- `go test -race ./internal/features/orders/...` — OK
- `golangci-lint run` — 0 issues

## Известные ограничения
- Plan→PO matching 1:1 (один approved plan → один PO). Multi-plan grouping per supplier per day — next iter.
- Single currency (UAH default из supplier.currency_code). Multi-currency conversion — next iter.
- Без tax/discount calculation.
- Без digital signatures.
- Pricing waterfall: products.unit_price → supplier.default_unit_price → NULL (с warning лог).

## Артефакты
- design.md, spec-interview, code-plan + status, 8 phase-файлов
- internal/features/orders/* — реализация
- pkg/errorspkg/errors_orders.go — 8 sentinel
- orders.* schema (purchase_orders, po_lines, po_status_history) + sequence po_number_seq
