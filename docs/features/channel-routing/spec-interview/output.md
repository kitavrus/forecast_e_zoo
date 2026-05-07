# Spec-Interview: channel-routing (defaulted)

**Дата:** 2026-05-07
**Tier:** M-L
**Mode:** compact / defaulted (без интерактивного интервью — решения зафиксированы из research)

## 1. Проблема и цель

Заказы (`orders.purchase_orders` со статусом `ready_to_send`), создаваемые Module 6 order-builder, должны автоматически отправляться в системы поставщиков (ERP/EDI/1С/CRM) через адаптер канала, специфичный для конкретного supplier'а. Модуль 7 — финальный этап MVP-пайплайна (zero-touch supply chain).

## 2. Сценарии

### Happy path
- Cron 06:30 Europe/Kyiv → ChannelRouter забирает все `ready_to_send` PO → для каждого PO резолвит channel config supplier'а → ChannelSender отправляет → при успехе PO.status='sent', sent_at, sent_to_channel заполнены, external_ref сохранён.

### On-demand
- Admin вызывает `POST /v1/channels/send` (всё что готово) или `POST /v1/channels/send/{po_id}/retry` (для конкретного failed PO).

### Edge cases
- PO в чужом статусе (не `ready_to_send`) → 409 ErrPONotReadyToSend.
- Нет channel config для supplier → marker error в send_attempts, PO остаётся `ready_to_send`, alert в Prometheus.
- Внешняя система down → retry 3 раза с exp backoff cap 30s; если все попытки fail → status='ready_to_send' остаётся, attempt записан как failed.
- Idempotency: если в send_attempts уже есть successful attempt с тем же po_id — возвращаем тот external_ref без повторного запроса.

## 3. Защищённые поля и enum'ы

- `channels.send_attempts.status` ∈ {`pending`, `success`, `failed`, `skipped`}
- `channels.supplier_channel_config.channel_type` ∈ {`erp_api`, `edi_x12`, `edi_edifact`, `1c_xml`, `crm`}
- `channels.supplier_channel_config.auth_mode` ∈ {`api_key`, `oauth2`, `mtls`, `none`}

## 4. Безопасность

- Read endpoints (GET): RoleITRead | RoleAdminCLI | RoleXFlowETL.
- Mutating (POST send/retry, PUT configs): RoleAdminCLI.
- Audit middleware на все /v1/channels/* admin (POST/PUT) endpoints.
- `auth_credentials_ref` хранит ИМЯ env-переменной (не сам секрет). Реальный credential приходит из process env во время send.

## 5. Ошибки

- 404 ErrSendAttemptNotFound — `GET /send-attempts/{id}` не найден.
- 409 ErrChannelNotConfigured — нет channel config для supplier_id.
- 409 ErrPONotReadyToSend — PO не в статусе `ready_to_send`.
- 503 ErrChannelUnavailable — внешняя система недоступна (после retry).

## 6. Принятые компромиссы

- MVP — только `ErpAPIChannelSender` (HTTP REST + JSON). EDI/1С/CRM — interface готов, реализация позже.
- Cron 06:30 + admin endpoints (без webhook от Module 6 на approval).
- Async confirmation от ERP — не в MVP.
- Retention `send_attempts` — 90 дней, partition по `started_at` RANGE month.
- `auth_credentials_ref` хранит env var name (никогда не сами секреты).

## 7. Открытые вопросы (deferred)

- Точная схема JSON body для ERP — после ответа IT клиента.
- Webhook callbacks из ERP — отдельный модуль.
- Schedule-based send (отправка в окно) — out of MVP.
