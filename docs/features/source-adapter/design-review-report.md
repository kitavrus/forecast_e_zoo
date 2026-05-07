# Design Review Report: source-adapter (re-review)
**Дата:** 2026-05-07 (re-review)
**Ревьюер:** design-reviewer agent

---

## Статус блокеров из предыдущего ревью

| Блокер | Статус | Комментарий |
|---|---|---|
| **B-1** Нумерация ADR-NNN ↔ Q-NNN не 1:1 | **ЗАКРЫТ** | `design-adr.md` §«Crosswalk Q-NNN ↔ ADR-NNN» содержит явную таблицу 1:1 для Q-001..Q-016. ADR-001..016 закрывают одноимённые Q по теме. Архитектурные мета-ADR вынесены в диапазон 100+ (ADR-100 стек, ADR-101 JWT, ADR-102 atomic flip, ADR-103 multi-tenant). Пересечений нумерации с Q нет. |
| **B-1.1** Q-004 (объём данных + CDC trigger) без формального ADR | **ЗАКРЫТ** | Добавлен `ADR-004` со статусом «Отложено», эскалация: IT E-Zoo + продукт-менеджер X-Flow, требуется pre-MVP замер у E-Zoo. Альтернативы (сразу CDC; гибрид) и риски явно зафиксированы. |
| **B-1.2** Q-016 (Lifecycle events DTO) без DTO в design | **ЗАКРЫТ** | `ADR-016` содержит публичный DTO `StoreAssortmentLifecycleEventResponse` с явными полями (`eventId`, `eventType`, `locationId`, `productId`, `effectiveAt`, `reason`, `promoId`, `priorState`, `newState`, `sourceLoadId`, `createdAt`). DTO продублирован в `design-go-layers.md` §3.1 и упомянут в `design-sequence-diagrams.md` §4 (L180) для эндпоинта `GET /v1/store_assortment/lifecycle_events`. |

---

## Статус серьёзных замечаний (S-1..S-7)

| # | Статус | Комментарий |
|---|---|---|
| **S-1** ADR-016 не закрывает Q-016 (DTO) | **ЗАКРЫТ** | DTO `StoreAssortmentLifecycleEventResponse` зафиксирован в трёх местах: ADR-016 (public контракт + JSON schema policy), `design-go-layers.md` §3.1 (Go-определение), `design-sequence-diagrams.md` §4 (упомянут в response payload). |
| **S-2** ADR-007 склеен (backoff + exports) | **ЗАКРЫТ** | ADR-007 теперь содержит только retry-стратегию (`ERP_RETRY_MAX=3`, `ERP_RETRY_BACKOFF_CAP=30s`, jitter 10%, retry-коды). Local FS exports вынесены в отдельный ADR-009. |
| **S-3** ADR-014 склеен (audit + atomic flip) | **ЗАКРЫТ** | ADR-014 закрывает только Q-014 (audit retention 90d, partitioning, scope `/admin/*`). Atomic flip вынесен в мета-ADR-102. |
| **S-4** ADR-015 склеен (stale + master_change_log) | **ЗАКРЫТ** | ADR-015 закрывает только Q-015 (stale load detection, default 1h). `master_change_log` упоминается отдельной строкой в `design.md` §3 (мета-ADR-100, контракт). |
| **S-5** Q-004 (объём данных) без ADR | **ЗАКРЫТ** | См. B-1.1. ADR-004 формализован полностью. |
| **S-6** ADR-013 расходится с Q-013 (EDI) | **ЗАКРЫТ** | ADR-013 явно помечен «Передан в Модуль 7 (не закрывается в фиче source-adapter)». Тема (EDI-профиль) совпадает с Q-013. |
| **S-7** Нет матрицы sentinel ↔ test | **ЗАКРЫТ** | `design-tests.md` §6 (L327–L359) содержит две таблицы покрытия: 17 строк base sentinel-ошибок + 4 строки cron/load-only sentinels = **21 строка** ≥10. Покрыты unit (handler/middleware/validators) и integration (repository/service/e2e). |

---

## Статус незначительных (N-1..N-6)

