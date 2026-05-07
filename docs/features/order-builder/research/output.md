# Research: order-builder (Модуль 6)
**Дата:** 2026-05-07
**Mode:** compact

## Контекст
Из draft-plan: «Блок подготовки заказа. Этот модуль формирует заказы, подготавливает данные, тоже в своих это все делается в таблицах. И дальше подготовленные данные передаются в следующий модуль.»

## Flow
```
forecast.replenishment_plans (status='approved') → OrderBuilder → orders.purchase_orders (status='ready_to_send') → Модуль 7 (channel-routing)
```

## Architecture
- Обработчик: подписывается на approved replenishment_plans (через cron polling каждые 5 мин ИЛИ on-approval webhook от Модуля 5)
- Convert plan + lines → purchase order document (PO)
- Add: PO number generation, supplier contact info, billing address, delivery instructions, payment terms
- Validate: completeness check (all required fields), business rules (negative qty, MOQ check repeated)
- Output: orders.purchase_orders + orders.po_lines, status='ready_to_send'
- Endpoints: list/get PO, admin actions (cancel, regenerate)

## Tier: M
- Endpoints: 5
- Новых сущностей: 2 (purchase_orders, po_lines)
- Миграций: 1 (4001_orders_schema)
- Cron: 06:00 Europe/Kyiv (после forecast 05:00)
- Diff: ~1500 LOC

## Schema
```
orders.purchase_orders(po_id, po_number unique, supplier_id, location_id, plan_id refs forecast.replenishment_plans, status, total_qty, total_amount, currency, delivery_date, created_at, sent_at, sent_to_channel)
orders.po_lines(po_line_id, po_id, product_id, qty, unit_price, total_amount, ...)
orders.po_status_history(history_id, po_id, from_status, to_status, changed_at, changed_by, reason)
```

## Status workflow
```
draft → ready_to_send → sent → confirmed_by_erp → received | cancelled
```

## Endpoints
- GET /v1/orders/purchase-orders (list, filter by status/supplier/date)
- GET /v1/orders/purchase-orders/:id (с lines + status history)
- POST /v1/orders/purchase-orders/build (admin-cli, ondemand recompute из всех approved plans)
- POST /v1/orders/purchase-orders/:id/cancel (admin-cli) — cancel before send
- POST /v1/orders/purchase-orders/:id/regenerate (admin-cli) — recreate PO from same plan

## Q-NNN (defaulted)
- Q-001: Trigger → cron 06:00 + ondemand admin endpoint
- Q-002: PO number format → PO-{YYYY}{MM}{DD}-{seq6} (например PO-20260507-000001), seq per supplier
- Q-003: Currency → из supplier configuration (UAH default)
- Q-004: Delivery date → calculation: order_date + supplier.lead_time_days
- Q-005: Plan→PO matching → 1:1 (один approved plan → один PO). Multiple plans от same supplier per day → отдельные PO (можно сгруппировать в next iter).
- Q-006: Pricing → из products.unit_price (если есть) ИЛИ supplier.default_unit_price ИЛИ NULL (запросить у supplier).
- Q-007: PO regeneration → создаёт новый PO с новым po_number, старый помечает 'cancelled'
- Q-008: Подпись/digital signature → не в MVP (Q-NNN отложено)

## Что уже есть (после Модулей 1-5)
- pgxpool, JWT, role middleware, scheduler pattern, advisory lock pattern
- forecast.replenishment_plans с status workflow
- mart_master_current (supplier info)
- pkg/errorspkg, mappers паттерн

## Non-goals MVP
- Multi-currency conversion
- Tax calculation
- Discount rules (volume, promo)
- Approval workflow внутри PO (single approve)
- Digital signatures
- Multi-language PO documents
- PDF generation (только JSON структура)
