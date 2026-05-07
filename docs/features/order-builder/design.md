# Design: order-builder (Модуль 6)

**Дата:** 2026-05-07
**Tier:** M
**Mode:** compact

```yaml
# Triage
tier: M
touches: {db: true, fe: false, infra: false, external: false}
risk: reversible
novelty: standard-crud
```

## 1. Обзор и flow

Модуль 6 — конвертер approved replenishment_plans в формализованные purchase_orders.

```
forecast.replenishment_plans (status='approved')
    │
    ▼  (cron 06:00 Europe/Kyiv ИЛИ POST /build)
[OrderBuilder]
    │  1. SELECT approved plans FOR UPDATE
    │  2. Resolve supplier (lead_time, currency) из marts.mart_master_current
    │  3. Resolve product pricing (products.unit_price | supplier default | NULL)
    │  4. Generate PO number (orders.po_number_seq) → PO-YYYYMMDD-NNNNNN
    │  5. INSERT orders.purchase_orders (status='ready_to_send', plan_id, ...)
    │  6. INSERT orders.po_lines (1 per calculation_line)
    │  7. INSERT orders.po_status_history (NULL → ready_to_send)
    │  8. UPDATE plan.status='converted'
    │
    ▼
orders.purchase_orders (status='ready_to_send')
    │
    ▼  Модуль 7 (channel-routing)
```

Status workflow:
```
[init] ─┬─→ draft ──→ ready_to_send ──→ sent ──→ confirmed_by_erp ──→ received
        │                  │              │
        └──────────────────┴──────────────┴──→ cancelled (terminal до received)
```

Builder заводит PO сразу в `ready_to_send` (`draft` зарезервирован для ручных правок post-MVP).

## 2. Endpoints (5)

| Endpoint | Role | DTO |
|---|---|---|
| `GET /v1/orders/purchase-orders` | IT-Read \| Admin \| X-Flow-ETL | `ListPurchaseOrdersResponse{items[], next_cursor}` |
| `GET /v1/orders/purchase-orders/:id` | IT-Read \| Admin \| X-Flow-ETL | `PurchaseOrderWithLinesResponse{order, lines, history}` |
| `POST /v1/orders/purchase-orders/build` | Admin | `BuildResponse{run_id, started, plans_processed, pos_created}` |
| `POST /v1/orders/purchase-orders/:id/cancel` | Admin | body `{reason}` → `PurchaseOrderResponse{order}` |
| `POST /v1/orders/purchase-orders/:id/regenerate` | Admin | body `{reason}` → `RegenerateResponse{old_po_id, new_po_id, new_po_number}` |

Sequence (cron build):
```mermaid
sequenceDiagram
    autonumber
    participant Cron
    participant Sched as Scheduler
    participant DB as Postgres
    participant Svc as POBuilder Service
    participant Build as POBuilder
    participant Num as PONumberGenerator
    Cron->>Sched: tick 06:00
    Sched->>DB: pg_try_advisory_lock(OBDRLDV)
    DB-->>Sched: acquired=true
    Sched->>Svc: BuildAll(ctx)
    Svc->>DB: BEGIN; SELECT approved plans FOR UPDATE
    DB-->>Svc: [plan1, plan2, ...]
    loop per plan
        Svc->>DB: read plan_lines, supplier marts row, products
        Svc->>Num: NextNumber(date)
        Num->>DB: nextval('orders.po_number_seq')
        DB-->>Num: 42
        Num-->>Svc: PO-20260507-000042
        Svc->>Build: Build(plan, lines, supplier, products)
        Build-->>Svc: PO + lines (in-memory)
        Svc->>DB: INSERT purchase_orders, po_lines, po_status_history
        Svc->>DB: UPDATE plan.status='converted'
    end
    Svc->>DB: COMMIT
    Sched->>DB: pg_advisory_unlock
```

## 3. SQL queries + миграция

