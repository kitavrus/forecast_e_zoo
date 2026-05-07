# Pipeline Report: channel-routing (Модуль 7)
**Дата:** 2026-05-07
**Tier:** M (compact)
**Status:** Финальный модуль MVP

## Что сделано

Реализован channel-routing: cron 06:30 Europe/Kyiv → подбирает ready_to_send POs → resolve channel config per supplier → ChannelSender (MVP: ErpAPISender) → пишет send_attempt → обновляет PO status. 6 endpoints. Pluggable interfaces для будущих EDI/1С/CRM.

## Endpoints

| Method | Path | Auth | Описание |
|---|---|---|---|
| GET | /v1/channels/send-attempts | admin/etl/it-read | List + filter + cursor |
| GET | /v1/channels/send-attempts/:id | admin/etl/it-read | Single attempt + logs |
| GET | /v1/channels/configs | admin/etl/it-read | List channel configs |
| POST | /v1/channels/send | admin-cli | Ondemand send всех ready_to_send (202 + run_id) |
| POST | /v1/channels/send/:po_id/retry | admin-cli | Idempotent retry |
| PUT | /v1/channels/configs/:supplier_id | admin-cli | Update channel config |

## Pluggable architecture (mirror Module 1)

- **ChannelSender interface** — MVP: ErpAPISender (HTTP retry/backoff cap 30s, 4xx no-retry, 5xx/429 retry). Заглушки: edi_x12, edi_edifact, 1c_xml, crm.
- **AuthProvider interface** — MVP: APIKeyProvider. Заглушки: oauth2 client_credentials, mTLS.
- **BodyFormatter interface** — MVP: JSONFormatter. Заглушки: EDI X12, EDIFACT, 1C XML.

## Метрики прогона
- 5 git коммитов в этом прогоне (4-8 фазы) + 3 предыдущих (1-3)
- 1 миграция (5001_channels_schema): supplier_channel_config + send_attempts (partitioned)
- 5 sentinel + supportMessage коды
- 4 Prometheus метрики (channel_send_total, channel_send_duration_seconds, channel_retry_count_total, channel_router_run_total)

## Quality gates
- `go build ./...` — OK
- `go vet ./...` — OK
- `go test -race ./...` — OK (все пакеты включая 6 новых: auth, formatter, sender, router_svc, scheduler, service, validators)

## Sensitivity / Security
- `auth_credentials_ref` в `supplier_channel_config` — только имя env var / vault path. Реальные креды НЕ хранятся в БД.
- Idempotency: через `external_ref` (если PO уже принят ERP — retry возвращает existing attempt).
- Per-PO транзакция: SELECT FOR UPDATE → InsertAttempt → Send → FinishAttempt → MarkPOSent → commit.

## Известные ограничения (MVP)

- Только `erp_api` channel реализован. EDI/1С/CRM — interface готов, реализация позже.
- Только `api_key` auth. OAuth2/mTLS — interface готов.
- Без async confirmation webhooks от ERP (status='confirmed_by_erp' остаётся ручным).
- Без bulk batching POs одного supplier в один EDI message.
- Sample order body согласован с client ERP условно — конкретная схема будет финализирована после ответа IT E-Zoo (Q-NNN отложен).

## Артефакты
- design.md, spec-interview, code-plan + status, 8 phase-файлов
- internal/features/channels/* — реализация
- pkg/errorspkg/errors_channels.go — 5 sentinel
- channels.* schema (supplier_channel_config + send_attempts)

## Связь с Q-013 source-adapter
Q-013 «EDI-профиль» (отложен из spec-interview/source-adapter в Module 7) — текущий MVP реализует interface, но не EDI implementation. Финальный EDI-профиль закрывается в next iter после ответа IT E-Zoo.
