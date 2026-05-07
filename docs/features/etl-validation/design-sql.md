# Design SQL — etl-validation

> Все SQL — `go:embed` из `internal/features/etl_validation/sqls/`. Migrations нумеруются с **1001** (Модуль 1 занимает `0001..0099`). Owner — `golang-migrate`. Запуск — `make migrate-up-etl`.

---

## 1. Migration 1001 — schema marts + 5 mart-таблиц

### 1001_marts_schema.up.sql

```sql
-- 1) Schema
CREATE SCHEMA IF NOT EXISTS marts;

-- 2) Read-only role mart_reader (ADR-023)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mart_reader') THEN
        CREATE ROLE mart_reader NOLOGIN;
    END IF;
END$$;
GRANT USAGE ON SCHEMA marts TO mart_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA marts GRANT SELECT ON TABLES TO mart_reader;

-- 3) mart_demand_history (partitioned by as_of_date, monthly)
CREATE TABLE marts.mart_demand_history (
    as_of_date              DATE        NOT NULL,
    location_id             TEXT        NOT NULL,
    product_id              TEXT        NOT NULL,
    qty_sold                NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_returned            NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_promo_bonus         NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_gift                NUMERIC(18,4) NOT NULL DEFAULT 0,
    revenue_paid            NUMERIC(18,4) NOT NULL DEFAULT 0,
    discount_total          NUMERIC(18,4) NOT NULL DEFAULT 0,
    transactions_count      INTEGER     NOT NULL DEFAULT 0,
    had_promo               BOOLEAN     NOT NULL DEFAULT false,
    promo_type              TEXT,
    was_in_assortment       BOOLEAN     NOT NULL DEFAULT false,
    lifecycle_state_at_date TEXT,
    was_oos                 BOOLEAN     NOT NULL DEFAULT false,
    etl_run_id              UUID        NOT NULL,
    source_load_id          UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (product_id, location_id, as_of_date)
) PARTITION BY RANGE (as_of_date);

-- стартовые партиции (создаём 14 месяцев скользящим окном при инициализации)
CREATE TABLE marts.mart_demand_history_2026_01 PARTITION OF marts.mart_demand_history
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE marts.mart_demand_history_2026_02 PARTITION OF marts.mart_demand_history
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
-- ... остальные генерируются Job-ом 'mart_partition_maintenance' при старте приложения.

CREATE INDEX idx_mart_demand_history_etl_run ON marts.mart_demand_history (etl_run_id);
CREATE INDEX idx_mart_demand_history_source_load ON marts.mart_demand_history (source_load_id);

-- 4) mart_calculation_input (current snapshot)
CREATE TABLE marts.mart_calculation_input (
    product_id              TEXT        NOT NULL,
    location_id             TEXT        NOT NULL,
    on_hand                 NUMERIC(18,4) NOT NULL DEFAULT 0,
    in_transit              NUMERIC(18,4) NOT NULL DEFAULT 0,
    safety_stock            NUMERIC(18,4),
    forecast_horizon_days   INTEGER,
    daily_demand            NUMERIC(18,4),
    min_qty                 NUMERIC(18,4),
    max_qty                 NUMERIC(18,4),
    applicable_rule_id      TEXT,                       -- order_rule.id ИЛИ supply_spec.id
    applicable_rule_kind    TEXT NOT NULL,              -- 'order_rule' | 'supply_spec' | 'none'
    formula                 TEXT,                       -- reorder_point | min_max | econ_order_qty
    supplier_id             TEXT,
    lead_time_days          INTEGER,
    etl_run_id              UUID        NOT NULL,
    source_load_id          UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (product_id, location_id)
);

CREATE INDEX idx_mart_calc_input_etl_run ON marts.mart_calculation_input (etl_run_id);
CREATE INDEX idx_mart_calc_input_supplier ON marts.mart_calculation_input (supplier_id);

-- 5) mart_kpi_daily (partitioned)
CREATE TABLE marts.mart_kpi_daily (
    as_of_date         DATE        NOT NULL,
    location_id        TEXT        NOT NULL,
    kpi_name           TEXT        NOT NULL,             -- revenue_total, transactions, oos_pct, ...
    kpi_value          NUMERIC(18,6) NOT NULL,
    kpi_unit           TEXT,
    etl_run_id         UUID        NOT NULL,
    source_load_id     UUID        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (location_id, kpi_name, as_of_date)
) PARTITION BY RANGE (as_of_date);

CREATE TABLE marts.mart_kpi_daily_2026_01 PARTITION OF marts.mart_kpi_daily
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE marts.mart_kpi_daily_2026_02 PARTITION OF marts.mart_kpi_daily
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE INDEX idx_mart_kpi_daily_etl_run ON marts.mart_kpi_daily (etl_run_id);

-- 6) mart_master_current (current snapshot)
CREATE TABLE marts.mart_master_current (
    entity_type        TEXT        NOT NULL,            -- 'product' | 'location' | 'supplier' | ...
    entity_id          TEXT        NOT NULL,
    payload            JSONB       NOT NULL,            -- денормализованные поля
    etl_run_id         UUID        NOT NULL,
    source_load_id     UUID        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id)
);

CREATE INDEX idx_mart_master_current_etl_run ON marts.mart_master_current (etl_run_id);

-- 7) mart_supplier_scorecard (rolling weekly)
CREATE TABLE marts.mart_supplier_scorecard (
    supplier_id            TEXT        NOT NULL,
    week_start             DATE        NOT NULL,
    fill_rate_avg          NUMERIC(8,4),
    otif_pct               NUMERIC(8,4),
    lead_time_actual_avg   NUMERIC(10,2),
    qty_short_total        NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_damaged_total      NUMERIC(18,4) NOT NULL DEFAULT 0,
    qty_returned_total     NUMERIC(18,4) NOT NULL DEFAULT 0,
    lines_delivered        INTEGER     NOT NULL DEFAULT 0,
    lines_late             INTEGER     NOT NULL DEFAULT 0,
    etl_run_id             UUID        NOT NULL,
    source_load_id         UUID        NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (supplier_id, week_start)
);

CREATE INDEX idx_mart_supplier_scorecard_etl_run ON marts.mart_supplier_scorecard (etl_run_id);
```

