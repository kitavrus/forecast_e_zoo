# Design SQL — source-adapter

Все SQL-файлы — через `go:embed`. Миграции — `golang-migrate/v4`. PostgreSQL 18.

---

## 0. Конвенции

- **Файлы миграций:** `internal/features/data_export/sqls/migrations/000N_<name>.up.sql` /
  `000N_<name>.down.sql`. Не модифицируем после применения в prod.
- **Файлы запросов:** `internal/features/data_export/sqls/queries/<entity>_<action>.sql`.
- **Auto-apply:** **запрещён**. Миграции применяются явно через CLI (`migrate -path ... -database ... up`).
- **Партиционирование:** все факты — по `event_date` (PG18 declarative partitioning, RANGE по
  месяцам). Партиции создаются `pg_partman` или ручным скриптом
  `0099_create_monthly_partitions.sql` на следующие 3 месяца — см. §10.
- **PK на партиционированных:** включает `event_date` (требование PG18 для declarative).
- **UPSERT:** `INSERT ... ON CONFLICT (...) DO UPDATE SET ...`. `created_at` всегда immutable
  (`= EXCLUDED.created_at` НЕ перезаписывается).

## 1. Migration 0001 — loads + snapshot_pointer

```sql
-- 0001_init_loads_snapshot.up.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE loads (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at        TIMESTAMPTZ NULL,
    status             TEXT NOT NULL CHECK (status IN ('running','committed','failed')),
    source             TEXT NOT NULL CHECK (source IN ('erp_e_zoo','manual','retry')),
    parent_load_id     UUID NULL REFERENCES loads(id),
    entities_summary   JSONB NULL,
    failure_reason     TEXT NULL
);

CREATE INDEX loads_status_started_idx ON loads (status, started_at DESC);
CREATE INDEX loads_finished_at_idx    ON loads (finished_at DESC) WHERE status = 'committed';

CREATE TABLE snapshot_pointer (
    id                 SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1), -- single-row guard
    current_load_id    UUID NULL REFERENCES loads(id),
    previous_load_id   UUID NULL REFERENCES loads(id),
    committed_at       TIMESTAMPTZ NULL
);

INSERT INTO snapshot_pointer (id) VALUES (1) ON CONFLICT DO NOTHING;
```

```sql
-- 0001_init_loads_snapshot.down.sql
DROP TABLE IF EXISTS snapshot_pointer;
DROP TABLE IF EXISTS loads;
```

### snapshot_flip.sql

```sql
-- queries/snapshot_flip.sql
WITH old AS (
    SELECT current_load_id FROM snapshot_pointer WHERE id = 1 FOR UPDATE
)
UPDATE snapshot_pointer
   SET previous_load_id = (SELECT current_load_id FROM old),
       current_load_id  = $1,
       committed_at     = now()
 WHERE id = 1;
```

### snapshot_select_current.sql

```sql
-- queries/snapshot_select_current.sql
SELECT current_load_id, previous_load_id, committed_at
  FROM snapshot_pointer
 WHERE id = 1;
```

## 2. Migration 0002 — master entities

