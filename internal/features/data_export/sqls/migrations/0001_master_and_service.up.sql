-- =============================================================================
-- Migration 0001: master + service tables (source-adapter, MVP)
-- 17 не-партиционированных таблиц.
-- Партиционированные факты — в миграции 0002.
-- =============================================================================

-- LTREE для иерархии категорий (опционально; категории также имеют parent_id).
CREATE EXTENSION IF NOT EXISTS ltree;

-- ============================================================
-- 1. loads — журнал запусков load-цикла.
-- ============================================================
CREATE TABLE IF NOT EXISTS loads (
    load_id        uuid        PRIMARY KEY,
    started_at     timestamptz NOT NULL DEFAULT now(),
    committed_at   timestamptz NULL,
    failed_at      timestamptz NULL,
    status         text        NOT NULL
                                CHECK (status IN ('running','committed','failed','aborted')),
    failure_reason text        NULL,
    source         text        NOT NULL,
    lines_total    bigint      NOT NULL DEFAULT 0,
    lines_failed   bigint      NOT NULL DEFAULT 0,
    entity_stats   jsonb       NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_loads_status_started
    ON loads (status, started_at);
CREATE INDEX IF NOT EXISTS idx_loads_source_started_desc
    ON loads (source, started_at DESC);

-- ============================================================
-- 2. snapshot_pointer — single-row table (id=1) с указателем на текущий committed snapshot.
-- ============================================================
CREATE TABLE IF NOT EXISTS snapshot_pointer (
    id                 smallint    PRIMARY KEY DEFAULT 1
                                    CHECK (id = 1),
    current_load_id    uuid        NULL REFERENCES loads(load_id) ON DELETE RESTRICT,
    previous_load_id   uuid        NULL REFERENCES loads(load_id) ON DELETE RESTRICT,
    committed_at       timestamptz NULL
);
-- Seed единственной строки.
INSERT INTO snapshot_pointer (id, current_load_id, previous_load_id, committed_at)
VALUES (1, NULL, NULL, NULL)
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 3. reject_log — отклонённые строки (severity error/warn).
-- ============================================================
CREATE TABLE IF NOT EXISTS reject_log (
    id         bigserial   PRIMARY KEY,
    load_id    uuid        NOT NULL REFERENCES loads(load_id) ON DELETE CASCADE,
    entity     text        NOT NULL,
    payload    jsonb       NOT NULL,
    errors     jsonb       NOT NULL DEFAULT '[]'::jsonb,
    severity   text        NOT NULL CHECK (severity IN ('error','warn')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_reject_log_load_entity
    ON reject_log (load_id, entity);
CREATE INDEX IF NOT EXISTS idx_reject_log_created_at
    ON reject_log (created_at);

-- ============================================================
-- 4. entity_checkpoint — последний committed load по сущности.
-- ============================================================
CREATE TABLE IF NOT EXISTS entity_checkpoint (
    entity            text        PRIMARY KEY,
    last_load_id      uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL,
    last_committed_at timestamptz NULL
);

-- ============================================================
-- 5. audit_access — лог доступа к /admin/*.
-- TODO: ретеншн 365d (см. design-sql §9, AUDIT_RETENTION).
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_access (
    id          bigserial   PRIMARY KEY,
    at          timestamptz NOT NULL DEFAULT now(),
    actor_role  text        NOT NULL,
    actor_sub   text        NOT NULL,
    method      text        NOT NULL,
    path        text        NOT NULL,
    status      int         NOT NULL,
    trace_id    text        NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_access_at_desc
    ON audit_access (at DESC);

-- ============================================================
-- 6. category — справочник категорий.
-- ============================================================
CREATE TABLE IF NOT EXISTS category (
    id         text        PRIMARY KEY,
    parent_id  text        NULL REFERENCES category(id) ON DELETE SET NULL,
    name       text        NOT NULL,
    path       ltree       NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    load_id    uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_category_parent_id
    ON category (parent_id);

-- ============================================================
-- 7. location — точки/склады.
-- ============================================================
CREATE TABLE IF NOT EXISTS location (
    id         text        PRIMARY KEY,
    type       text        NOT NULL,
    name       text        NOT NULL,
    region     text        NULL,
    address    text        NULL,
    lat        numeric     NULL,
    lon        numeric     NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    load_id    uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);

-- ============================================================
-- 8. supplier — поставщики.
-- ============================================================
CREATE TABLE IF NOT EXISTS supplier (
    id         text        PRIMARY KEY,
    name       text        NOT NULL,
    inn        text        NULL,
    kpp        text        NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    load_id    uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);

-- ============================================================
-- 9. products — товары (master).
-- ============================================================
CREATE TABLE IF NOT EXISTS products (
    id          text        PRIMARY KEY,
    sku         text        NOT NULL UNIQUE,
    name        text        NOT NULL,
    category_id text        NULL REFERENCES category(id) ON DELETE SET NULL,
    unit        text        NOT NULL,
    pack_size   numeric     NULL,
    is_active   boolean     NOT NULL DEFAULT TRUE,
    attributes  jsonb       NOT NULL DEFAULT '{}'::jsonb,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    load_id     uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_products_category_id
    ON products (category_id);

-- ============================================================
-- 10. product_barcodes — штрихкоды товара (1:N).
-- ============================================================
CREATE TABLE IF NOT EXISTS product_barcodes (
    product_id text    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    barcode    text    NOT NULL,
    is_primary boolean NOT NULL DEFAULT FALSE,
    PRIMARY KEY (product_id, barcode)
);
CREATE INDEX IF NOT EXISTS idx_product_barcodes_barcode
    ON product_barcodes (barcode);

-- ============================================================
-- 11. store_assortment — текущий ассортимент по точке/товару.
-- ============================================================
CREATE TABLE IF NOT EXISTS store_assortment (
    location_id text        NOT NULL REFERENCES location(id) ON DELETE CASCADE,
    product_id  text        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    start_date  date        NOT NULL,
    end_date    date        NULL,
    is_active   boolean     NOT NULL DEFAULT TRUE,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    load_id     uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL,
    PRIMARY KEY (location_id, product_id)
);

-- ============================================================
-- 12. store_assortment_lifecycle_events — события lifecycle (start/stop/promo).
-- ============================================================
CREATE TABLE IF NOT EXISTS store_assortment_lifecycle_events (
    id          bigserial   PRIMARY KEY,
    location_id text        NOT NULL REFERENCES location(id) ON DELETE CASCADE,
    product_id  text        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    event_type  text        NOT NULL CHECK (event_type IN ('start','stop','promo')),
    event_date  date        NOT NULL,
    payload     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    load_id     uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_lifecycle_loc_prod_date_desc
    ON store_assortment_lifecycle_events (location_id, product_id, event_date DESC);

-- ============================================================
-- 13. supply_spec — спецификация поставки (товар × поставщик × valid_from).
-- ============================================================
CREATE TABLE IF NOT EXISTS supply_spec (
    product_id     text    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    supplier_id    text    NOT NULL REFERENCES supplier(id) ON DELETE CASCADE,
    pack_qty       numeric NULL,
    lead_time_days int     NULL,
    min_order_qty  numeric NULL,
    multiple       numeric NULL,
    valid_from     date    NOT NULL,
    valid_to       date    NULL,
    load_id        uuid    NULL REFERENCES loads(load_id) ON DELETE SET NULL,
    PRIMARY KEY (product_id, supplier_id, valid_from)
);

-- ============================================================
-- 14. promo — акции (точка × товар × период).
-- ============================================================
CREATE TABLE IF NOT EXISTS promo (
    id           text        PRIMARY KEY,
    location_id  text        NOT NULL REFERENCES location(id) ON DELETE CASCADE,
    product_id   text        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    start_date   date        NOT NULL,
    end_date     date        NOT NULL,
    discount_pct numeric     NULL,
    payload      jsonb       NOT NULL DEFAULT '{}'::jsonb,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    load_id      uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_promo_loc_prod_dates
    ON promo (location_id, product_id, start_date, end_date);

-- ============================================================
-- 15. order_rule — правила заказа (на товар или на категорию).
-- ============================================================
CREATE TABLE IF NOT EXISTS order_rule (
    id          text    PRIMARY KEY,
    location_id text    NOT NULL REFERENCES location(id) ON DELETE CASCADE,
    product_id  text    NULL REFERENCES products(id) ON DELETE CASCADE,
    category_id text    NULL REFERENCES category(id) ON DELETE CASCADE,
    rule_type   text    NOT NULL,
    payload     jsonb   NOT NULL DEFAULT '{}'::jsonb,
    valid_from  date    NOT NULL,
    valid_to    date    NULL,
    load_id     uuid    NULL REFERENCES loads(load_id) ON DELETE SET NULL,
    CHECK (product_id IS NOT NULL OR category_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_order_rule_loc_valid_from
    ON order_rule (location_id, valid_from);

-- ============================================================
-- 16. supply_plan — план поставок.
-- ============================================================
CREATE TABLE IF NOT EXISTS supply_plan (
    id          text        PRIMARY KEY,
    location_id text        NOT NULL REFERENCES location(id) ON DELETE CASCADE,
    product_id  text        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    supplier_id text        NOT NULL REFERENCES supplier(id) ON DELETE CASCADE,
    plan_date   date        NOT NULL,
    qty         numeric     NOT NULL,
    payload     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    load_id     uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_supply_plan_loc_prod_date
    ON supply_plan (location_id, product_id, plan_date);

-- ============================================================
-- 17. master_change_log — журнал изменений master-полей (tracked fields).
-- ============================================================
CREATE TABLE IF NOT EXISTS master_change_log (
    id         bigserial   PRIMARY KEY,
    entity     text        NOT NULL,
    entity_pk  jsonb       NOT NULL,
    field      text        NOT NULL,
    old_value  jsonb       NULL,
    new_value  jsonb       NULL,
    changed_at timestamptz NOT NULL DEFAULT now(),
    load_id    uuid        NULL REFERENCES loads(load_id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_master_change_log_entity_changed_desc
    ON master_change_log (entity, changed_at DESC);
