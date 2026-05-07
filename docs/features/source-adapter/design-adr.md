# Design ADR — source-adapter

ADR-001..016 — **строго 1:1** с Q-001..016 из `spec-interview/output.md` (по теме).
ADR-100..103 — мета-ADR для архитектурных решений, которые НЕ закрывают конкретный
открытый Q (стек/JWT/multi-tenant/atomic-flip и т.п.).

Если вопрос требует решения от внешней стороны (IT E-Zoo / ИБ E-Zoo / продукт E-Zoo) —
статус **Отложено**, эскалация явно зафиксирована.

---

## Crosswalk Q-NNN ↔ ADR-NNN

| Q-NNN | Тема | ADR | Статус |
|---|---|---|---|
| Q-001 | ERP auth method | ADR-001 | Отложено (ИБ E-Zoo) |
| Q-002 | ERP стек клиента | ADR-002 | Отложено (IT E-Zoo) |
| Q-003 | Контракт ERP (REST/SOAP/SFTP) + 1% threshold | ADR-003 | Принято структурно |
| Q-004 | Объём данных + CDC trigger | ADR-004 | Отложено (IT E-Zoo + продукт) |
| Q-005 | Cron schedule default | ADR-005 | Принято |
| Q-006 | Severity-rules стартовый набор | ADR-006 | Принято |
| Q-007 | Backoff cap для ERP HTTP-клиента | ADR-007 | Принято |
| Q-008 | snapshot_pointer схема | ADR-008 | Принято |
| Q-009 | Local FS exports | ADR-009 | Принято |
| Q-010 | supplier_stock semantics (skip vs fail) | ADR-010 | Принято |
| Q-011 | Cold retention timeline | ADR-011 | Отложено (продукт + IT) |
| Q-012 | CI/Hosting timeline | ADR-012 | Отложено (IT E-Zoo) |
| Q-013 | EDI-профиль (Модуль 7) | ADR-013 | Передан в Модуль 7 |
| Q-014 | Audit volume budget | ADR-014 | Принято |
| Q-015 | Stale load detection | ADR-015 | Принято |
| Q-016 | Lifecycle events DTO | ADR-016 | Принято |

| Мета-ADR | Тема | Статус |
|---|---|---|
| ADR-100 | Стек: Go/Fiber v3/pgxpool/go:embed | Принято |
| ADR-101 | JWT-аутентификация API адаптера | Принято |
| ADR-102 | Atomic snapshot flip (single-row + транзакция) | Принято |
| ADR-103 | Multi-tenant архитектура | Отложено (продукт) |

---

## ADR-001 — ERP auth method (Q-001)

**Статус:** Отложено
**Отвечает на Q-001:** «OAuth2 client_credentials, mTLS или API-key + IP allowlist? Решение ИБ E-Zoo».
**Эскалация:** ИБ E-Zoo + IT E-Zoo. Ответственный за решение: CISO E-Zoo.
**Срок ответа:** до начала integration-тестов с реальным ERP (не блокирует MVP-разработку).

**Контекст:** spec `erp-integration-requirements.md` оставляет три варианта; интерфейс
`SourceAuth` должен поддержать выбор, чтобы разработка не блокировалась.

**Решение в design:**
- Введён интерфейс `SourceAuth` с реализациями `bearerAuth`, `mtlsAuth`, `apiKeyAuth`, `noAuth`.
- ENV `ERP_AUTH_MODE` управляет выбором (`none|bearer|mtls|apikey`).
- Default `none` → in-memory dev backend (Q-002).

**Альтернативы:**
- Зашить один auth-метод сразу (отклонено: меняем код при смене политики ИБ).
- Ждать решения ИБ перед стартом разработки (отклонено: блокирует MVP).

**Риски:**
- Если ИБ выберет mtls — потребуется управление сертификатами/ротация (новая операционная задача).
- Если bearer (OAuth2) — нужна реализация refresh-цикла и обработки 401 expired.