```sql
-- 0002_init_master.up.sql

CREATE TABLE category (
    category_id   TEXT PRIMARY KEY,
    parent_id     TEXT NULL REFERENCES category(category_id),
    level         SMALLINT NOT NULL CHECK (level BETWEEN 1 AND 10),
    name          TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id       UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX category_load_id_idx    ON category (load_id);
CREATE INDEX category_updated_at_idx ON category (updated_at);

CREATE TABLE location (
    location_id  TEXT PRIMARY KEY,
    type         TEXT NOT NULL CHECK (type IN ('STORE','DC','DARK_STORE')),
    name         TEXT NOT NULL,
    address      TEXT NULL,
    city         TEXT NULL,
    region       TEXT NULL,
    opened_at    TIMESTAMPTZ NULL,
    closed_at    TIMESTAMPTZ NULL,
    status       TEXT NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id      UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX location_load_id_idx    ON location (load_id);
CREATE INDEX location_updated_at_idx ON location (updated_at);

CREATE TABLE products (
    product_id              TEXT PRIMARY KEY,
    name                    TEXT NOT NULL,
    brand                   TEXT NULL,
    manufacturer            TEXT NULL,
    category_id             TEXT NOT NULL REFERENCES category(category_id),
    category_path           TEXT NOT NULL,
    weight_kg               NUMERIC(10,4) NULL,
    pallet_qty              INTEGER NULL,
    shelf_life_days         INTEGER NULL,
    storage_temp_min        NUMERIC(5,2) NULL,
    storage_temp_max        NUMERIC(5,2) NULL,
    requires_prescription   BOOLEAN NOT NULL DEFAULT false,
    is_dangerous_goods      BOOLEAN NOT NULL DEFAULT false,
    status                  TEXT NOT NULL CHECK (status IN ('active','archived')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id                 UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX products_load_id_idx     ON products (load_id);
CREATE INDEX products_updated_at_idx  ON products (updated_at);
CREATE INDEX products_category_id_idx ON products (category_id);

CREATE TABLE product_barcodes (
    barcode         TEXT PRIMARY KEY,
    product_id      TEXT NOT NULL REFERENCES products(product_id),
    pack_qty        INTEGER NOT NULL DEFAULT 1,
    is_primary      BOOLEAN NOT NULL DEFAULT false,
    country_origin  TEXT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id         UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX product_barcodes_product_id_idx ON product_barcodes (product_id);
CREATE INDEX product_barcodes_load_id_idx    ON product_barcodes (load_id);

CREATE TABLE supplier (
    supplier_id    TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    inn            TEXT NULL,
    gln            TEXT NULL,
    payment_terms  TEXT NULL,
    edi_profile    TEXT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id        UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX supplier_load_id_idx    ON supplier (load_id);
CREATE INDEX supplier_updated_at_idx ON supplier (updated_at);

CREATE TABLE supply_spec (
    supplier_id      TEXT NOT NULL REFERENCES supplier(supplier_id),
    product_id       TEXT NOT NULL REFERENCES products(product_id),
    location_id      TEXT NOT NULL REFERENCES location(location_id),
    priority         SMALLINT NOT NULL DEFAULT 1,
    min_order_qty    INTEGER NOT NULL,
    purchase_price   NUMERIC(12,4) NULL,
    currency         TEXT NOT NULL DEFAULT 'UAH',
    lead_time_days   INTEGER NOT NULL,
    pack_size        INTEGER NOT NULL DEFAULT 1,
    effective_from   TIMESTAMPTZ NOT NULL,
    effective_to     TIMESTAMPTZ NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id          UUID NOT NULL REFERENCES loads(id),
    PRIMARY KEY (supplier_id, product_id, location_id, effective_from)
);
CREATE INDEX supply_spec_load_id_idx ON supply_spec (load_id);

CREATE TABLE promo (
    promo_id              TEXT NOT NULL,
    product_id            TEXT NOT NULL REFERENCES products(product_id),
    location_id           TEXT NULL REFERENCES location(location_id),
    type                  TEXT NOT NULL CHECK (type IN ('discount','bundle','loyalty_bonus','markdown','gift')),
    discount_pct          NUMERIC(5,2) NULL,
    promo_price_with_vat  NUMERIC(12,4) NULL,
    date_from             DATE NOT NULL,
    date_to               DATE NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id               UUID NOT NULL REFERENCES loads(id),
    PRIMARY KEY (promo_id, product_id, COALESCE(location_id, 'GLOBAL'))
);
CREATE INDEX promo_load_id_idx     ON promo (load_id);
CREATE INDEX promo_date_range_idx  ON promo (date_from, date_to);

CREATE TABLE order_rule (
    rule_id            TEXT PRIMARY KEY,
    scope              TEXT NOT NULL CHECK (scope IN ('product','category','supplier','global')),
    scope_ref          TEXT NULL,
    location_id        TEXT NULL REFERENCES location(location_id),
    safety_stock_days  NUMERIC(6,2) NULL,
    service_level_pct  NUMERIC(5,2) NULL,
    override_moq       INTEGER NULL,
    effective_from     TIMESTAMPTZ NOT NULL,
    effective_to       TIMESTAMPTZ NULL,
    load_id            UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX order_rule_scope_idx   ON order_rule (scope, scope_ref);
CREATE INDEX order_rule_load_id_idx ON order_rule (load_id);

CREATE TABLE supply_plan (
    plan_id        TEXT PRIMARY KEY,
    supplier_id    TEXT NOT NULL REFERENCES supplier(supplier_id),
    location_id    TEXT NOT NULL REFERENCES location(location_id),
    planned_date   DATE NOT NULL,
    slot_time      TEXT NULL,
    cutoff_at      TIMESTAMPTZ NULL,
    status         TEXT NOT NULL DEFAULT 'planned',
    load_id        UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX supply_plan_load_id_idx        ON supply_plan (load_id);
CREATE INDEX supply_plan_planned_date_idx   ON supply_plan (planned_date);
```

