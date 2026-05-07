# Research: channel-routing (Модуль 7)
**Дата:** 2026-05-07
**Mode:** compact

## Контекст
Из draft-plan: «Маршрутизация каналов EDI / 1С / ERP / CRM по описи поставщика. Этот модуль у нас как и первый. Состоит из двух частей: первая получает заказы, сформированные из нашей системы, вторая часть отправляет заказы по API в систему клиента. Это адаптер заказов в зависимости от того, куда мы должны их отправить.»

> Зеркало Модуля 1: SourceReader → 16 entity readers; здесь ChannelSender → 1+ channel-specific senders. MVP: REST API client ERP. Future: EDI/1С/CRM.

## Architecture
```
orders.purchase_orders (status='ready_to_send') → ChannelRouter → ChannelSender (ERP/EDI/1C/CRM) → external system
                                                                                                    ↓
                                                                     orders.purchase_orders (status='sent' / 'confirmed_by_erp' / 'failed_to_send')
                                                                     orders.send_attempts (audit log)
```

## Key abstraction (mirror Module 1's SourceReader)
```go
type ChannelSender interface {
    Send(ctx context.Context, po PurchaseOrder, channel ChannelConfig) (SendResult, error)
    Channel() string  // "erp_api" | "edi_x12" | "edi_edifact" | "1c_xml" | "crm"
}

type SendResult struct {
    ExternalRef    string    // ID/номер в external system
    Status         string    // "accepted" | "queued" | "rejected"
    SentAt         time.Time
    ResponseBody   []byte    // для audit
    ErrorMessage   *string
}
```

## MVP implementation
- `ErpAPIChannelSender` — POST /api/orders на client ERP, JSON body, JWT/OAuth2/API-key auth (configurable)
- Configurable timeout, retry с backoff cap (как HTTP клиент Модуля 1)
- Channel config per supplier: `supplier_channel_config(supplier_id, channel_type, endpoint_url, auth_mode, ...)`

## Tier: M-L
- Endpoints: 5
- Новых сущностей: 2 (send_attempts, supplier_channel_config)
- Миграции: 1 (5001_channels_schema)
- Внешние интеграции: HTTP клиент к external ERP/EDI/CRM
- Cron: 06:30 Europe/Kyiv (после order-builder 06:00)
- Diff: ~1500-2000 LOC

## Endpoints
- GET /v1/channels/send-attempts (list, filter by po_id/supplier_id/status/date)
- GET /v1/channels/send-attempts/:id (с request/response logs)
- POST /v1/channels/send (admin-cli, ondemand send всех ready_to_send)
- POST /v1/channels/send/:po_id/retry (admin-cli) — retry failed send для конкретного PO
- GET /v1/channels/configs (list channel configs)
- PUT /v1/channels/configs/:supplier_id (admin-cli) — update channel config

## Schema
```
channels.supplier_channel_config(supplier_id PK, channel_type, endpoint_url, auth_mode, auth_credentials_ref (vault/env name), timeout_sec, retry_max, created_at, updated_at)
channels.send_attempts(attempt_id, po_id refs orders.purchase_orders, channel_type, started_at, finished_at, status, http_status_code, request_body, response_body, error_message, retry_count, external_ref)
```

## Q-NNN (defaulted)
- Q-001: Trigger → cron 06:30 + ondemand admin endpoint + status-based polling
- Q-002: Auth modes → API-key (MVP), OAuth2 client_credentials (next iter), mTLS (next iter). Pluggable AuthProvider interface.
- Q-003: Retry policy → max 3 attempts с exponential backoff cap 30s (consistent с Модулем 1)
- Q-004: PO body format → JSON (MVP). Pluggable BodyFormatter для EDI X12/EDIFACT/1С XML.
- Q-005: Failure handling → status='failed_to_send', alert via Prometheus, ручной retry через admin endpoint
- Q-006: Idempotency → external_ref хранится в send_attempts; retry проверяет если PO уже принят (idempotency key = po_number)
- Q-007: Send-on-approval webhook от Модуля 6 → не в MVP, только cron + admin (упрощение)
- Q-008: Audit log → send_attempts таблица + audit_access middleware для admin endpoints
- Q-009: ChannelSender pluggability → готовится interface; MVP реализация только erp_api. EDI/1С/CRM — следующая итерация.
- Q-010: Sample order body → JSON структура согласована с client ERP (Q-NNN: точная схема — после ответа IT E-Zoo)

## Что уже есть (Модули 1-6)
- pgxpool, JWT, role middleware, scheduler pattern, advisory lock pattern
- HTTP client паттерн (extractor из Модуля 2 — переиспользуется как образец)
- orders.purchase_orders с workflow status
- pkg/errorspkg, mappers, audit middleware
- Prometheus metrics infrastructure

## Non-goals MVP
- EDI X12/EDIFACT — interface заложен, реализация позже
- 1С XML/SOAP — позже
- CRM — позже
- Multi-channel (один PO в несколько каналов одновременно)
- Async confirmation webhooks от ERP
- Schedule-based send (отправка в определённое время дня)
- Bulk batching POs одного supplier в один EDI message

## Связь с Q-013 source-adapter
В spec-interview/source-adapter был Q-013 «EDI-профиль» — отложен в Модуль 7. В этом MVP мы тоже его откладываем (interface готов, реализация после ответа клиента).
