-- Module 7: Channel Routing schema (channel-routing).
-- Schema channels: supplier_channel_config + send_attempts (partitioned monthly).

CREATE SCHEMA IF NOT EXISTS channels;

-- Service role e_zoo_app: USAGE на схему. DML default privileges объявлены
-- в infra/pg/init/01_init.sh для роли-владельца.
GRANT USAGE ON SCHEMA channels TO e_zoo_app;

-- 1) supplier_channel_config — per-supplier channel configuration.
CREATE TABLE IF NOT EXISTS channels.supplier_channel_config (
    supplier_id           TEXT PRIMARY KEY,
    channel_type          TEXT NOT NULL CHECK (channel_type IN
        ('erp_api','edi_x12','edi_edifact','1c_xml','crm')),
    endpoint_url          TEXT NOT NULL,
    auth_mode             TEXT NOT NULL DEFAULT 'api_key' CHECK (auth_mode IN
        ('api_key','oauth2','mtls','none')),
    auth_credentials_ref  TEXT,
    timeout_sec           INT NOT NULL DEFAULT 30 CHECK (timeout_sec > 0),
    retry_max             INT NOT NULL DEFAULT 3 CHECK (retry_max >= 0),
    is_active             BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scc_active
    ON channels.supplier_channel_config(is_active) WHERE is_active = TRUE;

-- 2) send_attempts — audit log of channel send attempts, partitioned RANGE by started_at (monthly).
CREATE TABLE IF NOT EXISTS channels.send_attempts (
    id                UUID NOT NULL DEFAULT gen_random_uuid(),
    po_id             UUID NOT NULL,
    supplier_id       TEXT NOT NULL,
    channel_type      TEXT NOT NULL,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at       TIMESTAMPTZ,
    status            TEXT NOT NULL CHECK (status IN
        ('pending','success','failed','skipped')),
    http_status_code  INT,
    request_body      TEXT,
    response_body     TEXT,
    error_message     TEXT,
    retry_count       INT NOT NULL DEFAULT 0,
    external_ref      TEXT,
    PRIMARY KEY (id, started_at)
) PARTITION BY RANGE (started_at);

CREATE TABLE IF NOT EXISTS channels.send_attempts_2026_05 PARTITION OF channels.send_attempts
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS channels.send_attempts_2026_06 PARTITION OF channels.send_attempts
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS channels.send_attempts_2026_07 PARTITION OF channels.send_attempts
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE IF NOT EXISTS channels.send_attempts_2026_08 PARTITION OF channels.send_attempts
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

-- po_id index for fast retry lookup and idempotency check.
CREATE INDEX IF NOT EXISTS idx_send_attempts_po_id
    ON channels.send_attempts(po_id);

CREATE INDEX IF NOT EXISTS idx_send_attempts_supplier_started
    ON channels.send_attempts(supplier_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_send_attempts_status
    ON channels.send_attempts(status);

-- Idempotency: at most one successful attempt per po_id (partial unique index).
-- Note: PG partitioned tables don't support partial unique indexes across partitions
-- without including partition key. We rely on application-level lookup
-- (select_existing_success_attempt.sql) for idempotency.