### 1001_marts_schema.down.sql

```sql
DROP TABLE IF EXISTS marts.mart_supplier_scorecard;
DROP TABLE IF EXISTS marts.mart_master_current;
DROP TABLE IF EXISTS marts.mart_kpi_daily CASCADE;       -- + партиции
DROP TABLE IF EXISTS marts.mart_calculation_input;
DROP TABLE IF EXISTS marts.mart_demand_history CASCADE;  -- + партиции
DROP SCHEMA IF EXISTS marts CASCADE;
-- mart_reader role оставляем (может использоваться другими БД-объектами)
```

---

## 2. Migration 1002 — etl_runs + reject_log + audit_access

### 1002_etl_runs.up.sql

```sql
-- 1) etl_runs registry
CREATE TABLE marts.etl_runs (
    id                 UUID        PRIMARY KEY,
    started_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at        TIMESTAMPTZ,
    committed_at       TIMESTAMPTZ,
    status             TEXT        NOT NULL CHECK (status IN ('running','committed','failed','aborted')),
    kind               TEXT        NOT NULL DEFAULT 'full' CHECK (kind IN ('full','mart_refresh')),
    target_mart        TEXT,                                       -- если kind='mart_refresh'
    source_load_id     UUID,                                       -- NULL пока не зафиксирован
    parent_run_id      UUID        REFERENCES marts.etl_runs(id),  -- для retry
    trigger            TEXT        NOT NULL,                       -- 'cron'|'admin'|'retry'
    requester          TEXT,                                       -- JWT sub
    marts_summary      JSONB,                                      -- {mart_demand_history: {rows: 12345}, ...}
    failure_reason     TEXT,
    lines_total        BIGINT,
    lines_failed       BIGINT
);

CREATE INDEX idx_etl_runs_started_at ON marts.etl_runs (started_at DESC);
CREATE INDEX idx_etl_runs_status ON marts.etl_runs (status) WHERE status IN ('running','failed');
CREATE INDEX idx_etl_runs_source_load ON marts.etl_runs (source_load_id);

-- 2) reject_log
CREATE TABLE marts.reject_log (
    id                 BIGSERIAL   PRIMARY KEY,
    etl_run_id         UUID        NOT NULL REFERENCES marts.etl_runs(id),
    entity             TEXT        NOT NULL,
    business_key       TEXT,
    severity           TEXT        NOT NULL CHECK (severity IN ('critical','soft')),
    rule_id            TEXT        NOT NULL,
    field              TEXT,
    message            TEXT        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reject_log_run ON marts.reject_log (etl_run_id);
CREATE INDEX idx_reject_log_entity ON marts.reject_log (entity);
CREATE INDEX idx_reject_log_severity_run ON marts.reject_log (severity, etl_run_id);

-- 3) audit_access (для admin endpoint-ов)
CREATE TABLE marts.audit_access (
    id                 BIGSERIAL   PRIMARY KEY,
    occurred_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    method             TEXT        NOT NULL,
    path               TEXT        NOT NULL,
    requester          TEXT,                              -- JWT sub
    role               TEXT,
    status_code        INTEGER,
    request_id         TEXT
);

CREATE INDEX idx_audit_access_occurred ON marts.audit_access (occurred_at DESC);
CREATE INDEX idx_audit_access_requester ON marts.audit_access (requester);

-- 4) GRANT mart_reader на витрины + etl_runs (только select), НО НЕ на reject_log/audit_access
GRANT SELECT ON marts.mart_demand_history,
                 marts.mart_calculation_input,
                 marts.mart_kpi_daily,
                 marts.mart_master_current,
                 marts.mart_supplier_scorecard,
                 marts.etl_runs
TO mart_reader;
```