## ADR-002 — ERP стек клиента (Q-002)

**Статус:** Отложено
**Отвечает на Q-002:** «1С УТ / 1С Розница / SAP / кастомный? Влияет на тип контракта (REST/SOAP/SFTP)».
**Эскалация:** IT E-Zoo. Нужно выяснить: 1С (УТ/Розница), SAP, кастом?
**Срок ответа:** до integration-тестов.

**Контекст:** research OQ-3 + ответ пользователя «Custom / unknown».

**Решение в design:**
- В MVP реализуется `service/reader_inmem.go` (in-memory backend с детерминированными seed-данными).
- `service/reader_erp_http.go` — заглушка HTTP-импла с TODO-комментариями для адаптации под выбранный
  стек.
- Если SOAP — добавляется `reader_erp_soap.go`. Если SFTP — `reader_erp_sftp.go`. Все импл-ы
  удовлетворяют единому `SourceReader` интерфейсу.

**Альтернативы:**
- Сразу готовить SOAP-клиент под 1С (отклонено: 1С может оказаться не выбран).
- Ждать решения IT (отклонено: блокирует разработку).

**Риски:**
- Цена рефакторинга адаптера данных, когда ERP определится (мапперы под конкретные ERP-DTO).

## ADR-003 — Контракт ERP + 1% quality threshold (Q-003)

**Статус:** Принято (структурно), реализация транспорта — Отложено
**Отвечает на Q-003:** «Variant A (REST + cursor + ETag), Variant B (SOAP), Variant C (SFTP) — какой
выбрать?».
**Эскалация:** IT E-Zoo (после решения Q-002).

**Контекст:** erp-integration-requirements.md описывает 3 варианта транспорта. Quality threshold
(1% vs 5%) синхронизирован с Replenishment hard-threshold.

**Решение:**
- `SourceReader` интерфейс инвариантен к транспорту (REST/SOAP/SFTP).
- Quality threshold: **`lines_failed / lines_total > 1%` ⇒ load failed** (строже стандартного 5%).
  Это гарантирует консистентность downstream-витрин в Replenishment.
- ENV: `QUALITY_THRESHOLD_PCT=1.0`.

**Альтернативы:**
- Threshold 5% (как стандарт) — отклонено: тихая порча витрин при грязных данных.
- Threshold per-entity — отклонено: усложнение MVP.

**Риски:**
- Жёсткий старт приведёт к фейлам в первый месяц (грязные данные ERP), но это лучше тихой порчи витрин.

## ADR-004 — Объём данных + CDC trigger (Q-004)

**Статус:** Отложено
**Отвечает на Q-004:** «Реальный объём (SKU, locations, receipt_line/сутки) — потенциальное
превышение >10M строк/сутки означает невозможность pull-режима».
**Эскалация:** IT E-Zoo (запрос замера у E-Zoo до Date X) + продукт-менеджер X-Flow владеет
статусом.
**Срок ответа:** pre-MVP замер у E-Zoo.

**Контекст:** ERP клиента не выбран (Q-002), реальные объёмы (SKU, locations, receipt_line/сутки)
неизвестны. Если суточный объём фактов превысит ~10M строк — pull-режим неприменим, нужен CDC.

**Решение MVP:**
- Pull-режим, оценка 1M–5M строк/сутки приемлема для MVP.
- Pre-MVP замер у E-Zoo IT (дата фиксируется PM X-Flow).
- Если замер покажет >10M строк/сутки — переход на CDC/Debezium **отдельной задачей**
  (не блокирует MVP scheduler+reader).

**Альтернативы:**
- Сразу CDC (Debezium) — отклонено: overengineering до подтверждённой потребности.
- Гибрид (pull master + CDC факты) — отклонено: сложность инфраструктуры.

**Риски:**
- Pull не справится с реальным объёмом → перенос дедлайна Модуля 1.
- Необходимость репроектирования scheduler+reader при переходе на CDC.

