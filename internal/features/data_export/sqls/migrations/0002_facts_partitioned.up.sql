-- =============================================================================
-- Migration 0002: facts partitioned by event_date (PG18 declarative RANGE).
-- 4 партиционированные таблицы + initial по 4 месячные партиции на каждую.
-- Долгосрочное управление новыми партициями — фаза 12 (cron-tick).
-- =============================================================================

-- ============================================================
-- 1. receipt_line — линии чеков, партиционировано помесячно по event_date.
-- ============================================================
CREATE TABLE IF NOT EXISTS receipt_line (
    id          bigint      NOT NULL,
    receipt_id  text        NOT NULL,
    location_id text        NOT NULL,
    product_id  text        NOT NULL,
    qty         numeric     NOT NULL,
    price       numeric     NOT NULL,
    event_time  timestamptz NOT NULL,
    event_date  date        NOT NULL,
    payload     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    load_id     uuid        NULL,
    PRIMARY KEY (event_date, id)
) PARTITION BY RANGE (event_date);

CREATE INDEX IF NOT EXISTS idx_receipt_line_load
    ON receipt_line (load_id);
CREATE INDEX IF NOT EXISTS idx_receipt_line_loc_date
    ON receipt_line (location_id, event_date);
CREATE INDEX IF NOT EXISTS idx_receipt_line_prod_date
    ON receipt_line (product_id, event_date);

CREATE TABLE IF NOT EXISTS receipt_line_y2026m04
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m05
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m06
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS receipt_line_y2026m07
    PARTITION OF receipt_line
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- ============================================================
-- 2. location_stock_snapshot — стоковые срезы по точкам.
-- ============================================================
CREATE TABLE IF NOT EXISTS location_stock_snapshot (
    event_date    date        NOT NULL,
    location_id   text        NOT NULL,
    product_id    text        NOT NULL,
    qty_on_hand   numeric     NOT NULL,
    qty_reserved  numeric     NOT NULL DEFAULT 0,
    as_of         timestamptz NOT NULL,
    load_id       uuid        NULL,
    PRIMARY KEY (event_date, location_id, product_id)
) PARTITION BY RANGE (event_date);

CREATE INDEX IF NOT EXISTS idx_location_stock_snapshot_load
    ON location_stock_snapshot (load_id);
CREATE INDEX IF NOT EXISTS idx_location_stock_snapshot_loc_date_desc
    ON location_stock_snapshot (location_id, event_date DESC);

CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m04
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m05
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m06
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS location_stock_snapshot_y2026m07
    PARTITION OF location_stock_snapshot
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- ============================================================
-- 3. stock_movement — движения остатков.
-- ============================================================
CREATE TABLE IF NOT EXISTS stock_movement (
    id            bigint      NOT NULL,
    event_date    date        NOT NULL,
    event_time    timestamptz NOT NULL,
    location_id   text        NOT NULL,
    product_id    text        NOT NULL,
    movement_type text        NOT NULL,
    qty           numeric     NOT NULL,
    ref_id        text        NULL,
    payload       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    load_id       uuid        NULL,
    PRIMARY KEY (event_date, id)
) PARTITION BY RANGE (event_date);

CREATE INDEX IF NOT EXISTS idx_stock_movement_load
    ON stock_movement (load_id);
CREATE INDEX IF NOT EXISTS idx_stock_movement_loc_prod_date
    ON stock_movement (location_id, product_id, event_date);

CREATE TABLE IF NOT EXISTS stock_movement_y2026m04
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m05
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m06
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS stock_movement_y2026m07
    PARTITION OF stock_movement
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- ============================================================
-- 4. supplier_stock_snapshot — стоковые срезы у поставщиков.
-- ============================================================
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot (
    event_date     date        NOT NULL,
    supplier_id    text        NOT NULL,
    product_id     text        NOT NULL,
    qty_available  numeric     NOT NULL,
    as_of          timestamptz NOT NULL,
    load_id        uuid        NULL,
    PRIMARY KEY (event_date, supplier_id, product_id)
) PARTITION BY RANGE (event_date);

CREATE INDEX IF NOT EXISTS idx_supplier_stock_snapshot_load
    ON supplier_stock_snapshot (load_id);
CREATE INDEX IF NOT EXISTS idx_supplier_stock_snapshot_sup_date_desc
    ON supplier_stock_snapshot (supplier_id, event_date DESC);

CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m04
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m05
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m06
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS supplier_stock_snapshot_y2026m07
    PARTITION OF supplier_stock_snapshot
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