### products_upsert.sql

```sql
-- queries/products_upsert.sql
-- Используется через pgx CopyFrom либо batched INSERT. Здесь — bulk batched UPSERT.
INSERT INTO products (
    product_id, name, brand, manufacturer, category_id, category_path,
    weight_kg, pallet_qty, shelf_life_days, storage_temp_min, storage_temp_max,
    requires_prescription, is_dangerous_goods, status,
    created_at, updated_at, load_id
)
SELECT
    u.product_id, u.name, u.brand, u.manufacturer, u.category_id, u.category_path,
    u.weight_kg, u.pallet_qty, u.shelf_life_days, u.storage_temp_min, u.storage_temp_max,
    u.requires_prescription, u.is_dangerous_goods, u.status,
    now(), now(), $1
FROM unnest(
    $2::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[],
    $8::numeric[], $9::int[], $10::int[], $11::numeric[], $12::numeric[],
    $13::bool[], $14::bool[], $15::text[]
) AS u(
    product_id, name, brand, manufacturer, category_id, category_path,
    weight_kg, pallet_qty, shelf_life_days, storage_temp_min, storage_temp_max,
    requires_prescription, is_dangerous_goods, status
)
ON CONFLICT (product_id) DO UPDATE SET
    name                    = EXCLUDED.name,
    brand                   = EXCLUDED.brand,
    manufacturer            = EXCLUDED.manufacturer,
    category_id             = EXCLUDED.category_id,
    category_path           = EXCLUDED.category_path,
    weight_kg               = EXCLUDED.weight_kg,
    pallet_qty              = EXCLUDED.pallet_qty,
    shelf_life_days         = EXCLUDED.shelf_life_days,
    storage_temp_min        = EXCLUDED.storage_temp_min,
    storage_temp_max        = EXCLUDED.storage_temp_max,
    requires_prescription   = EXCLUDED.requires_prescription,
    is_dangerous_goods      = EXCLUDED.is_dangerous_goods,
    status                  = EXCLUDED.status,
    updated_at              = now(),
    load_id                 = $1
WHERE
    products.name             IS DISTINCT FROM EXCLUDED.name
 OR products.brand            IS DISTINCT FROM EXCLUDED.brand
 OR products.manufacturer     IS DISTINCT FROM EXCLUDED.manufacturer
 OR products.category_id      IS DISTINCT FROM EXCLUDED.category_id
 OR products.category_path    IS DISTINCT FROM EXCLUDED.category_path
 OR products.weight_kg        IS DISTINCT FROM EXCLUDED.weight_kg
 OR products.status           IS DISTINCT FROM EXCLUDED.status;
```

### products_select.sql

```sql
-- queries/products_select.sql
SELECT product_id, name, brand, manufacturer, category_id, category_path,
       weight_kg, pallet_qty, shelf_life_days, storage_temp_min, storage_temp_max,
       requires_prescription, is_dangerous_goods, status,
       created_at, updated_at, load_id
  FROM products
 WHERE load_id     = $1
   AND updated_at >= $2
   AND product_id  > $3
 ORDER BY product_id
 LIMIT $4;
```

## 3. Migration 0003 — store_assortment + lifecycle