## ADR-005 — Cron schedule default (Q-005)

**Статус:** Принято
**Отвечает на Q-005:** «Время + TZ суточной выгрузки. Дефолт нужен».

**Решение:**
- Default cron-выражение: `0 2 * * *` (02:00 каждый день).
- Default TZ: `Europe/Kyiv`.
- ENV: `SOURCE_ADAPTER_CRON_SCHEDULE`, `SOURCE_ADAPTER_TZ`.
- Library: `github.com/go-co-op/gocron/v2` с `WithSingletonMode(LimitModeReschedule)`.

**Альтернативы:**
- 04:00 (после ETL-окон Replenishment) — отклонено: даёт меньше запаса до 06:00 SLA.
- Без TZ-параметра (всегда UTC) — отклонено: операторы привязаны к Kyiv local time.

**Риски:**
- 02:00 Kyiv может оказаться внутри backup-окон ERP (узнать после первого месяца замеров).

## ADR-006 — Severity-движок валидации + стартовый набор правил (Q-006)

**Статус:** Принято
**Отвечает на Q-006:** «Где лежит validation_rules.yaml, кто owner правил, как меняется. Структура
YAML, mapping severity→action, базовый набор для master + фактов».

**Решение:**
- `validation_rules.yaml` — owner: BackEnd team (E-Zoo). Изменения через PR в репо.
- Severity = `critical | soft`.
- `critical` → строка идёт в `reject_log`, не в основную таблицу. Влияет на quality threshold (ADR-003).
- `soft` → строка попадает в основную таблицу + метрика `validation_soft_total{rule_id}`.
- Стартовый набор правил (см. ниже) покрывает master (`products`) + факты (`receipt_line`,
  `location_stock_snapshot`) + supplier_stock.

**Стартовый набор `validation_rules.yaml`:**

```yaml
version: 1
rules:
  - id: negative_qty
    entity: receipt_line
    field: qty
    severity: critical
    expr: "qty < 0"
    message: "Количество не может быть отрицательным"
  - id: future_event_time
    entity: receipt_line
    field: event_time
    severity: critical
    expr: "event_time > now() + interval '15 minutes'"
    message: "event_time не может быть в будущем (>15 минут)"
  - id: duplicate_product_in_load
    entity: products
    field: id
    severity: critical
    expr: "duplicate within load"
  - id: missing_required_field
    entity: products
    field: id
    severity: critical
    expr: "id IS NULL"
  - id: negative_stock_balance
    entity: location_stock_snapshot
    field: balance
    severity: critical
    expr: "balance < 0"
  - id: orphan_fk
    entity: receipt_line
    field: product_id
    severity: soft
    expr: "product_id NOT IN (SELECT id FROM products)"
  - id: stale_event_time
    entity: receipt_line
    field: event_time
    severity: soft
    expr: "event_time < now() - interval '90 days'"
```

**Альтернативы:**
- Hardcoded правила в Go — отклонено: при добавлении правила нужен релиз (медленно).
- DSL вместо YAML (CEL/Rego) — отклонено: overengineering для MVP.

**Риски:**
- Грязные данные ERP в первый месяц → много critical → load fail (mitigated через ADR-003 порог).

## ADR-007 — Backoff cap для ERP HTTP-клиента (Q-007)

**Статус:** Принято
**Отвечает на Q-007:** «Max retries, max delay, jitter».

**Контекст:** VPN flapping (риск §11 spec).

**Решение:**
- `ERP_RETRY_MAX=3` (default).
- `ERP_RETRY_BACKOFF_CAP=30s` (exp backoff: 1s, 2s, 4s, ..., capped at 30s).
- Jitter: 10%.
- Retry на: 408, 429, 500, 502, 503, 504, network errors.
- НЕ retry на: 4xx (кроме 408/429), `context.Canceled`.
- На уровне load-а: рестарт **всего load-а** (не per-entity checkpoint). Оператор делает
  `POST /admin/loads/{id}/retry`.