### Миграция `4001_orders_schema.up.sql`
```sql
CREATE SCHEMA IF NOT EXISTS orders;

CREATE SEQUENCE IF NOT EXISTS orders.po_number_seq AS BIGINT START 1;

CREATE TABLE IF NOT EXISTS orders.purchase_orders (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    po_number       TEXT NOT NULL,
    plan_id         UUID NOT NULL,           -- forecast.replenishment_plans.id
    supplier_id     TEXT NOT NULL,
    location_id     TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN
        ('draft','ready_to_send','sent','confirmed_by_erp','received','cancelled')),
    total_qty       NUMERIC(18,4) NOT NULL,
    total_amount    NUMERIC(18,4),           -- nullable если pricing partial
    currency        TEXT NOT NULL DEFAULT 'UAH',
    delivery_date   DATE,
    notes           TEXT,
    sent_at         TIMESTAMPTZ,
    sent_to_channel TEXT,
    cancel_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Monthly partitions (rolling, 365d retention).
CREATE TABLE IF NOT EXISTS orders.purchase_orders_2026_05 PARTITION OF orders.purchase_orders
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS orders.purchase_orders_2026_06 PARTITION OF orders.purchase_orders
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS orders.purchase_orders_2026_07 PARTITION OF orders.purchase_orders
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_orders_po_number
    ON orders.purchase_orders(po_number);

-- 1:1 plan→PO для не-cancelled.
CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_orders_plan_active
    ON orders.purchase_orders(plan_id) WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_purchase_orders_status
    ON orders.purchase_orders(status);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_supplier_date
    ON orders.purchase_orders(supplier_id, created_at DESC);

CREATE TABLE IF NOT EXISTS orders.po_lines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id           UUID NOT NULL,
    product_id      TEXT NOT NULL,
    qty             NUMERIC(18,4) NOT NULL CHECK (qty > 0),
    unit_price      NUMERIC(18,4),           -- nullable
    line_amount     NUMERIC(18,4),           -- qty * unit_price если есть
    pricing_source  TEXT,                    -- 'product'|'supplier_default'|'missing'
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_po_lines_po_id ON orders.po_lines(po_id);

CREATE TABLE IF NOT EXISTS orders.po_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id       UUID NOT NULL,
    from_status TEXT,
    to_status   TEXT NOT NULL,
    reason      TEXT,
    changed_by  TEXT,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_po_status_history_po_id
    ON orders.po_status_history(po_id);

-- Расширяем replenishment_plans статусом converted (через CHECK обход).
ALTER TABLE forecast.replenishment_plans
    DROP CONSTRAINT IF EXISTS replenishment_plans_status_check;
ALTER TABLE forecast.replenishment_plans
    ADD CONSTRAINT replenishment_plans_status_check
    CHECK (status IN ('draft','approved','cancelled','converted'));
```

### Queries (sqls/queries/)
- `select_approved_plans_for_update.sql` — `SELECT ... FOR UPDATE SKIP LOCKED LIMIT $1`
- `select_plan_lines_for_build.sql` — JOIN `forecast.calculation_lines`
- `select_supplier_master.sql` — `marts.mart_master_current` где entity_type='supplier'
- `select_product_master.sql` — `marts.mart_master_current` где entity_type='product' для price/currency
- `next_po_number.sql` — `SELECT nextval('orders.po_number_seq')`
- `insert_purchase_order.sql`, `insert_po_line.sql`, `insert_po_status_history.sql`
- `update_plan_to_converted.sql`
- `select_po_by_id.sql`, `select_po_lines.sql`, `select_po_history.sql`, `select_purchase_orders.sql`
- `update_po_cancel.sql` — переход в cancelled с проверкой статуса.
- `update_po_status.sql` — generic state transition (zero-allocs option for future).

## 4. Errors