### 1002_etl_runs.down.sql

```sql
DROP TABLE IF EXISTS marts.audit_access;
DROP TABLE IF EXISTS marts.reject_log;
DROP TABLE IF EXISTS marts.etl_runs;
```

---

## 3. SQL queries

### 3.1. etl_runs_insert.sql

```sql
INSERT INTO marts.etl_runs (
    id, started_at, status, kind, target_mart,
    parent_run_id, trigger, requester, source_load_id
) VALUES (
    $1, now(), 'running', $2, $3,
    $4, $5, $6, $7
);
```

### 3.2. etl_runs_update_status.sql

```sql
UPDATE marts.etl_runs
SET    status         = $2,
       finished_at    = now(),
       committed_at   = CASE WHEN $2 = 'committed' THEN now() ELSE committed_at END,
       failure_reason = $3,
       lines_total    = COALESCE($4, lines_total),
       lines_failed   = COALESCE($5, lines_failed),
       marts_summary  = COALESCE($6, marts_summary)
WHERE  id = $1;
```

### 3.3. etl_runs_get_by_id.sql

```sql
SELECT id, started_at, finished_at, committed_at, status, kind, target_mart,
       source_load_id, parent_run_id, trigger, requester, marts_summary,
       failure_reason, lines_total, lines_failed
FROM   marts.etl_runs
WHERE  id = $1;
```

### 3.4. etl_runs_list.sql (cursor pagination)

```sql
SELECT id, started_at, finished_at, committed_at, status, kind, target_mart,
       source_load_id, trigger, lines_total, lines_failed
FROM   marts.etl_runs
WHERE  ($1::text   IS NULL OR status  = $1)
  AND  ($2::text   IS NULL OR kind    = $2)
  AND  ($3::tstz   IS NULL OR started_at < $3)             -- cursor
ORDER  BY started_at DESC
LIMIT  $4;
```