**Альтернативы:**
- Без retry — отклонено: VPN flapping приводил бы к ежедневным фейлам.
- Per-entity checkpoint — отклонено: усложнение MVP без подтверждённой бизнес-потребности.

**Риски:**
- При VPN flapping load fail-нется быстро, оператор перезапускает.

## ADR-008 — snapshot_pointer схема (Q-008)

**Статус:** Принято
**Отвечает на Q-008:** «Структура таблицы (single-row? per-entity? lock-режим?). DDL +
транзакционная семантика flip».

**Решение:**
- `snapshot_pointer` — single-row table с гарантией `id=1` (CHECK).
- Поля: `current_load_id`, `previous_load_id`, `committed_at`.
- Flip — атомарная транзакция (см. мета-ADR-102):
  `UPDATE snapshot_pointer SET previous_load_id = current_load_id, current_load_id = $1, committed_at = now() WHERE id = 1`.
- Чтение из `/v1/*` — фиксирует `current_load_id` в начале запроса (не следит за flip в середине).

**Альтернативы:**
- Per-entity pointer — отклонено: «рваные» снапшоты для X-Flow ETL.
- Append-only журнал снапшотов — отклонено: усложнение чтения.

**Риски:**
- При connection lost во время UPDATE — `loads.status='failed', failure_reason='flip_failed'`.

## ADR-009 — Local FS exports (Q-009)

**Статус:** Принято
**Отвечает на Q-009:** «Путь хранения, retention, cleanup-механизм, как отдаётся».

**Решение:**
- **Async exports >50 MB:** local FS, путь `/var/exports/{id}.{format}`.
- Retention: 24 часа. Cleanup-job внутри сервиса (cron каждый час).
- Выдача: handler `GET /v1/exports/{id}/download` через `Fiber c.SendFile` + JWT-проверка
  middleware (доступ только для аутентифицированного потребителя).
- Интерфейс `ExportsStorage` (Write/Open/Delete) — для будущей замены на S3 без изменений в
  loader/handler.

**Альтернативы:**
- S3/MinIO — отклонено в MVP (лишний инфраструктурный компонент).
- Signed URL с временным токеном — отклонено: JWT через middleware проще.

**Риски:**
- Local FS не масштабируется на multi-instance, но MVP — single-instance.

## ADR-010 — supplier_stock semantics (Q-010)

**Статус:** Принято
**Отвечает на Q-010:** «Если ERP не отдаёт supplier_stock_snapshot — должен ли load fail, или
skip-ать сущность?».

**Решение:**
- В `validation_rules.yaml` сущность помечена `optional: true` (entity-level флаг).
- Если ERP не отдаёт — load **не** fail-ается, эндпоинт `/v1/supplier_stock_snapshot`
  возвращает пустой результат `[]`.
- Метрика: `entity_rows_processed{entity="supplier_stock_snapshot",result="skipped"}`.

**Альтернативы:**
- Fail load при отсутствии supplier_stock — отклонено: ERP клиента может не поддерживать сущность.
- Альтернативный источник (EDI INVRPT, SFTP поставщика) — не MVP, отдельная задача.

**Риски:**
- Replenishment не получит данных о складе поставщика → пропадёт фактор для прогноза дефицита.

## ADR-011 — Cold retention timeline (Q-011)

**Статус:** Отложено
**Отвечает на Q-011:** «Когда вводим S3/Parquet 365d cold-слой? Триггер: объём PG > N GB или
календарный milestone?».
**Эскалация:** Продукт E-Zoo + IT E-Zoo (после первого месяца — замер объёма PG).

**Решение в design:**
- Hot 30d в PG18 — достаточно для MVP.
- Партиционирование по месяцам уже введено (PG18 declarative) — drop-партиции дешёво, когда
  retention превысит лимит.
- S3/Parquet 365d cold-слой реализуется в отдельной фиче (вне source-adapter), активируется по
  триггеру: PG > N GB ИЛИ календарный milestone (TBD).