```sql
-- 0003_init_store_assortment.up.sql

CREATE TABLE store_assortment (
    location_id        TEXT NOT NULL REFERENCES location(location_id),
    product_id         TEXT NOT NULL REFERENCES products(product_id),
    lifecycle_state    TEXT NOT NULL CHECK (lifecycle_state IN ('active','phasing_in','phasing_out','inactive')),
    assortment_class   TEXT NULL,
    price_min          NUMERIC(12,4) NULL,
    price_max          NUMERIC(12,4) NULL,
    effective_from     TIMESTAMPTZ NOT NULL,
    effective_to       TIMESTAMPTZ NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id            UUID NOT NULL REFERENCES loads(id),
    PRIMARY KEY (location_id, product_id, effective_from)
);
CREATE INDEX store_assortment_load_id_idx        ON store_assortment (load_id);
CREATE INDEX store_assortment_lifecycle_idx      ON store_assortment (lifecycle_state);
CREATE INDEX store_assortment_location_id_idx    ON store_assortment (location_id);

CREATE TABLE store_assortment_lifecycle_events (
    event_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    location_id      TEXT NOT NULL REFERENCES location(location_id),
    product_id       TEXT NOT NULL REFERENCES products(product_id),
    transition_type  TEXT NOT NULL,
    from_state       TEXT NULL,
    to_state         TEXT NOT NULL,
    transition_at    TIMESTAMPTZ NOT NULL,
    reason           TEXT NULL,
    evidence_path    TEXT NULL,
    load_id          UUID NOT NULL REFERENCES loads(id)
);
CREATE INDEX assortment_lifecycle_load_id_idx     ON store_assortment_lifecycle_events (load_id);
CREATE INDEX assortment_lifecycle_transition_idx  ON store_assortment_lifecycle_events (transition_at);
```

## 4. Migration 0004 — partitioned facts

```sql
-- 0004_init_facts_partitioned.up.sql

-- ============== receipt_line ==============
CREATE TABLE receipt_line (
    receipt_id        TEXT NOT NULL,
    line_no           INTEGER NOT NULL,
    location_id       TEXT NOT NULL,
    product_id        TEXT NOT NULL,
    barcode_scanned   TEXT NULL,
    qty               NUMERIC(14,4) NOT NULL,
    line_kind         TEXT NOT NULL CHECK (line_kind IN ('sale','refund','gift','promo_bonus')),
    unit_price_base   NUMERIC(12,4) NOT NULL,
    unit_price_paid   NUMERIC(12,4) NOT NULL,
    discount_amount   NUMERIC(12,4) NOT NULL DEFAULT 0,
    markdown_pct      NUMERIC(5,2) NULL,
    promo_id          TEXT NULL,
    event_date        DATE NOT NULL,
    event_time        TIMESTAMPTZ NOT NULL,
    loyalty_hash      TEXT NULL,
    valid_from        TIMESTAMPTZ NOT NULL,
    valid_to          TIMESTAMPTZ NOT NULL DEFAULT 'infinity',
    system_time_from  TIMESTAMPTZ NOT NULL DEFAULT now(),
    system_time_to    TIMESTAMPTZ NOT NULL DEFAULT 'infinity',
    load_id           UUID NOT NULL REFERENCES loads(id),
    PRIMARY KEY (event_date, receipt_id, line_no, system_time_from)
) PARTITION BY RANGE (event_date);

CREATE INDEX receipt_line_load_id_idx        ON receipt_line (load_id);
CREATE INDEX receipt_line_location_id_idx    ON receipt_line (location_id);
CREATE INDEX receipt_line_product_id_idx     ON receipt_line (product_id);

-- ============== location_stock_snapshot ==============
CREATE TABLE location_stock_snapshot (
    location_id        TEXT NOT NULL,
    product_id         TEXT NOT NULL,
    qty_on_hand        NUMERIC(14,4) NOT NULL,
    qty_reserved       NUMERIC(14,4) NOT NULL DEFAULT 0,
    qty_available      NUMERIC(14,4) NOT NULL,
    event_date         DATE NOT NULL,
    snapshot_at        TIMESTAMPTZ NOT NULL,
    system_time_from   TIMESTAMPTZ NOT NULL DEFAULT now(),
    system_time_to     TIMESTAMPTZ NOT NULL DEFAULT 'infinity',
    load_id            UUID NOT NULL REFERENCES loads(id),
    PRIMARY KEY (event_date, location_id, product_id, system_time_from)
) PARTITION BY RANGE (event_date);

CREATE INDEX location_stock_load_id_idx ON location_stock_snapshot (load_id);

-- ============== stock_movement ==============
CREATE TABLE stock_movement (
    movement_id        TEXT NOT NULL,
    type               TEXT NOT NULL CHECK (type IN ('receiving','transfer','write_off','return_to_vendor','damage','inventory_adj')),
    location_from      TEXT NULL,
    location_to        TEXT NULL,
    product_id         TEXT NOT NULL,
    qty                NUMERIC(14,4) NOT NULL,
    event_date         DATE NOT NULL,
    event_time         TIMESTAMPTZ NOT NULL,
    supplier_id        TEXT NULL,
    details            JSONB NULL,
    system_time_from   TIMESTAMPTZ NOT NULL DEFAULT now(),
    system_time_to     TIMESTAMPTZ NOT NULL DEFAULT 'infinity',
    load_id            UUID NOT NULL REFERENCES loads(id),
    PRIMARY KEY (event_date, movement_id, system_time_from)
) PARTITION BY RANGE (event_date);

CREATE INDEX stock_movement_load_id_idx     ON stock_movement (load_id);
CREATE INDEX stock_movement_product_id_idx  ON stock_movement (product_id);
CREATE INDEX stock_movement_type_idx        ON stock_movement (type);

-- ============== supplier_stock_snapshot ==============
CREATE TABLE supplier_stock_snapshot (
    supplier_id    TEXT NOT NULL,
    product_id     TEXT NOT NULL,
    qty_available  NUMERIC(14,4) NOT NULL,
    snapshot_at    TIMESTAMPTZ NOT NULL,
    event_date     DATE NOT NULL,
    load_id        UUID NOT NULL REFERENCES loads(id),
    PRIMARY KEY (event_date, supplier_id, product_id)
) PARTITION BY RANGE (event_date);

CREATE INDEX supplier_stock_load_id_idx ON supplier_stock_snapshot (load_id);
```

