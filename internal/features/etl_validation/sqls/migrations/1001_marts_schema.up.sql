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
CREATE TABLE IF NOT EXISTS marts.mart_demand_history (
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

-- стартовые партиции на текущий + следующий месяц.
-- Остальные генерируются Job 'mart_partition_maintenance' при старте etlapp (фаза 13/14).
CREATE TABLE IF NOT EXISTS marts.mart_demand_history_2026_05 PARTITION OF marts.mart_demand_history
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS marts.mart_demand_history_2026_06 PARTITION OF marts.mart_demand_history
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX IF NOT EXISTS idx_mart_demand_history_etl_run
    ON marts.mart_demand_history (etl_run_id);
CREATE INDEX IF NOT EXISTS idx_mart_demand_history_source_load
    ON marts.mart_demand_history (source_load_id);

-- 4) mart_calculation_input (current snapshot)
CREATE TABLE IF NOT EXISTS marts.mart_calculation_input (
    product_id              TEXT        NOT NULL,
    location_id             TEXT        NOT NULL,
    on_hand                 NUMERIC(18,4) NOT NULL DEFAULT 0,
    in_transit              NUMERIC(18,4) NOT NULL DEFAULT 0,
    safety_stock            NUMERIC(18,4),
    forecast_horizon_days   INTEGER,
    daily_demand            NUMERIC(18,4),
    min_qty                 NUMERIC(18,4),
    max_qty                 NUMERIC(18,4),
    applicable_rule_id      TEXT,
    applicable_rule_kind    TEXT NOT NULL,
    formula                 TEXT,
    supplier_id             TEXT,
    lead_time_days          INTEGER,
    etl_run_id              UUID        NOT NULL,
    source_load_id          UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (product_id, location_id)
);

CREATE INDEX IF NOT EXISTS idx_mart_calc_input_etl_run
    ON marts.mart_calculation_input (etl_run_id);
CREATE INDEX IF NOT EXISTS idx_mart_calc_input_supplier
    ON marts.mart_calculation_input (supplier_id);

-- 5) mart_kpi_daily (partitioned)
CREATE TABLE IF NOT EXISTS marts.mart_kpi_daily (
    as_of_date         DATE        NOT NULL,
    location_id        TEXT        NOT NULL,
    kpi_name           TEXT        NOT NULL,
    kpi_value          NUMERIC(18,6) NOT NULL,
    kpi_unit           TEXT,
    etl_run_id         UUID        NOT NULL,
    source_load_id     UUID        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (location_id, kpi_name, as_of_date)
) PARTITION BY RANGE (as_of_date);

CREATE TABLE IF NOT EXISTS marts.mart_kpi_daily_2026_05 PARTITION OF marts.mart_kpi_daily
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS marts.mart_kpi_daily_2026_06 PARTITION OF marts.mart_kpi_daily
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX IF NOT EXISTS idx_mart_kpi_daily_etl_run
    ON marts.mart_kpi_daily (etl_run_id);

-- 6) mart_master_current (current snapshot)
CREATE TABLE IF NOT EXISTS marts.mart_master_current (
    entity_type        TEXT        NOT NULL,
    entity_id          TEXT        NOT NULL,
    payload            JSONB       NOT NULL,
    etl_run_id         UUID        NOT NULL,
    source_load_id     UUID        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_mart_master_current_etl_run
    ON marts.mart_master_current (etl_run_id);

-- 7) mart_supplier_scorecard (rolling weekly)
CREATE TABLE IF NOT EXISTS marts.mart_supplier_scorecard (
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

CREATE INDEX IF NOT EXISTS idx_mart_supplier_scorecard_etl_run
    ON marts.mart_supplier_scorecard (etl_run_id);