**Альтернативы:**
- Сразу S3 в MVP — отклонено: лишний инфраструктурный компонент в пилоте.

**Риски:**
- При росте объёмов >50 GB запросы могут замедлиться (большие индексы). Mitigation —
  drop старых партиций.

## ADR-012 — CI/Hosting timeline (Q-012)

**Статус:** Отложено
**Отвечает на Q-012:** «Когда вводим CI и где деплоим в prod?».
**Эскалация:** IT E-Zoo. Нужно решение по CI (GitHub Actions / Bitbucket / GitLab) и target prod
(VM/k8s/managed).

**Решение в design:**
- В MVP CI **не настраивается**. `Makefile` содержит локальные команды (`make test`, `make lint`,
  `make docker-build`).
- Docker-образ пригоден для k8s (distroless, non-root, healthcheck).
- Логи — stdout (compatible с любым log shipper).

**Альтернативы:**
- Сразу настроить GitHub Actions — отклонено: target CI ещё не выбран.

**Риски:**
- Разработчик обязан гонять `make test-all && make lint` перед коммитом вручную. На малой команде
  допустимо.

## ADR-013 — EDI-профиль для маршрутизации заказов (Q-013)

**Статус:** Передан в Модуль 7 (не закрывается в фиче source-adapter)
**Отвечает на Q-013:** «EDI-профиль для маршрутизации заказов — Модуль 7».
**Эскалация:** Перенесено в research/spec Модуля 7 (Order routing / EDI ORDERS+DESADV).

**Решение в design:**
- В source-adapter ничего активного не делаем. Поле `supplier.edi_profile` сохраняется как
  свободный текст без enum-валидации (placeholder для будущего модуля).

**Альтернативы:**
- Захардкодить enum EDI-профилей в адаптере — отклонено: ответственность Модуля 7.

**Риски:**
- При появлении Модуля 7 потребуется миграция значений `supplier.edi_profile` под enum.

## ADR-014 — Audit volume budget (Q-014)

**Статус:** Принято
**Отвечает на Q-014:** «Сколько строк audit_access в день ожидается? Cleanup retention (365d? 90d?)».

**Решение:**
- Audit retention: **90 дней** (`AUDIT_RETENTION=2160h`). Cleanup-job ежедневный.
- Оценка объёма: ~100 admin-запросов/сутки × 90 дней = ~9 тыс. строк → пренебрежимо.
- Партиционирование `audit_access` по месяцу (`PARTITION BY RANGE (ts)`); drop партиций по retention.
- Audit пишется ТОЛЬКО для `/admin/*` (см. spec §Безопасность). `/v1/*` — без audit (взрыв объёма).

**Альтернативы:**
- 365d retention — отклонено: для MVP избыточно (compliance не требует).
- Без партиционирования — отклонено: cleanup через DELETE медленнее, чем DROP PARTITION.

**Риски:**
- При росте admin-нагрузки >10k req/day партиции по месяцу станут крупными — пересмотреть
  retention (на квартал) или схему партиционирования.

## ADR-015 — Stale load detection (Q-015)

**Статус:** Принято
**Отвечает на Q-015:** «Через сколько часов loads.status='running' считается стейл-записью и
помечается aborted при следующем cron-tick? Default 1ч?».

**Решение:**
- На старте процесса (после миграций) запускается `RecoverStaleLoads(staleTimeout)`.
- Default: `SOURCE_ADAPTER_STALE_LOAD_TIMEOUT=1h`. ENV-vars с префиксом `SOURCE_ADAPTER_*`
  для консистентности с другими переменными модуля (`SOURCE_ADAPTER_CRON_SCHEDULE`,
  `SOURCE_ADAPTER_TZ`).
- `loads.status='running'` старше `staleTimeout` → принудительно
  `status='failed', failure_reason='stale_after_restart'`.