### 3.5. reject_log_insert.sql (batch via UNNEST)

```sql
INSERT INTO marts.reject_log (etl_run_id, entity, business_key, severity, rule_id, field, message)
SELECT $1, e, b, s, r, f, m
FROM   UNNEST($2::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[])
       AS t(e, b, s, r, f, m);
```

### 3.6. reject_log_select.sql

```sql
SELECT id, etl_run_id, entity, business_key, severity, rule_id, field, message, created_at
FROM   marts.reject_log
WHERE  ($1::uuid IS NULL OR etl_run_id = $1)
  AND  ($2::text IS NULL OR entity = $2)
  AND  ($3::text IS NULL OR severity = $3)
  AND  ($4::bigint IS NULL OR id < $4)                     -- cursor
ORDER  BY id DESC
LIMIT  $5;
```

### 3.7. mart_demand_history_insert.sql (агрегация)

```sql
INSERT INTO marts.mart_demand_history (
    as_of_date, location_id, product_id,
    qty_sold, qty_returned, qty_promo_bonus, qty_gift,
    revenue_paid, discount_total, transactions_count,
    had_promo, promo_type, was_in_assortment, lifecycle_state_at_date, was_oos,
    etl_run_id, source_load_id
)
SELECT
    rl.event_time::date                                                 AS as_of_date,
    rl.location_id,
    rl.product_id,
    SUM(CASE WHEN rl.line_kind='sale'        THEN rl.qty ELSE 0 END)    AS qty_sold,
    SUM(CASE WHEN rl.line_kind='return'      THEN rl.qty ELSE 0 END)    AS qty_returned,
    SUM(CASE WHEN rl.line_kind='promo_bonus' THEN rl.qty ELSE 0 END)    AS qty_promo_bonus,
    SUM(CASE WHEN rl.line_kind='gift'        THEN rl.qty ELSE 0 END)    AS qty_gift,
    SUM(CASE WHEN rl.line_kind='sale' THEN rl.qty * rl.unit_price_paid ELSE 0 END) AS revenue_paid,
    SUM(rl.discount_amount)                                             AS discount_total,
    COUNT(DISTINCT rl.receipt_id)                                       AS transactions_count,
    bool_or(p.id IS NOT NULL)                                           AS had_promo,
    MIN(p.type)                                                         AS promo_type,
    bool_or(sa.product_id IS NOT NULL)                                  AS was_in_assortment,
    MIN(sa.lifecycle_state)                                             AS lifecycle_state_at_date,
    bool_or(COALESCE(soh.qty_on_hand,0) = 0)                            AS was_oos,
    $1                                                                  AS etl_run_id,
    $2                                                                  AS source_load_id
FROM   pg_temp.stg_receipt_line rl
LEFT JOIN pg_temp.stg_promo p
       ON p.product_id = rl.product_id
      AND rl.event_time::date BETWEEN p.date_from AND p.date_to
LEFT JOIN pg_temp.stg_store_assortment sa
       ON sa.product_id = rl.product_id
      AND sa.location_id = rl.location_id
      AND rl.event_time::date BETWEEN sa.valid_from AND COALESCE(sa.valid_to, '9999-12-31')
LEFT JOIN pg_temp.stg_stock_on_hand soh
       ON soh.product_id  = rl.product_id
      AND soh.location_id = rl.location_id
      AND soh.as_of_date  = rl.event_time::date
GROUP  BY rl.event_time::date, rl.location_id, rl.product_id
ON CONFLICT (product_id, location_id, as_of_date) DO UPDATE
   SET qty_sold       = EXCLUDED.qty_sold,
       qty_returned   = EXCLUDED.qty_returned,
       revenue_paid   = EXCLUDED.revenue_paid,
       etl_run_id     = EXCLUDED.etl_run_id,
       source_load_id = EXCLUDED.source_load_id,
       created_at     = now();
```

