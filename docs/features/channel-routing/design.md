# Design: channel-routing (Module 7) — compact M-L

**Дата:** 2026-05-07
**Module path:** `github.com/Kitavrus/e_zoo/internal/features/channels`
**Mode:** compact

## 1. Обзор и архитектура

Финальный модуль MVP-пайплайна. Зеркало Module 1 (SourceReader → entity readers): здесь `ChannelSender` interface → channel-specific implementations. MVP содержит `ErpAPIChannelSender` (HTTP REST + JSON, API-key auth).

```
+-----------------------------+
| orders.purchase_orders      |
| status='ready_to_send'      |
+--------------+--------------+
               |
               v   FOR UPDATE SKIP LOCKED
+-----------------------------+      +-------------------------+
| channels.ChannelRouter      | ---> | supplier_channel_config |
| (orchestrator)              | <--- | (per supplier_id)       |
+--------------+--------------+      +-------------------------+
               |
               v
+-----------------------------+
| ChannelSender interface     |  -- erp_api  --> ErpAPIChannelSender (MVP)
|                             |  -- edi_x12  --> (future)
|                             |  -- 1c_xml   --> (future)
|                             |  -- crm      --> (future)
+--------------+--------------+
               |
               v
+-----------------------------+      +-------------------------+
| External ERP / EDI / CRM    |      | channels.send_attempts  |
+-----------------------------+      | (audit log)             |
                                     +-------------------------+
```

**Слои:** handler → service → routing.ChannelRouter → sender.ChannelSender → external HTTP/EDI.
**Запись результата:** repository пишет send_attempts + обновляет purchase_orders.status в той же tx.

## 2. Endpoints (5 шт)

| Method | Path | Roles | DTO |
|---|---|---|---|
| GET | `/v1/channels/send-attempts` | ITRead/AdminCLI/XFlowETL | ListSendAttemptsResponse (filter: po_id, supplier_id, status, from, to, cursor, limit) |
| GET | `/v1/channels/send-attempts/:id` | ITRead/AdminCLI/XFlowETL | SendAttemptDetailResponse (включая request/response bodies) |
| POST | `/v1/channels/send` | AdminCLI | TriggerSendResponse `{run_id, started}` (202 при success, 409 если lock busy) |
| POST | `/v1/channels/send/:po_id/retry` | AdminCLI | RetryResponse `{attempt_id, status, external_ref}` |
| GET | `/v1/channels/configs` | ITRead/AdminCLI | ListChannelConfigsResponse |
| PUT | `/v1/channels/configs/:supplier_id` | AdminCLI | ChannelConfigItem (upsert) |

> Audit middleware (`auditWriter.Middleware()`) — на все mutating endpoints.

## 3. SQL и миграция 5001

### Migration `5001_channels_schema.up.sql`

```sql
CREATE SCHEMA IF NOT EXISTS channels;

-- 1) supplier_channel_config — per-supplier настройка канала.
CREATE TABLE IF NOT EXISTS channels.supplier_channel_config (
    supplier_id           TEXT PRIMARY KEY,
    channel_type          TEXT NOT NULL CHECK (channel_type IN
        ('erp_api','edi_x12','edi_edifact','1c_xml','crm')),
    endpoint_url          TEXT NOT NULL,
    auth_mode             TEXT NOT NULL DEFAULT 'api_key' CHECK (auth_mode IN
        ('api_key','oauth2','mtls','none')),
    auth_credentials_ref  TEXT,                       -- env var NAME (e.g. CHANNEL_AUTH_ERP_API)
    timeout_sec           INT NOT NULL DEFAULT 30,
    retry_max             INT NOT NULL DEFAULT 3,
    is_active             BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2) send_attempts — audit log, partitioned monthly.
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

CREATE INDEX IF NOT EXISTS idx_send_attempts_po_id
    ON channels.send_attempts(po_id);
CREATE INDEX IF NOT EXISTS idx_send_attempts_supplier_started
    ON channels.send_attempts(supplier_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_send_attempts_status
    ON channels.send_attempts(status);
```

### Queries (key)

- `select_ready_to_send_pos.sql` — SELECT FOR UPDATE SKIP LOCKED `ready_to_send` PO с лимитом.
- `select_supplier_channel_config.sql` — by supplier_id (only is_active=true).
- `upsert_supplier_channel_config.sql` — ON CONFLICT (supplier_id) DO UPDATE.
- `insert_send_attempt.sql` — RETURNING id, started_at.
- `update_send_attempt_finish.sql` — finished_at, status, http_status_code, response_body, error_message, external_ref.
- `update_po_to_sent.sql` — status='sent', sent_at, sent_to_channel.
- `select_existing_success_attempt.sql` — для idempotency (по po_id, status='success').
- `select_send_attempts.sql` — list with filters/cursor.
- `select_send_attempt_by_id.sql` — by attempt_id.
- `list_channel_configs.sql` — ORDER BY supplier_id.

## 4. ChannelSender interface + ErpAPIChannelSender

```go
package sender

type SendInput struct {
    PO          PurchaseOrderForSend
    ChannelType string
    EndpointURL string
    AuthMode    string
    AuthSecret  string  // resolved from env (NEVER persisted)
    TimeoutSec  int
    RetryMax    int
}

type SendResult struct {
    Status         string // "success"|"failed"|"skipped"
    HTTPStatusCode *int
    ExternalRef    string
    RequestBody    string
    ResponseBody   string
    ErrorMessage   string
    RetryCount     int
}

type ChannelSender interface {
    Send(ctx context.Context, in SendInput) (SendResult, error)
    Channel() string  // "erp_api" | "edi_x12" | ...
}
```

