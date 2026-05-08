-- mock-erp PostgreSQL schema. Idempotent (CREATE TABLE IF NOT EXISTS).
-- Mirrors the contract used by source-adapter erp_e_zoo_reader.go.

-- ===== Master entities =====

CREATE TABLE IF NOT EXISTS products (
    id          VARCHAR(64) PRIMARY KEY,
    sku         VARCHAR(64) NOT NULL,
    name        TEXT        NOT NULL,
    category_id VARCHAR(64) NOT NULL,
    unit        VARCHAR(16) NOT NULL,
    pack_size   DOUBLE PRECISION NOT NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    attributes  JSONB,
    updated_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_products_sku         ON products (sku);
CREATE INDEX IF NOT EXISTS ix_products_category_id ON products (category_id);
CREATE INDEX IF NOT EXISTS ix_products_updated_at  ON products (updated_at);

CREATE TABLE IF NOT EXISTS product_barcodes (
    barcode    VARCHAR(64) PRIMARY KEY,
    product_id VARCHAR(64) NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_product_barcodes_product_id ON product_barcodes (product_id);

CREATE TABLE IF NOT EXISTS category (
    id         VARCHAR(64) PRIMARY KEY,
    name       TEXT        NOT NULL,
    path       TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_category_updated_at ON category (updated_at);

CREATE TABLE IF NOT EXISTS location (
    id         VARCHAR(64) PRIMARY KEY,
    type       VARCHAR(16) NOT NULL,
    name       TEXT        NOT NULL,
    region     TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_location_updated_at ON location (updated_at);

CREATE TABLE IF NOT EXISTS supplier (
    id         VARCHAR(64) PRIMARY KEY,
    name       TEXT        NOT NULL,
    inn        VARCHAR(32),
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_supplier_updated_at ON supplier (updated_at);

CREATE TABLE IF NOT EXISTS supply_spec (
    row_id         BIGSERIAL    PRIMARY KEY,
    product_id     VARCHAR(64)  NOT NULL,
    supplier_id    VARCHAR(64)  NOT NULL,
    pack_qty       INTEGER      NOT NULL,
    lead_time_days INTEGER      NOT NULL,
    min_order_qty  INTEGER      NOT NULL,
    multiple       INTEGER      NOT NULL,
    valid_from     TIMESTAMPTZ  NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_supply_spec_product_id  ON supply_spec (product_id);
CREATE INDEX IF NOT EXISTS ix_supply_spec_supplier_id ON supply_spec (supplier_id);
CREATE INDEX IF NOT EXISTS ix_supply_spec_valid_from  ON supply_spec (valid_from);

CREATE TABLE IF NOT EXISTS promo (
    id           VARCHAR(64) PRIMARY KEY,
    location_id  VARCHAR(64) NOT NULL,
    product_id   VARCHAR(64) NOT NULL,
    start_date   TIMESTAMPTZ NOT NULL,
    end_date     TIMESTAMPTZ NOT NULL,
    discount_pct INTEGER     NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_promo_location_id ON promo (location_id);
CREATE INDEX IF NOT EXISTS ix_promo_product_id  ON promo (product_id);
CREATE INDEX IF NOT EXISTS ix_promo_updated_at  ON promo (updated_at);

CREATE TABLE IF NOT EXISTS order_rule (
    id          VARCHAR(64) PRIMARY KEY,
    location_id VARCHAR(64) NOT NULL,
    rule_type   VARCHAR(32) NOT NULL,
    payload     JSONB,
    valid_from  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_order_rule_location_id ON order_rule (location_id);
CREATE INDEX IF NOT EXISTS ix_order_rule_valid_from  ON order_rule (valid_from);

CREATE TABLE IF NOT EXISTS supply_plan (
    id          VARCHAR(64) PRIMARY KEY,
    location_id VARCHAR(64) NOT NULL,
    product_id  VARCHAR(64) NOT NULL,
    supplier_id VARCHAR(64) NOT NULL,
    plan_date   TIMESTAMPTZ NOT NULL,
    qty         INTEGER     NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_supply_plan_plan_date ON supply_plan (plan_date);

CREATE TABLE IF NOT EXISTS store_assortment (
    row_id      BIGSERIAL   PRIMARY KEY,
    location_id VARCHAR(64) NOT NULL,
    product_id  VARCHAR(64) NOT NULL,
    start_date  TIMESTAMPTZ NOT NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_store_assortment_location_id ON store_assortment (location_id);
CREATE INDEX IF NOT EXISTS ix_store_assortment_product_id  ON store_assortment (product_id);
CREATE INDEX IF NOT EXISTS ix_store_assortment_updated_at  ON store_assortment (updated_at);

CREATE TABLE IF NOT EXISTS store_assortment_lifecycle_events (
    row_id      BIGSERIAL   PRIMARY KEY,
    location_id VARCHAR(64) NOT NULL,
    product_id  VARCHAR(64) NOT NULL,
    event_type  VARCHAR(16) NOT NULL,
    event_date  TIMESTAMPTZ NOT NULL,
    payload     JSONB
);
CREATE INDEX IF NOT EXISTS ix_store_assort_lc_event_date ON store_assortment_lifecycle_events (event_date);

CREATE TABLE IF NOT EXISTS master_change_log (
    row_id     BIGSERIAL   PRIMARY KEY,
    entity     VARCHAR(64) NOT NULL,
    entity_pk  JSONB,
    field      TEXT        NOT NULL,
    old_value  TEXT,
    new_value  TEXT,
    changed_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_master_change_log_entity     ON master_change_log (entity);
CREATE INDEX IF NOT EXISTS ix_master_change_log_changed_at ON master_change_log (changed_at);

-- ===== Facts entities =====

CREATE TABLE IF NOT EXISTS receipt_line (
    id          BIGSERIAL    PRIMARY KEY,
    receipt_id  VARCHAR(64)  NOT NULL,
    location_id VARCHAR(64)  NOT NULL,
    product_id  VARCHAR(64)  NOT NULL,
    qty         INTEGER      NOT NULL,
    price       DOUBLE PRECISION NOT NULL,
    event_time  TIMESTAMPTZ  NOT NULL,
    event_date  TIMESTAMPTZ  NOT NULL,
    payload     JSONB
);
CREATE INDEX IF NOT EXISTS ix_receipt_line_event_date    ON receipt_line (event_date);
CREATE INDEX IF NOT EXISTS ix_receipt_line_event_date_id ON receipt_line (event_date, id);
CREATE INDEX IF NOT EXISTS ix_receipt_line_location_id   ON receipt_line (location_id);
CREATE INDEX IF NOT EXISTS ix_receipt_line_product_id    ON receipt_line (product_id);

CREATE TABLE IF NOT EXISTS location_stock_snapshot (
    row_id       BIGSERIAL    PRIMARY KEY,
    event_date   TIMESTAMPTZ  NOT NULL,
    location_id  VARCHAR(64)  NOT NULL,
    product_id   VARCHAR(64)  NOT NULL,
    qty_on_hand  INTEGER      NOT NULL,
    qty_reserved INTEGER      NOT NULL,
    as_of        TIMESTAMPTZ  NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_loc_stock_event_date     ON location_stock_snapshot (event_date);
CREATE INDEX IF NOT EXISTS ix_loc_stock_event_date_row ON location_stock_snapshot (event_date, row_id);
CREATE INDEX IF NOT EXISTS ix_loc_stock_location_id    ON location_stock_snapshot (location_id);
CREATE INDEX IF NOT EXISTS ix_loc_stock_product_id     ON location_stock_snapshot (product_id);

CREATE TABLE IF NOT EXISTS stock_movement (
    id            BIGSERIAL    PRIMARY KEY,
    event_date    TIMESTAMPTZ  NOT NULL,
    event_time    TIMESTAMPTZ  NOT NULL,
    location_id   VARCHAR(64)  NOT NULL,
    product_id    VARCHAR(64)  NOT NULL,
    movement_type VARCHAR(32)  NOT NULL,
    qty           INTEGER      NOT NULL,
    ref_id        VARCHAR(128) NOT NULL,
    payload       JSONB
);
CREATE INDEX IF NOT EXISTS ix_stock_movement_event_date    ON stock_movement (event_date);
CREATE INDEX IF NOT EXISTS ix_stock_movement_event_date_id ON stock_movement (event_date, id);
CREATE INDEX IF NOT EXISTS ix_stock_movement_location_id   ON stock_movement (location_id);
CREATE INDEX IF NOT EXISTS ix_stock_movement_product_id    ON stock_movement (product_id);

CREATE TABLE IF NOT EXISTS supplier_stock_snapshot (
    row_id      BIGSERIAL    PRIMARY KEY,
    event_date  TIMESTAMPTZ  NOT NULL,
    supplier_id VARCHAR(64)  NOT NULL,
    product_id  VARCHAR(64)  NOT NULL,
    qty         INTEGER      NOT NULL,
    as_of       TIMESTAMPTZ  NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_sup_stock_event_date     ON supplier_stock_snapshot (event_date);
CREATE INDEX IF NOT EXISTS ix_sup_stock_event_date_row ON supplier_stock_snapshot (event_date, row_id);
CREATE INDEX IF NOT EXISTS ix_sup_stock_supplier_id    ON supplier_stock_snapshot (supplier_id);

-- ===== Received orders (write side) =====

CREATE TABLE IF NOT EXISTS received_orders (
    id           BIGSERIAL    PRIMARY KEY,
    external_ref VARCHAR(64)  NOT NULL,
    po_number    VARCHAR(128),
    supplier_id  VARCHAR(64),
    location_id  VARCHAR(64),
    accepted_at  TIMESTAMPTZ  NOT NULL,
    raw_body     JSONB        NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_received_orders_external_ref ON received_orders (external_ref);
CREATE INDEX IF NOT EXISTS ix_received_orders_accepted_at  ON received_orders (accepted_at);

-- ===== Seeder state =====
-- Single-row table tracking the day-by-day on-demand seeder.
-- master_seeded: TRUE after the initial bootstrap (master entities) has run.
-- current_date: the next day to be generated when /admin/seed/days fires.
-- demand_map: JSONB encoding {"<product_id>|<location_id>": <int_lambda>} —
--   built once during master seed and re-used by every day-batch so that
--   demand stays stable across runs.
CREATE TABLE IF NOT EXISTS seeder_state (
    id                INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    master_seeded     BOOLEAN NOT NULL DEFAULT FALSE,
    current_seed_date TIMESTAMPTZ,
    days_generated    INTEGER NOT NULL DEFAULT 0,
    demand_map        JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO seeder_state (id, master_seeded, current_seed_date, days_generated)
VALUES (1, FALSE, NULL, 0)
ON CONFLICT (id) DO NOTHING;
