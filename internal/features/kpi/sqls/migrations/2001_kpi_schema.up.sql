-- Module 4: KPI Engine schema (kpi-calibration).
-- Schema kpi: snapshots (partitioned monthly) + hierarchical calibrations.

CREATE SCHEMA IF NOT EXISTS kpi;

-- Service role e_zoo_app: USAGE на схему. DML default privileges объявлены
-- в infra/pg/init/01_init.sh для роли-владельца.
GRANT USAGE ON SCHEMA kpi TO e_zoo_app;

-- 1) kpi_calibrations — иерархия (kpi_name, scope_type, scope_id) → JSON params.
--    scope_id NULL для scope_type='global'.
CREATE TABLE IF NOT EXISTS kpi.kpi_calibrations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kpi_name    TEXT NOT NULL,
    scope_type  TEXT NOT NULL CHECK (scope_type IN ('global','category','supplier','location','product_location')),
    scope_id    TEXT,
    params      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- COALESCE для NULL scope_id (global) гарантирует уникальность строки global per KPI.
CREATE UNIQUE INDEX IF NOT EXISTS uq_kpi_calibrations_scope
    ON kpi.kpi_calibrations (kpi_name, scope_type, COALESCE(scope_id, ''));

-- 2) kpi_snapshots — партиционирована RANGE по as_of_date месячно.
CREATE TABLE IF NOT EXISTS kpi.kpi_snapshots (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    as_of_date      DATE NOT NULL,
    kpi_name        TEXT NOT NULL,
    scope_type      TEXT NOT NULL CHECK (scope_type IN ('global','category','supplier','location','product_location')),
    scope_id        TEXT,
    value           NUMERIC(18,6) NOT NULL,
    calibration_id  UUID,
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    etl_run_id      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, as_of_date)
) PARTITION BY RANGE (as_of_date);

CREATE TABLE IF NOT EXISTS kpi.kpi_snapshots_2026_05 PARTITION OF kpi.kpi_snapshots
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS kpi.kpi_snapshots_2026_06 PARTITION OF kpi.kpi_snapshots
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS kpi.kpi_snapshots_2026_07 PARTITION OF kpi.kpi_snapshots
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE INDEX IF NOT EXISTS idx_kpi_snapshots_filter
    ON kpi.kpi_snapshots (as_of_date, kpi_name, scope_type);
CREATE INDEX IF NOT EXISTS idx_kpi_snapshots_etl_run
    ON kpi.kpi_snapshots (etl_run_id);

-- 3) Сидинг дефолтных global калибровок.
INSERT INTO kpi.kpi_calibrations (kpi_name, scope_type, scope_id, params) VALUES
    ('osa',         'global', NULL, '{"lookback_days":30,"min_observations":7}'::jsonb),
    ('otif',        'global', NULL, '{"late_grace_hours":0,"fill_rate_threshold":0.95}'::jsonb),
    ('stock_days',  'global', NULL, '{"include_in_transit":true,"min_daily_demand":0.001,"cap_days":365}'::jsonb)
ON CONFLICT DO NOTHING;