### Скрипт создания партиций (0099 / pg_partman)

```sql
-- queries/create_monthly_partition.sql
-- Параметры: $1 = parent table, $2 = month_start, $3 = month_end_exclusive
-- Имя партиции: <parent>_y<YYYY>m<MM>
DO $$
DECLARE
    parent_tbl text  := $1;
    p_start    date  := $2;
    p_end      date  := $3;
    p_name     text;
BEGIN
    p_name := format('%s_y%sm%s', parent_tbl, to_char(p_start, 'YYYY'), to_char(p_start, 'MM'));
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L);',
        p_name, parent_tbl, p_start, p_end
    );
END $$;
```

> **Recommendation:** в проде использовать `pg_partman` для авто-создания партиций. На MVP —
> ежедневный maintenance-job в самом адаптере, который создаёт партиции на следующие 3 месяца.

## 5. Migration 0005 — master_change_log

```sql
-- 0005_init_master_change_log.up.sql

CREATE TABLE master_change_log (
    event_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity      TEXT NOT NULL CHECK (entity IN ('products','product_barcodes')),
    entity_pk   JSONB NOT NULL,
    field       TEXT NOT NULL,
    old_value   JSONB NULL,
    new_value   JSONB NOT NULL,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    load_id     UUID NOT NULL REFERENCES loads(id)
);

CREATE INDEX mcl_entity_changed_idx ON master_change_log (entity, changed_at DESC);
CREATE INDEX mcl_field_idx          ON master_change_log (field);
CREATE INDEX mcl_load_id_idx        ON master_change_log (load_id);
CREATE INDEX mcl_entity_pk_gin      ON master_change_log USING GIN (entity_pk);
```

## 6. Migration 0006 — reject_log

```sql
-- 0006_init_reject_log.up.sql

CREATE TABLE reject_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    load_id     UUID NOT NULL REFERENCES loads(id),
    entity      TEXT NOT NULL,
    pk_value    JSONB NOT NULL,
    severity    TEXT NOT NULL CHECK (severity IN ('critical','soft')),
    reason      TEXT NOT NULL,
    raw         JSONB NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX reject_log_load_id_idx     ON reject_log (load_id);
CREATE INDEX reject_log_entity_idx      ON reject_log (entity);
CREATE INDEX reject_log_severity_idx    ON reject_log (severity);
CREATE INDEX reject_log_detected_idx    ON reject_log (detected_at DESC);
```