**Альтернативы:**
- Heartbeat в `loads.last_heartbeat_at` — отклонено: усложнение, не нужно при single-instance MVP.
- Default 30 минут — отклонено: типичный happy-path load на 5-10M строк может занять 20-40 мин.

**Риски:**
- При легитимном long-running load (>1ч) на больших объёмах — преждевременный abort. Mitigation:
  оператор поднимает `SOURCE_ADAPTER_STALE_LOAD_TIMEOUT` через env.

## ADR-016 — Lifecycle events DTO для store_assortment (Q-016)

**Статус:** Принято
**Отвечает на Q-016:** «Контракт lifecycle_events endpoint (start/stop/promo) — что именно
отдавать. Уточнённая схема DTO в design».

**Контекст:** contract-2026-05-06.md требует endpoint `GET /v1/store_assortment/lifecycle_events`
для X-Flow ETL — журнал переходов состояний (`active`, `inactive`, `promo`).

**Решение:**

Зафиксирован DTO ответа `StoreAssortmentLifecycleEventResponse`:

```go
type StoreAssortmentLifecycleEventResponse struct {
    EventID         string    `json:"eventId"`         // UUID
    EventType       string    `json:"eventType"`       // "started" | "stopped" | "promo_started" | "promo_stopped"
    LocationID      string    `json:"locationId"`
    ProductID       string    `json:"productId"`
    EffectiveAt     time.Time `json:"effectiveAt"`     // когда событие вступило в силу
    Reason          *string   `json:"reason,omitempty"` // напр. "out_of_stock", "promo_id=PR123"
    PromoID         *string   `json:"promoId,omitempty"`
    PriorState      *string   `json:"priorState,omitempty"`  // "active" | "inactive" | "promo"
    NewState        string    `json:"newState"`              // "active" | "inactive" | "promo"
    SourceLoadID    string    `json:"sourceLoadId"`
    CreatedAt       time.Time `json:"createdAt"`
}
```

JSON Schema: `additionalProperties: false`. Расширение `eventType` enum — forward-compatible
через миграцию + новую версию контракта.

Полные поля DTO (Go-уровень) — см. [design-go-layers.md](design-go-layers.md) §3.1
(`dto/store_assortment_lifecycle.go`). Sequence для эндпоинта — см.
[design-sequence-diagrams.md](design-sequence-diagrams.md) §4.

**Альтернативы:**
- (a) Единый event-log с полем `payload JSONB` — отклонено: неудобно для X-Flow ETL, теряется
  типизация.
- (b) Отдельные таблицы start/stop/promo — отклонено: избыточная сложность DDL.

**Риски:**
- При добавлении новых типов событий нужно расширять enum (forward-compatible через
  `additionalProperties: false` + версионирование контракта).

---

## Мета-ADR (вне диапазона Q-NNN)

> **Не отвечают на Q-NNN — самостоятельные архитектурные решения.**

## ADR-100 — Стек: Go/Fiber v3/pgxpool/go:embed

**Статус:** Принято
**Не отвечает на Q-NNN — самостоятельное архитектурное решение** (выбор стека продиктован
требованиями проекта в `spec-interview/output.md` §Технические ограничения).

**Решение:**
- Go 1.26 (минимально 1.21 для slog).
- Fiber v3: `c.Bind().Query(&req)`, `c.Bind().JSON(&req)`. `fiber.Ctx` (без указателя в v3).
- pgx/v5 + pgxpool. **Никаких ORM** (sqlx/gorm/ent).
- go:embed для миграций и SQL-запросов.
- golang-migrate/v4 для миграций; auto-apply **запрещён**, явный CLI-шаг.
- dockertest/v3 + `postgres:18-alpine` для integration-тестов.
- HTTP к ERP: `net/http` (стандартная библиотека) + кастомный retry-transport.
- Logger: `log/slog` (стандарт).
- Cron: `github.com/go-co-op/gocron/v2`.
- Config: `github.com/kelseyhightower/envconfig`.
- Metrics: `github.com/prometheus/client_golang`.

