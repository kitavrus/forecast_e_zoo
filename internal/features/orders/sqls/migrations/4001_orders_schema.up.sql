-- Module 6: Order Builder schema (order-builder).
-- Schema orders: purchase_orders (partitioned monthly) + po_lines + po_status_history + po_number_seq.

CREATE SCHEMA IF NOT EXISTS orders;

-- Service role e_zoo_app: USAGE на схему. DML default privileges объявлены
-- в infra/pg/init/01_init.sh для роли-владельца.
GRANT USAGE ON SCHEMA orders TO e_zoo_app;

-- Sequence for PO numbering: PO-YYYYMMDD-NNNNNN.
CREATE SEQUENCE IF NOT EXISTS orders.po_number_seq AS BIGINT START 1;

-- 1) purchase_orders — partitioned RANGE by created_at (monthly).
CREATE TABLE IF NOT EXISTS orders.purchase_orders (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    po_number       TEXT NOT NULL,
    plan_id         UUID NOT NULL,
    supplier_id     TEXT NOT NULL,
    location_id     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'ready_to_send' CHECK (status IN
        ('draft','ready_to_send','sent','confirmed_by_erp','received','cancelled')),
    total_qty       NUMERIC(18,4) NOT NULL,
    total_amount    NUMERIC(18,4),
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

CREATE TABLE IF NOT EXISTS orders.purchase_orders_2026_05 PARTITION OF orders.purchase_orders
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS orders.purchase_orders_2026_06 PARTITION OF orders.purchase_orders
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS orders.purchase_orders_2026_07 PARTITION OF orders.purchase_orders
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- Partition key (created_at) обязан входить в UNIQUE-индекс на partitioned таблице.
-- PO number имеет date-prefix (PO-YYYYMMDD-NNNNNN), поэтому уникальность сохраняется по построению.
CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_orders_po_number
    ON orders.purchase_orders(po_number, created_at);

-- 1:1 plan→PO для не-cancelled (regenerate помечает старый cancelled и вставляет новый).
-- Partition key (created_at) добавлен по требованию PG для partitioned-таблицы.
CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_orders_plan_active
    ON orders.purchase_orders(plan_id, created_at) WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_purchase_orders_status
    ON orders.purchase_orders(status);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_supplier_created
    ON orders.purchase_orders(supplier_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_plan_id
    ON orders.purchase_orders(plan_id);

-- 2) po_lines — позиции PO.
CREATE TABLE IF NOT EXISTS orders.po_lines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id           UUID NOT NULL,
    product_id      TEXT NOT NULL,
    qty             NUMERIC(18,4) NOT NULL CHECK (qty > 0),
    unit_price      NUMERIC(18,4),
    line_amount     NUMERIC(18,4),
    pricing_source  TEXT CHECK (pricing_source IN ('product','supplier_default','missing')),
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_po_lines_po_id
    ON orders.po_lines(po_id);
CREATE INDEX IF NOT EXISTS idx_po_lines_product
    ON orders.po_lines(product_id);

-- 3) po_status_history — audit log переходов статуса PO.
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
CREATE INDEX IF NOT EXISTS idx_po_status_history_changed_at
    ON orders.po_status_history(changed_at DESC);

-- 4) Расширяем replenishment_plans статусом 'converted'.
ALTER TABLE forecast.replenishment_plans
    DROP CONSTRAINT IF EXISTS replenishment_plans_status_check;
ALTER TABLE forecast.replenishment_plans
    ADD CONSTRAINT replenishment_plans_status_check
    CHECK (status IN ('draft','approved','cancelled','converted'));
