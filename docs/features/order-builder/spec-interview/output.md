# Spec Interview: order-builder (Модуль 6)

**Дата:** 2026-05-07
**Mode:** compact (defaulted из research/output.md)
**Tier:** M

```yaml
# Triage
tier: M
touches: {db: true, fe: false, infra: false, external: false}
risk: reversible
novelty: standard-crud
decisions: [Q-002, Q-004, Q-005, Q-006, Q-007]
```

## Проблема и цель
Модуль 5 (forecast) формирует `replenishment_plans` с `status='approved'` после ручного аппрува. Этих данных недостаточно для отправки в каналы (Модуль 7): нужен формализованный документ заказа (Purchase Order = PO) с уникальным номером, привязкой к поставщику, датами, ценами и статусной моделью. Модуль 6 — конвертер approved-планов в покупательные заказы, источник правды для дальнейшей рассылки.

## Сценарии
1. **Cron 06:00 Europe/Kyiv** — scheduler tick: pg_advisory_lock('order-builder-run') → SELECT FOR UPDATE approved plans → build PO → mark plan='converted' → INSERT purchase_orders + lines → status='ready_to_send'.
2. **Admin on-demand build** — `POST /v1/orders/purchase-orders/build` запускает тот же pipeline вне расписания.
3. **List/Get** — IT-Read и Admin читают PO с фильтрами (status, supplier, date, plan_id).
4. **Cancel** — `POST /:id/cancel` переводит PO в `cancelled` (только из `draft`/`ready_to_send`/`sent`).
5. **Regenerate** — `POST /:id/regenerate` создаёт новый PO с новым номером из того же plan, старый помечает `cancelled`.

## Defaulted answers (Q-NNN)
| Q | Решение |
|---|---|
| Q-001 | Trigger: cron 06:00 Europe/Kyiv + on-demand admin endpoint. |
| Q-002 | PO number: `PO-{YYYY}{MM}{DD}-{seq6}` — глобальная PG sequence `orders.po_number_seq`, дата из `created_at::date`. (Per-supplier-per-day усложняет race conditions без бизнес-выгоды на MVP.) |
| Q-003 | Currency: подтягиваем из `marts.mart_master_current` (entity_type='supplier', `currency_code`); fallback `'UAH'`. |
| Q-004 | Delivery date: `created_at::date + supplier.lead_time_days` (из marts; fallback 7 дней). |
| Q-005 | Plan→PO: 1:1 — `plan_id UNIQUE` на `purchase_orders`. Несколько approved-планов одного supplier за день → отдельные PO (next iter — группировка). |
| Q-006 | Pricing: `products.unit_price` → `supplier.default_unit_price` → NULL + предупреждение в `notes`. |
| Q-007 | Regenerate: новая `po_id`, новый `po_number`, старый PO → `cancelled` reason='regenerated'; plan остаётся `converted`, но снова появляется ассоциация с новым PO (нарушение plan_id UNIQUE решается partial unique index по `WHERE status NOT IN ('cancelled')`). |
| Q-008 | Digital signatures: не в MVP. |

## Edge cases
- **Plan уже converted** → `409 ErrPlanAlreadyConverted` (внешний build endpoint).
- **PO в финальном статусе при cancel** → `409 ErrPONotCancellable`.
- **PO sent при regenerate** → `409 ErrPOAlreadySent` (regenerate только до `sent`).
- **Marts row для supplier отсутствует** → currency='UAH', lead_time=7, log warning.
- **Approved plan с lines_count=0** (теоретический edge) → пропустить, log warning.
- **Concurrent scheduler + manual build** → защита через `pg_advisory_lock(0x4F4244524C4456)` ("OBDRLDV").

## Открытые вопросы для Design (нет блокеров)
Все Q-NNN дефолтнуты. Design агент фиксирует ADR.

## Компромиссы
- Single-row partial-unique index на `plan_id` обеспечивает 1:1 при наличии cancellation.
- Числовая последовательность PO глобальная, не per-supplier — упрощает race conditions.
- Pricing waterfall — NULL допустимо в MVP (Модуль 7 решает что делать).