**ErpAPIChannelSender (MVP):**
- POST `{endpoint_url}/api/orders` Content-Type: application/json
- Auth: `Authorization: ApiKey {auth_secret}` (API-key mode) или OAuth2 (next).
- Retry: max 3 attempts с exp backoff cap 30s + jitter (как extractor.client).
- Idempotency-key: `po_number` в заголовке `X-Idempotency-Key`.
- Парсинг ответа: 2xx → success, external_ref из body; 4xx (кроме 429) → failed (no retry); 5xx/429 → retry.
- ErrChannelUnavailable если retry exhausted.

**Pluggability:** registry `map[string]ChannelSender`. MVP регистрирует только `erp_api`. Future: добавить `edi_x12_sender.go`, `onec_xml_sender.go` без изменения routing.

## 5. Errors (новые в `pkg/errorspkg/errors_channels.go`)

```go
ErrSendAttemptNotFound  // 404 CR-001
ErrChannelNotConfigured // 409 CR-002
ErrPONotReadyToSend     // 409 CR-003
ErrChannelUnavailable   // 503 CR-004
ErrChannelRoutingInProgress // 409 CR-005 (advisory lock busy)
ErrChannelRoutingUnavailable // 503 CR-006 (scheduler not configured)
ErrInvalidChannelType   // 400 CR-007
ErrInvalidAuthMode      // 400 CR-008
```

Поддерживающие `SupportMessageCodes`: `CR-001..CR-008`.

## 6. Tests

**Unit (sender):**
- `erp_api_sender_test.go` — happy path 200, 4xx no retry, 5xx retry exhausted, idempotency-key header.
- `mock_sender.go` — для тестов routing/scheduler.

**Integration (repository):**
- `channels_integration_test.go` — image `postgres:18-alpine`, миграции `4001` + `5001`, scenarios: insert+update send_attempt, upsert config, list with filters, FOR UPDATE SKIP LOCKED конкуренция.

**Service:**
- `channel_router_test.go` — mock repo + mock sender, проверка status transitions, idempotency lookup.

## 7. ADR (10 шт)

1. **ADR-001: ChannelSender interface — single Send method.** Trade-off: vs per-channel constructors. Driver: pluggability для EDI/1С/CRM future.
2. **ADR-002: Retry max 3 + exp backoff cap 30s.** Trade-off: vs unlimited / aggressive. Driver: согласование с extractor.client (Module 2).
3. **ADR-003: API-key auth for MVP, OAuth2/mTLS deferred.** Trade-off: simplicity now. Driver: ответ IT клиента ещё не получен.
4. **ADR-004: Idempotency через X-Idempotency-Key=po_number + lookup existing successful attempt.** Trade-off: vs server-side dedup. Driver: спецификация ERP.
5. **ADR-005: send_attempts partitioned by started_at monthly + retention 90d.** Trade-off: vs flat table. Driver: audit volume + KPI compute alignment.
6. **ADR-006: auth_credentials_ref хранит ENV var name, не сам секрет.** Trade-off: vs encrypted column. Driver: secrets out of DB (compliance).
7. **ADR-007: Cron 06:30 Europe/Kyiv после order-builder 06:00.** Trade-off: vs webhook on approve. Driver: simpler MVP, фиксирует SLA "next day delivery".
8. **ADR-008: SELECT FOR UPDATE SKIP LOCKED при выборке ready_to_send.** Trade-off: vs PG advisory lock per PO. Driver: параллелизм если в будущем несколько workers.
9. **ADR-009: ChannelRouter resolve channel config per-PO (not per-batch).** Trade-off: N+1 vs batch. Driver: малый объём (≤500 PO/run).
10. **ADR-010: Failed attempts оставляют PO в `ready_to_send` (не вводим `failed_to_send`).** Trade-off: vs новый статус. Driver: совместимость с CHECK constraint orders.purchase_orders без миграции.

## 8. Конфигурация

ENV vars (новые):
- `CHANNEL_ROUTING_CRON_SCHEDULE` (default `30 6 * * *`).
- `CHANNEL_ROUTING_CRON_TZ` (default `Europe/Kyiv`).
- `CHANNEL_ROUTING_MAX_POS` (default `500`).
- `CHANNEL_ROUTING_HTTP_TIMEOUT_SEC` (default `30`).
- `CHANNEL_AUTH_ERP_API` — common API key для erp_api channel (используется если auth_credentials_ref пустой).

## 9. Pluggability (future hooks)

```go
// ChannelSenderRegistry — мапа channel_type → ChannelSender.
type ChannelSenderRegistry struct {
    senders map[string]ChannelSender
}
func (r *ChannelSenderRegistry) Register(s ChannelSender) { r.senders[s.Channel()] = s }
func (r *ChannelSenderRegistry) Get(channel string) (ChannelSender, bool) { ... }
```

## 10. Hard invariants

- **Никогда** не пишем секреты в БД (ADR-006).
- **Idempotency** — обязательная проверка перед отправкой.
- **Tx isolation** — update PO + insert send_attempt + update send_attempt в одной транзакции.
- **Audit middleware** на admin endpoints.
- **Migration 5001** — additive, без backfill, rollback простой DROP SCHEMA.
