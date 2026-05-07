-- Module 5: Forecast Engine schema (forecast-engine).
-- Schema forecast: forecast_runs + forecasts (partitioned monthly) + calculation_lines + replenishment_plans.

CREATE SCHEMA IF NOT EXISTS forecast;

-- Service role e_zoo_app: USAGE на схему. DML default privileges объявлены
-- в infra/pg/init/01_init.sh для роли-владельца.
GRANT USAGE ON SCHEMA forecast TO e_zoo_app;

-- 1) forecast_runs — registry прогон engine.
CREATE TABLE IF NOT EXISTS forecast.forecast_runs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at         TIMESTAMPTZ,
    status              TEXT NOT NULL CHECK (status IN ('running','committed','failed')),
    horizon_days        INTEGER NOT NULL DEFAULT 14,
    snapshot_etl_run_id UUID,
    forecasts_count     INTEGER NOT NULL DEFAULT 0,
    lines_count         INTEGER NOT NULL DEFAULT 0,
    plans_count         INTEGER NOT NULL DEFAULT 0,
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_forecast_runs_started
    ON forecast.forecast_runs(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_forecast_runs_status
    ON forecast.forecast_runs(status);

-- 2) forecasts — partitioned RANGE по forecast_date (monthly).
CREATE TABLE IF NOT EXISTS forecast.forecasts (
    run_id          UUID NOT NULL REFERENCES forecast.forecast_runs(id) ON DELETE CASCADE,
    product_id      TEXT NOT NULL,
    location_id     TEXT NOT NULL,
    forecast_date   DATE NOT NULL,
    forecast_qty    NUMERIC(18,4) NOT NULL,
    lower_bound     NUMERIC(18,4),
    upper_bound     NUMERIC(18,4),
    model_name      TEXT NOT NULL DEFAULT 'sma_seasonal',
    confidence      NUMERIC(5,4),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, product_id, location_id, forecast_date)
) PARTITION BY RANGE (forecast_date);

CREATE TABLE IF NOT EXISTS forecast.forecasts_2026_05 PARTITION OF forecast.forecasts
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS forecast.forecasts_2026_06 PARTITION OF forecast.forecasts
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS forecast.forecasts_2026_07 PARTITION OF forecast.forecasts
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE INDEX IF NOT EXISTS idx_forecasts_run
    ON forecast.forecasts(run_id);
CREATE INDEX IF NOT EXISTS idx_forecasts_pl_date
    ON forecast.forecasts(product_id, location_id, forecast_date);

-- 3) calculation_lines — рассчитанные точки заказа per (product, location).
CREATE TABLE IF NOT EXISTS forecast.calculation_lines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID NOT NULL REFERENCES forecast.forecast_runs(id) ON DELETE CASCADE,
    product_id      TEXT NOT NULL,
    location_id     TEXT NOT NULL,
    supplier_id     TEXT,
    current_stock   NUMERIC(18,4) NOT NULL,
    in_transit      NUMERIC(18,4) NOT NULL DEFAULT 0,
    daily_demand    NUMERIC(18,4) NOT NULL,
    lead_time_days  INTEGER NOT NULL DEFAULT 7,
    safety_stock    NUMERIC(18,4) NOT NULL,
    reorder_point   NUMERIC(18,4) NOT NULL,
    target_stock    NUMERIC(18,4) NOT NULL,
    reorder_qty     NUMERIC(18,4) NOT NULL,
    calculated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, product_id, location_id)
);

CREATE INDEX IF NOT EXISTS idx_calc_lines_run
    ON forecast.calculation_lines(run_id);
CREATE INDEX IF NOT EXISTS idx_calc_lines_supplier
    ON forecast.calculation_lines(supplier_id);

-- 4) replenishment_plans — агрегаты по поставщику от Constructor.
CREATE TABLE IF NOT EXISTS forecast.replenishment_plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID NOT NULL REFERENCES forecast.forecast_runs(id) ON DELETE CASCADE,
    supplier_id     TEXT NOT NULL,
    location_id     TEXT NOT NULL,
    plan_date       DATE NOT NULL,
    total_qty       NUMERIC(18,4) NOT NULL,
    lines_count     INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved','cancelled')),
    approved_at     TIMESTAMPTZ,
    approved_by     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_plans_run
    ON forecast.replenishment_plans(run_id);
CREATE INDEX IF NOT EXISTS idx_plans_supplier_date
    ON forecast.replenishment_plans(supplier_id, plan_date);
CREATE INDEX IF NOT EXISTS idx_plans_status
    ON forecast.replenishment_plans(status);