```go
// pkg/errorspkg/errors_orders.go
var (
    ErrPurchaseOrderNotFound = &Error{
        Code: "purchase_order_not_found", HTTP: 404,
        SupportMessage: SupportPurchaseOrderNotFound, Message: "purchase order not found",
    }
    ErrPlanAlreadyConverted = &Error{
        Code: "plan_already_converted", HTTP: 409,
        SupportMessage: SupportPlanAlreadyConverted, Message: "plan already converted to PO",
    }
    ErrPlanNotApproved = &Error{
        Code: "plan_not_approved", HTTP: 409,
        SupportMessage: SupportPlanNotApproved, Message: "plan must be approved to build PO",
    }
    ErrPONotCancellable = &Error{
        Code: "po_not_cancellable", HTTP: 409,
        SupportMessage: SupportPONotCancellable, Message: "PO cannot be cancelled in current status",
    }
    ErrPOAlreadySent = &Error{
        Code: "po_already_sent", HTTP: 409,
        SupportMessage: SupportPOAlreadySent, Message: "PO already sent and cannot be regenerated",
    }
    ErrInvalidPOStatus = &Error{
        Code: "invalid_po_status", HTTP: 400,
        SupportMessage: SupportInvalidPOStatus, Message: "invalid status filter",
    }
    ErrOrderBuilderUnavailable = &Error{
        Code: "order_builder_unavailable", HTTP: 503,
        SupportMessage: SupportOrderBuilderUnavailable, Message: "order builder is not configured",
    }
    ErrOrderBuilderInProgress = &Error{
        Code: "order_builder_in_progress", HTTP: 409,
        SupportMessage: SupportOrderBuilderInProgress, Message: "another build run is in progress",
    }
)
```

Support-коды: `OB-001..OB-008`.

## 5. Tests

### Unit
- `numbering_test.go` — sequence parsing, format `PO-YYYYMMDD-NNNNNN`, padding.
- `builder_test.go` — Plan + lines + supplier + products → PO + lines, edge cases (NULL price, missing supplier marts row, lines_count=0).
- `validators_test.go` — status enum, cancel reason length, regenerate reason length.
- `service_test.go` — lifecycle: cancel/regenerate state transitions, no scheduler → 503.

### Integration (`postgres:18-alpine` через dockertest)
- `repository_integration_test.go` — INSERT/SELECT FOR UPDATE/UNIQUE constraint, partial unique index on plan_id, partition pruning.
- `numbering_integration_test.go` — concurrent nextval не даёт коллизий.

## 6. ADR

- **ADR-001 (Q-002): Глобальная sequence для po_number** — простота и отсутствие race vs. per-supplier-per-day. Trade-off: номера не показывают порядок per-supplier; принято.
- **ADR-002 (Q-005): Partial unique index на plan_id WHERE status<>'cancelled'** — поддерживает 1:1 с возможностью regenerate. Альтернатива (history table) сложнее.
- **ADR-003 (Q-006): Pricing waterfall с pricing_source меткой** — прозрачность для Модуля 7. Альтернатива (фейлить если NULL) преждевременна.
- **ADR-004 (Q-007): Regenerate = cancel old + create new** — audit trail сохраняется. Альтернатива (UPDATE in place) ломает history.
- **ADR-005: Status workflow без `draft` для builder** — builder создаёт сразу `ready_to_send`. `draft` зарезервирован под ручные правки post-MVP.
- **ADR-006: pg_advisory_lock OBDRLDV (`0x4F4244524C4456`)** — отдельный lock от forecast (FCTERGNE). Cron 06:00 после forecast 05:00.
- **ADR-007: Plan status расширен до `converted`** — добавлен через ALTER CHECK без миграции данных. Forecast service не должен ломаться (mappers + validators в forecast не reject'ят новый статус, только enum-validator query фильтра — но тот не знает converted и вернёт 400 для query, что приемлемо).
- **ADR-008: Партицирование purchase_orders по created_at monthly, retention 365d** — те же паттерны что forecasts/marts.

## Enums и константы

```go
// constants/constants.go
const (
    POStatusDraft           = "draft"
    POStatusReadyToSend     = "ready_to_send"
    POStatusSent            = "sent"
    POStatusConfirmedByERP  = "confirmed_by_erp"
    POStatusReceived        = "received"
    POStatusCancelled       = "cancelled"

    PlanStatusConverted = "converted"

    PricingSourceProduct         = "product"
    PricingSourceSupplierDefault = "supplier_default"
    PricingSourceMissing         = "missing"

    DefaultCurrency  = "UAH"
    DefaultLeadTime  = 7

    AdvisoryLockKey int64 = 0x4F4244524C4456 // "OBDRLDV"
)
```

## Rollback

- Cron disable: `FORECAST_BUILDER_CRON_SCHEDULE=""` (scheduler пропускает Start).
- DB: `4001_orders_schema.down.sql` — DROP SCHEMA orders CASCADE + ALTER CHECK (откат до 3 статусов).
- Plan_id с `status='converted'` нужно вернуть в `'approved'` (rollback SQL это делает в одной транзакции).