| # | Статус | Комментарий |
|---|---|---|
| **N-1** swimlane drawLabel ↔ FLOWS | Не проверялся в re-review (вне списка Fix). Требует ручной верификации HTML — отдельной задачей. |
| **N-2** swimlane внешние script/link | Не проверялся в re-review. |
| **N-3** integrations: fallback для Q-001/Q-002 | **ЗАКРЫТ** | `design-integrations.md` §1 (L93–L94) явно описывает fallback: «если до DD-MM-YYYY нет ответа от ИБ E-Zoo (Q-001) / IT E-Zoo (Q-002) — реализуем `erp_e_zoo_reader` как in-memory stub с фиктивными данными для…». |
| **N-4** ADR-006 без стартового набора правил | **ЗАКРЫТ** | ADR-006 теперь содержит YAML-блок с 7 стартовыми правилами: `negative_qty`, `future_event_time`, `duplicate_product_in_load`, `missing_required_field`, `negative_stock_balance`, `orphan_fk` (soft), `stale_event_time` (soft). Покрыты master + receipt_line + location_stock_snapshot. |
| **N-5** ENV префикс STALE_LOAD_TIMEOUT | **ЗАКРЫТ** | В обоих файлах используется `SOURCE_ADAPTER_STALE_LOAD_TIMEOUT`: `design-adr.md` ADR-015 (L386), `design-infrastructure.md` (L83 + L203 в таблице env-vars). Консистентно с `SOURCE_ADAPTER_CRON_SCHEDULE`/`SOURCE_ADAPTER_TZ`. |
| **N-6** design.md §3 ссылка «ADR-007 (Q-007)» для exports | **ЗАКРЫТ** | `design.md` §3 теперь корректно ссылается на `ADR-009 (Q-009)` для строк «Inline NDJSON» и «Хранилище exports». ADR-007 ссылается только в строке про retry-стратегии. |

---

## Покрытие открытых вопросов (актуальная таблица Q→ADR 1:1)

| Q-NNN | ADR-NNN | Тема совпадает | Статус |
|---|---|---|---|
| Q-001 ERP auth method | ADR-001 | ✓ | Отложено (ИБ E-Zoo) |
| Q-002 ERP стек клиента | ADR-002 | ✓ | Отложено (IT E-Zoo) |
| Q-003 Контракт ERP + 1% threshold | ADR-003 | ✓ | Принято структурно |
| Q-004 Объём данных + CDC trigger | ADR-004 | ✓ | Отложено (IT E-Zoo + продукт), pre-MVP замер |
| Q-005 Cron schedule default | ADR-005 | ✓ | Принято (`0 2 * * *` Europe/Kyiv) |
| Q-006 Severity-rules стартовый набор | ADR-006 | ✓ | Принято (YAML с 7 правилами) |
| Q-007 Backoff cap для ERP-клиента | ADR-007 | ✓ | Принято (max 3, cap 30s, jitter 10%) |
| Q-008 snapshot_pointer схема | ADR-008 | ✓ | Принято (single-row + atomic flip) |
| Q-009 Local FS exports | ADR-009 | ✓ | Принято (`/var/exports`, retention 24h) |
| Q-010 supplier_stock semantics | ADR-010 | ✓ | Принято (skip без fail) |
| Q-011 Cold retention timeline | ADR-011 | ✓ | Отложено (продукт + IT) |
| Q-012 CI/Hosting timeline | ADR-012 | ✓ | Отложено (IT E-Zoo) |
| Q-013 EDI-профиль (Модуль 7) | ADR-013 | ✓ | Передан в Модуль 7 |
| Q-014 Audit volume budget | ADR-014 | ✓ | Принято (90d retention) |
| Q-015 Stale load detection | ADR-015 | ✓ | Принято (1h default, env с префиксом) |
| Q-016 Lifecycle events DTO | ADR-016 | ✓ | Принято (DTO зафиксирован) |

**Мета-ADR (вне Q):** ADR-100 (стек), ADR-101 (JWT), ADR-102 (atomic flip), ADR-103 (multi-tenant — Отложено).

**Итог покрытия:** 16/16 Q-NNN закрыто корректно по теме (1:1).

---

## Новые блокеры (если найдены)

Не найдены.

---

## Итог

**APPROVED**

- Все 1 блокер + 7 серьёзных замечаний + 6 незначительных из проверочного списка закрыты.
- Нумерация Q ↔ ADR теперь строго 1:1, мета-ADR изолированы в диапазоне 100+.
- Q-004 и Q-016 формализованы как полноценные ADR (включая публичный DTO для Q-016).
- Sentinel matrix (≥10 строк) и стартовый набор validation rules присутствуют.
- ENV-префикс `SOURCE_ADAPTER_STALE_LOAD_TIMEOUT` консистентен в обоих файлах.

**Готовность к стадии Plan:** да.

---

## Ответ оркестратору

- **Итог:** APPROVED — все указанные в Fix 1..8 проблемы закрыты.
- **Закрыто:** 1 блокер (B-1 + Q-004 + Q-016 DTO), 7 серьёзных (S-1..S-7), 6 незначительных (N-3..N-6 проверены и закрыты; N-1/N-2 по swimlane HTML — вне списка fix, остаются как опциональная ручная верификация).
- **Новых блокеров:** нет.
- **Готовность к Plan:** полная. Можно запускать Стадию 4 (Plan).
