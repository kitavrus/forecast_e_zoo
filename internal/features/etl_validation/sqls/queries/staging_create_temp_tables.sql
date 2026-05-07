-- Создание временных таблиц pg_temp.stg_* для инкрементальной загрузки
-- внутри одной транзакции ETL run. Все таблицы — ON COMMIT DROP, чтобы
-- они автоматически уничтожались по завершении tx (комит/rollback).
--
-- Поля таблиц совпадают с DTO Modul 1 source-adapter
-- (internal/features/data_export/models/dto/*.go) — см. также Go-модели в
-- internal/features/etl_validation/models/staging.go.
--
-- Принципы:
--   1. Имена колонок строго совпадают с json-тегами DTO source-adapter
--      (single-source-of-truth — DTO).
--   2. NOT NULL только на строго обязательных полях (ID-поля + event_time для
--      facts). Опциональные/derived-поля — NULLABLE, чтобы COPY не падал, если
--      source-adapter не вернул значение (например, derived-флаги, опциональные
--      reference-поля). Дефолтная политика: «лучше NULL → NULL, чем падение
--      pipeline-а».

-- stg_receipt_line — DTO dto.ReceiptLine.
-- ВАЖНО:
--   * unit_price_base / unit_price_paid — имена json source-adapter
--     (раньше было unit_price_list — поле, которого нет в DTO).
--   * had_promo / promo_type — derived-поля, источник их не отдаёт; они
--     вычисляются JOIN-ом с stg_promo в mart_demand_history. Здесь —
--     NULLABLE с default'ом, чтобы COPY с пустыми значениями не падал.
CREATE TEMP TABLE IF NOT EXISTS stg_receipt_line (
    receipt_id        TEXT        NOT NULL,
    location_id       TEXT        NOT NULL,
    product_id        TEXT        NOT NULL,
    line_kind         TEXT        NOT NULL,
    qty               NUMERIC(18,4) NOT NULL DEFAULT 0,
    unit_price_base   NUMERIC(18,4) NULL DEFAULT 0,
    unit_price_paid   NUMERIC(18,4) NULL DEFAULT 0,
    discount_amount   NUMERIC(18,4) NULL DEFAULT 0,
    had_promo         BOOLEAN     NULL DEFAULT false,
    promo_type        TEXT        NULL,
    event_time        TIMESTAMPTZ NOT NULL
) ON COMMIT DROP;

-- stg_stock_on_hand — fact-сущность, source-adapter MVP отдаёт через
-- /v1/location_stock_snapshot (DTO dto.LocationStockSnapshot). В MVP
-- маппинг dto→staging держим минимальным: PK + qty_on_hand + as_of_date.
-- qty_in_transit отсутствует в DTO LocationStockSnapshot — NULLABLE.
CREATE TEMP TABLE IF NOT EXISTS stg_stock_on_hand (
    product_id     TEXT        NOT NULL,
    location_id    TEXT        NOT NULL,
    qty_on_hand    NUMERIC(18,4) NULL DEFAULT 0,
    qty_in_transit NUMERIC(18,4) NULL DEFAULT 0,
    as_of_date     DATE        NULL DEFAULT CURRENT_DATE
) ON COMMIT DROP;

-- stg_products — DTO dto.Product. PK source-adapter — product_id, не id.
-- status — TEXT (active/discontinued/...), staging хранит как is_active
-- (boolean) — деривируется в transformer, источник его не отдаёт.
CREATE TEMP TABLE IF NOT EXISTS stg_products (
    product_id  TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    category_id TEXT        NULL,
    unit_id     TEXT        NULL,
    status      TEXT        NULL,
    is_active   BOOLEAN     NULL DEFAULT true
) ON COMMIT DROP;

-- stg_locations — DTO dto.Location. PK source-adapter — location_id.
CREATE TEMP TABLE IF NOT EXISTS stg_locations (
    location_id TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    status      TEXT        NULL,
    is_active   BOOLEAN     NULL DEFAULT true
) ON COMMIT DROP;

-- stg_suppliers — DTO dto.Supplier. PK source-adapter — supplier_id.
CREATE TEMP TABLE IF NOT EXISTS stg_suppliers (
    supplier_id TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    status      TEXT        NULL,
    is_active   BOOLEAN     NULL DEFAULT true
) ON COMMIT DROP;

-- stg_order_rule — DTO dto.OrderRule. PK source-adapter — rule_id; продукт
-- задаётся через scope/scope_ref (не отдельным product_id). В staging
-- сохраняем плоский набор для transformer-а; обязательные поля минимальны.
CREATE TEMP TABLE IF NOT EXISTS stg_order_rule (
    rule_id                TEXT        NOT NULL,
    scope                  TEXT        NULL,
    scope_ref              TEXT        NULL,
    product_id             TEXT        NULL,
    location_id            TEXT        NULL,
    formula                TEXT        NULL,
    safety_stock           NUMERIC(18,4) NULL,
    safety_stock_days      NUMERIC(10,2) NULL,
    service_level_pct      NUMERIC(5,2)  NULL,
    forecast_horizon_days  INTEGER     NULL,
    daily_demand           NUMERIC(18,4) NULL,
    min_qty                NUMERIC(18,4) NULL,
    max_qty                NUMERIC(18,4) NULL,
    override_moq           INTEGER     NULL,
    supplier_id            TEXT        NULL,
    lead_time_days         INTEGER     NULL,
    is_active              BOOLEAN     NULL DEFAULT true
) ON COMMIT DROP;

-- stg_supply_spec — DTO dto.SupplySpec. Composite-PK = (supplier_id, product_id, location_id).
CREATE TEMP TABLE IF NOT EXISTS stg_supply_spec (
    supplier_id    TEXT        NOT NULL,
    product_id     TEXT        NOT NULL,
    location_id    TEXT        NULL,
    lead_time_days INTEGER     NULL,
    safety_stock   NUMERIC(18,4) NULL,
    min_qty        NUMERIC(18,4) NULL,
    max_qty        NUMERIC(18,4) NULL,
    min_order_qty  INTEGER     NULL,
    purchase_price NUMERIC(18,4) NULL,
    currency       TEXT        NULL,
    pack_size      INTEGER     NULL,
    is_active      BOOLEAN     NULL DEFAULT true
) ON COMMIT DROP;

-- stg_receiving_details — entity не реализован в source-adapter MVP
-- (см. internal/features/data_export/handler/not_implemented.go). Таблица
-- остаётся ради совместимости с mart_supplier_scorecard_insert.sql; до
-- появления handler-а COPY будет вставлять 0 строк.
CREATE TEMP TABLE IF NOT EXISTS stg_receiving_details (
    supplier_id      TEXT        NOT NULL,
    product_id       TEXT        NOT NULL,
    delivery_date    DATE        NOT NULL,
    fill_rate        NUMERIC(8,4) NULL DEFAULT 0,
    otif             BOOLEAN     NULL DEFAULT false,
    lead_time_actual NUMERIC(10,2) NULL DEFAULT 0,
    qty_short        NUMERIC(18,4) NULL DEFAULT 0,
    qty_damaged      NUMERIC(18,4) NULL DEFAULT 0,
    qty_returned     NUMERIC(18,4) NULL DEFAULT 0,
    late             BOOLEAN     NULL DEFAULT false
) ON COMMIT DROP;

-- stg_promo — DTO dto.Promo. PK source-adapter — promo_id (не id).
CREATE TEMP TABLE IF NOT EXISTS stg_promo (
    promo_id    TEXT        NOT NULL,
    product_id  TEXT        NOT NULL,
    location_id TEXT        NULL,
    type        TEXT        NULL,
    date_from   DATE        NULL,
    date_to     DATE        NULL
) ON COMMIT DROP;

-- stg_store_assortment — DTO dto.StoreAssortment. Источник отдаёт
-- effective_from / effective_to; staging переименовывает их в valid_*
-- для совместимости с существующими mart-запросами (date-range JOIN-ы).
CREATE TEMP TABLE IF NOT EXISTS stg_store_assortment (
    product_id      TEXT        NOT NULL,
    location_id     TEXT        NOT NULL,
    effective_from  DATE        NULL,
    effective_to    DATE        NULL,
    valid_from      DATE        NULL,
    valid_to        DATE        NULL,
    lifecycle_state TEXT        NULL
) ON COMMIT DROP;