```sql
-- queries/reject_log_insert.sql
INSERT INTO reject_log (load_id, entity, pk_value, severity, reason, raw, detected_at)
VALUES ($1, $2, $3, $4, $5, $6, now());
```

## 7. Migration 0007 — audit_access

```sql
-- 0007_init_audit_access.up.sql

CREATE TABLE audit_access (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester    TEXT NOT NULL,
    endpoint     TEXT NOT NULL,
    method       TEXT NOT NULL,
    query        JSONB NULL,
    bytes_out    BIGINT NOT NULL DEFAULT 0,
    status_code  INTEGER NOT NULL,
    ts           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_access_ts_idx        ON audit_access (ts DESC);
CREATE INDEX audit_access_endpoint_idx  ON audit_access (endpoint, ts DESC);
CREATE INDEX audit_access_requester_idx ON audit_access (requester, ts DESC);
```

## 8. Migration 0008 — exports + entity_checkpoint

```sql
-- 0008_init_exports.up.sql

CREATE TABLE exports (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity       TEXT NOT NULL,
    snapshot_id  UUID NOT NULL REFERENCES loads(id),
    format       TEXT NOT NULL CHECK (format IN ('parquet','ndjson')),
    status       TEXT NOT NULL CHECK (status IN ('queued','running','ready','failed')),
    path         TEXT NULL,
    size_bytes   BIGINT NULL,
    error        TEXT NULL,
    requester    TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at   TIMESTAMPTZ NULL,
    finished_at  TIMESTAMPTZ NULL
);

CREATE INDEX exports_status_idx     ON exports (status, created_at);
CREATE INDEX exports_requester_idx  ON exports (requester, created_at DESC);

CREATE TABLE entity_checkpoint (
    load_id      UUID NOT NULL REFERENCES loads(id),
    entity       TEXT NOT NULL,
    cursor       JSONB NOT NULL,
    rows_done    BIGINT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (load_id, entity)
);
```

```sql
-- queries/exports_claim.sql (worker poll)
UPDATE exports
   SET status = 'running', started_at = now()
 WHERE id = (
    SELECT id FROM exports
     WHERE status = 'queued'
     ORDER BY created_at
     FOR UPDATE SKIP LOCKED
     LIMIT 1
 )
RETURNING *;
```

## 9. Cleanup / retention

```sql
-- queries/cleanup_orphan_load_rows.sql
-- Удаляет строки из *_master, привязанные к load_id, который НЕ committed
-- и не текущий runner. Запускается раз в сутки (maintenance job).
DELETE FROM products
 WHERE load_id IN (
   SELECT id FROM loads
    WHERE status = 'failed'
      AND finished_at < now() - INTERVAL '7 days'
 );
-- (повторяется для каждой master + facts таблицы)

-- queries/cleanup_old_audit_access.sql
DELETE FROM audit_access WHERE ts < now() - INTERVAL '90 days';

-- queries/cleanup_old_reject_log.sql
DELETE FROM reject_log WHERE detected_at < now() - INTERVAL '90 days';

-- queries/cleanup_old_exports.sql
-- + Локальный FS cleanup в коде по списку path-ов из этого DELETE RETURNING
DELETE FROM exports
 WHERE status IN ('ready','failed')
   AND created_at < now() - INTERVAL '24 hours'
RETURNING id, path;
```

## 10. Healthz

```sql
-- queries/healthz_ping.sql
SELECT 1;
```

## 11. Advisory lock helpers

В Go (`repository/snapshot.go`):

```go
func (r *Repo) TryAdvisoryLock(ctx context.Context, key string) (bool, func() error, error) {
    var ok bool
    err := r.pool.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, key).Scan(&ok)
    if err != nil { return false, nil, err }
    if !ok       { return false, func() error { return nil }, nil }
    release := func() error {
        _, err := r.pool.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, key)
        return err
    }
    return true, release, nil
}
```

Ключ для daily-load: `"source-adapter:daily-load"`.