**Альтернативы:**
- chi/echo/gin вместо Fiber — отклонено: команда выбрала Fiber v3.
- sqlx/gorm — отклонено: нужен полный контроль SQL и понятный план запросов.

**Риски:**
- ORM-отсутствие → больше boilerplate. Mitigation — go:embed + типизированные helpers.

## ADR-101 — JWT-аутентификация API адаптера

**Статус:** Принято
**Не отвечает на Q-NNN — самостоятельное архитектурное решение** (auth API адаптера явно
зафиксирован в spec §Безопасность как Service-to-service JWT).

**Решение:**
- Service-to-service JWT, `HS256` для MVP, `RS256` опционально.
- Middleware на всех `/v1/*` и `/admin/*`.
- `JWT_ALG`, `JWT_SECRET` (HS256) или `JWT_PUBLIC_KEY_PATH` (RS256) через env.
- Issuer/sub claim определяет роль (`x-flow-etl`, `admin-cli`, `it-read`).
- 401 при отсутствии/невалидном токене, 403 при недостаточной роли.

**Альтернативы:**
- API key + IP allowlist — отклонено: JWT даёт лучше управление ролями (потребители разные).

**Риски:**
- HS256 секрет требует безопасного распространения. RS256 устраняет, ценой управления ключами.

## ADR-102 — Atomic snapshot flip (single-row UPDATE в транзакции)

**Статус:** Принято
**Не отвечает на Q-NNN — самостоятельное архитектурное решение** (atomic flip принят в
технических ограничениях spec-interview как обязательная семантика; ADR-008 закрывает только
схему таблицы).

**Решение:**
- Один UPDATE в транзакции, `snapshot_pointer.current_load_id` меняется только после успеха
  ВСЕХ сущностей и проверки quality threshold.
- Если flip fail (race / connection lost) — `loads.status='failed'`, `failure_reason='flip_failed'`.
- Чтение `/v1/*` фиксирует `current_load_id` в начале запроса.

**Альтернативы:**
- Hybrid (master atomic, факты per-entity) — отклонено: «рваные» снапшоты для X-Flow ETL.

**Риски:**
- Atomic flip требует чтобы ВСЕ сущности успешно загрузились перед commit. На объёмах
  <10M строк/сутки — приемлемо.

## ADR-103 — Multi-tenant архитектура

**Статус:** Отложено
**Не отвечает на Q-NNN — самостоятельное архитектурное решение.** Multi-tenant в spec-interview
зафиксирован как отдельный компромисс §«Принятые компромиссы», не как открытый Q.

**Эскалация:** Продукт E-Zoo. Не блокирует MVP.

**Решение в design:** В MVP — single-tenant. Никаких `tenant_id` колонок, никакого RLS.

**Альтернативы:**
- Сразу tenant_id во все таблицы — отклонено: overengineering для MVP с одним клиентом.

**Риски:**
- Добавление multi-tenant потребует миграции схемы (BREAKING).

---

## Summary статусов ADR

| Статус | ADR | Кол-во |
|---|---|---|
| **Принято (по Q-NNN)** | 003, 005, 006, 007, 008, 009, 010, 014, 015, 016 | 10 |
| **Отложено (эскалация)** | 001 (ИБ E-Zoo), 002 (IT E-Zoo), 004 (IT + продукт), 011 (продукт + IT), 012 (IT E-Zoo) | 5 |
| **Передан в другой модуль** | 013 (Модуль 7) | 1 |
| **Принято (мета)** | 100, 101, 102 | 3 |
| **Отложено (мета)** | 103 (Multi-tenant, продукт) | 1 |

**Итого:** 16 ADR-NNN покрывают 16 Q-NNN из spec-interview (1:1). 4 мета-ADR покрывают
самостоятельные архитектурные решения. 6 эскалаций требуют решения внешних сторон до начала
integration-фазы; ни одна не блокирует разработку MVP.