### 3.8. mart_calculation_input_truncate_insert.sql (с приоритетом order_rule > supply_spec, ADR-024)

```sql
TRUNCATE TABLE marts.mart_calculation_input;

INSERT INTO marts.mart_calculation_input (
    product_id, location_id, on_hand, in_transit, safety_stock,
    forecast_horizon_days, daily_demand, min_qty, max_qty,
    applicable_rule_id, applicable_rule_kind, formula,
    supplier_id, lead_time_days,
    etl_run_id, source_load_id
)
WITH stock AS (
    SELECT product_id, location_id, qty_on_hand AS on_hand, qty_in_transit AS in_transit
    FROM   pg_temp.stg_stock_on_hand
),
rule_priority AS (
    -- сначала order_rule (приоритет), затем supply_spec
    SELECT product_id, location_id, id AS rule_id, 'order_rule'::text AS rule_kind,
           formula, min_qty, max_qty, safety_stock, forecast_horizon_days, daily_demand,
           supplier_id, lead_time_days,
           1 AS prio
    FROM   pg_temp.stg_order_rule
    UNION ALL
    SELECT product_id, location_id, id AS rule_id, 'supply_spec'::text,
           NULL, NULL, NULL, safety_stock, NULL, NULL,
           supplier_id, lead_time_days,
           2 AS prio
    FROM   pg_temp.stg_supply_spec
),
chosen AS (
    SELECT DISTINCT ON (product_id, location_id) *
    FROM   rule_priority
    ORDER  BY product_id, location_id, prio
)
SELECT s.product_id, s.location_id, s.on_hand, s.in_transit,
       c.safety_stock, c.forecast_horizon_days, c.daily_demand,
       c.min_qty, c.max_qty,
       c.rule_id, COALESCE(c.rule_kind, 'none'),
       c.formula, c.supplier_id, c.lead_time_days,
       $1, $2
FROM   stock s
LEFT JOIN chosen c USING (product_id, location_id);
```

### 3.9. mart_kpi_daily_insert.sql

```sql
INSERT INTO marts.mart_kpi_daily (
    as_of_date, location_id, kpi_name, kpi_value, kpi_unit,
    etl_run_id, source_load_id
)
SELECT as_of_date, location_id, kpi_name, kpi_value, kpi_unit, $1, $2
FROM (
    SELECT rl.event_time::date AS as_of_date,
           rl.location_id,
           'revenue_total'::text AS kpi_name,
           SUM(CASE WHEN rl.line_kind='sale' THEN rl.qty * rl.unit_price_paid ELSE 0 END) AS kpi_value,
           'EUR'::text AS kpi_unit
    FROM   pg_temp.stg_receipt_line rl
    GROUP  BY rl.event_time::date, rl.location_id
    UNION ALL
    SELECT rl.event_time::date,
           rl.location_id,
           'transactions',
           COUNT(DISTINCT rl.receipt_id)::numeric,
           'count'
    FROM   pg_temp.stg_receipt_line rl
    GROUP  BY rl.event_time::date, rl.location_id
    -- ... и т.д. для oos_pct, returns_count
) t
ON CONFLICT (location_id, kpi_name, as_of_date) DO UPDATE
   SET kpi_value      = EXCLUDED.kpi_value,
       etl_run_id     = EXCLUDED.etl_run_id,
       source_load_id = EXCLUDED.source_load_id,
       created_at     = now();
```

### 3.10. mart_master_current_truncate_insert.sql

```sql
TRUNCATE TABLE marts.mart_master_current;

INSERT INTO marts.mart_master_current (entity_type, entity_id, payload, etl_run_id, source_load_id)
SELECT 'product', id, to_jsonb(p) - 'id', $1, $2 FROM pg_temp.stg_products p
UNION ALL
SELECT 'location', id, to_jsonb(l) - 'id', $1, $2 FROM pg_temp.stg_locations l
UNION ALL
SELECT 'supplier', id, to_jsonb(s) - 'id', $1, $2 FROM pg_temp.stg_suppliers s;
```

