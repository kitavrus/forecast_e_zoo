-- 1) etl_runs registry
CREATE TABLE IF NOT EXISTS marts.etl_runs (
    id                 UUID        PRIMARY KEY,
    started_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at        TIMESTAMPTZ,
    committed_at       TIMESTAMPTZ,
    status             TEXT        NOT NULL CHECK (status IN ('running','committed','failed','aborted')),
    kind               TEXT        NOT NULL DEFAULT 'full' CHECK (kind IN ('full','mart_refresh')),
    target_mart        TEXT,
    source_load_id     UUID,
    parent_run_id      UUID        REFERENCES marts.etl_runs(id),
    trigger            TEXT        NOT NULL CHECK (trigger IN ('cron','admin','retry')),
    requester          TEXT,
    marts_summary      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    failure_reason     TEXT,
    lines_total        BIGINT      NOT NULL DEFAULT 0,
    lines_failed       BIGINT      NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_etl_runs_started_at
    ON marts.etl_runs (started_at DESC);
CREATE INDEX IF NOT EXISTS idx_etl_runs_status
    ON marts.etl_runs (status) WHERE status IN ('running','failed');
CREATE INDEX IF NOT EXISTS idx_etl_runs_source_load
    ON marts.etl_runs (source_load_id);

-- 2) reject_log
CREATE TABLE IF NOT EXISTS marts.reject_log (
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

CREATE INDEX IF NOT EXISTS idx_reject_log_run
    ON marts.reject_log (etl_run_id);
CREATE INDEX IF NOT EXISTS idx_reject_log_entity
    ON marts.reject_log (entity);
CREATE INDEX IF NOT EXISTS idx_reject_log_severity_run
    ON marts.reject_log (severity, etl_run_id);

-- 3) audit_access (для admin endpoints)
CREATE TABLE IF NOT EXISTS marts.audit_access (
    id                 BIGSERIAL   PRIMARY KEY,
    occurred_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    method             TEXT        NOT NULL,
    path               TEXT        NOT NULL,
    requester          TEXT,
    role               TEXT,
    status_code        INTEGER,
    request_id         TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_access_occurred
    ON marts.audit_access (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_access_requester
    ON marts.audit_access (requester);

-- 4) GRANT mart_reader на витрины + etl_runs (только select).
-- ADR-023: НЕ выдаём grant на reject_log и audit_access (операционные данные).
GRANT SELECT ON marts.mart_demand_history,
                 marts.mart_calculation_input,
                 marts.mart_kpi_daily,
                 marts.mart_master_current,
                 marts.mart_supplier_scorecard,
                 marts.etl_runs
TO mart_reader;
