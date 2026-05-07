-- Создание временных таблиц pg_temp.stg_* для инкрементальной загрузки
-- внутри одной транзакции ETL run. Все таблицы — ON COMMIT DROP, чтобы
-- они автоматически уничтожались по завершении tx (комит/rollback).
--
-- Поля таблиц совпадают с DTO Modul 1 source-adapter (см.
-- internal/features/etl_validation/models/staging.go).

CREATE TEMP TABLE IF NOT EXISTS stg_receipt_line (
    receipt_id        TEXT        NOT NULL,
    location_id       TEXT        NOT NULL,
    product_id        TEXT        NOT NULL,
    line_kind         TEXT        NOT NULL,
    qty               NUMERIC(18,4) NOT NULL DEFAULT 0,
    unit_price_list   NUMERIC(18,4) NOT NULL DEFAULT 0,
    unit_price_paid   NUMERIC(18,4) NOT NULL DEFAULT 0,
    discount_amount   NUMERIC(18,4) NOT NULL DEFAULT 0,
    had_promo         BOOLEAN     NOT NULL DEFAULT false,
    promo_type        TEXT,
    event_time        TIMESTAMPTZ NOT NULL
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_stock_on_hand (
    product_id     TEXT        NOT NULL,
    location_id    TEXT        NOT NULL,
    qty_on_hand    NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_in_transit NUMERIC(18,4) NOT NULL DEFAULT 0,
    as_of_date     DATE        NOT NULL DEFAULT CURRENT_DATE
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_products (
    id          TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    category_id TEXT,
    unit_id     TEXT,
    is_active   BOOLEAN     NOT NULL DEFAULT true
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_locations (
    id        TEXT        NOT NULL,
    name      TEXT        NOT NULL,
    is_active BOOLEAN     NOT NULL DEFAULT true
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_suppliers (
    id        TEXT        NOT NULL,
    name      TEXT        NOT NULL,
    is_active BOOLEAN     NOT NULL DEFAULT true
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_order_rule (
    id                     TEXT        NOT NULL,
    product_id             TEXT        NOT NULL,
    location_id            TEXT        NOT NULL,
    formula                TEXT        NOT NULL,
    safety_stock           NUMERIC(18,4),
    forecast_horizon_days  INTEGER,
    daily_demand           NUMERIC(18,4),
    min_qty                NUMERIC(18,4),
    max_qty                NUMERIC(18,4),
    supplier_id            TEXT,
    lead_time_days         INTEGER,
    is_active              BOOLEAN     NOT NULL DEFAULT true
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_supply_spec (
    id             TEXT        NOT NULL,
    supplier_id    TEXT        NOT NULL,
    product_id     TEXT        NOT NULL,
    location_id    TEXT,
    lead_time_days INTEGER,
    safety_stock   NUMERIC(18,4),
    min_qty        NUMERIC(18,4),
    max_qty        NUMERIC(18,4),
    is_active      BOOLEAN     NOT NULL DEFAULT true
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_receiving_details (
    supplier_id      TEXT        NOT NULL,
    product_id       TEXT        NOT NULL,
    delivery_date    DATE        NOT NULL,
    fill_rate        NUMERIC(8,4) NOT NULL DEFAULT 0,
    otif             BOOLEAN     NOT NULL DEFAULT false,
    lead_time_actual NUMERIC(10,2) NOT NULL DEFAULT 0,
    qty_short        NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_damaged      NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_returned     NUMERIC(18,4) NOT NULL DEFAULT 0,
    late             BOOLEAN     NOT NULL DEFAULT false
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_promo (
    id          TEXT        NOT NULL,
    product_id  TEXT        NOT NULL,
    type        TEXT,
    date_from   DATE        NOT NULL,
    date_to     DATE        NOT NULL
) ON COMMIT DROP;

CREATE TEMP TABLE IF NOT EXISTS stg_store_assortment (
    product_id      TEXT        NOT NULL,
    location_id     TEXT        NOT NULL,
    valid_from      DATE        NOT NULL,
    valid_to        DATE,
    lifecycle_state TEXT
) ON COMMIT DROP;