### 3.11. mart_supplier_scorecard_insert.sql

```sql
INSERT INTO marts.mart_supplier_scorecard (
    supplier_id, week_start,
    fill_rate_avg, otif_pct, lead_time_actual_avg,
    qty_short_total, qty_damaged_total, qty_returned_total,
    lines_delivered, lines_late,
    etl_run_id, source_load_id
)
SELECT supplier_id,
       date_trunc('week', delivery_date)::date AS week_start,
       AVG(fill_rate)            AS fill_rate_avg,
       AVG(CASE WHEN otif THEN 1.0 ELSE 0.0 END) * 100 AS otif_pct,
       AVG(lead_time_actual)     AS lead_time_actual_avg,
       SUM(qty_short)            AS qty_short_total,
       SUM(qty_damaged)          AS qty_damaged_total,
       SUM(qty_returned)         AS qty_returned_total,
       COUNT(*)                  AS lines_delivered,
       SUM(CASE WHEN late THEN 1 ELSE 0 END) AS lines_late,
       $1, $2
FROM   pg_temp.stg_receiving_details
GROUP  BY supplier_id, date_trunc('week', delivery_date)
ON CONFLICT (supplier_id, week_start) DO UPDATE
   SET fill_rate_avg        = EXCLUDED.fill_rate_avg,
       otif_pct              = EXCLUDED.otif_pct,
       lead_time_actual_avg  = EXCLUDED.lead_time_actual_avg,
       qty_short_total       = EXCLUDED.qty_short_total,
       qty_damaged_total     = EXCLUDED.qty_damaged_total,
       qty_returned_total    = EXCLUDED.qty_returned_total,
       lines_delivered       = EXCLUDED.lines_delivered,
       lines_late            = EXCLUDED.lines_late,
       etl_run_id            = EXCLUDED.etl_run_id,
       source_load_id        = EXCLUDED.source_load_id,
       created_at            = now();
```

### 3.12. audit_access_insert.sql

```sql
INSERT INTO marts.audit_access (occurred_at, method, path, requester, role, status_code, request_id)
VALUES (now(), $1, $2, $3, $4, $5, $6);
```

---

## 4. Партиционирование и retention

- **Создание партиций (rolling 14 month window):** Job `mart_partition_maintenance` запускается при старте `etlapp` и ежедневно через scheduler. Создаёт next-month партиции на 14 месяцев вперёд.
- **Retention 365d (ADR-008):** тот же Job DROP'ает партиции старше `now() - 365d`.
- SQL helper: `pg_partman` НЕ используется (ADR-105 — partition_maintenance в Go-коде, чтобы не вводить новую зависимость).

---

## 5. Advisory lock

```sql
-- TryLock
SELECT pg_try_advisory_lock(hashtextextended($1, 0));
-- Unlock
SELECT pg_advisory_unlock(hashtextextended($1, 0));
```

Ключ — литерал `'etl-run'`. Параллельный ondemand `mart_supplier_scorecard` использует тот же ключ (Q-021 / E8: ondemand ждёт окончания daily run, либо отвечает 409).

---

## 6. Stale run detection (Q-025/ADR-025)

```sql
-- Найти зависшие running runs
SELECT id, started_at
FROM   marts.etl_runs
WHERE  status = 'running'
  AND  started_at < now() - $1::interval;
-- Помечаем как aborted
UPDATE marts.etl_runs
SET    status = 'aborted', finished_at = now(), failure_reason = 'stale_timeout'
WHERE  id = $1;
```

Запускается отдельным cron-job-ом при старте приложения, с интервалом `5m` и порогом `ETL_STALE_RUN_TIMEOUT` (default 1h).
