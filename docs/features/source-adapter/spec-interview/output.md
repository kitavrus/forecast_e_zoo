# Spec: source-adapter
**Дата интервью:** 2026-05-07
**На основе:** docs/features/source-adapter/research/output.md + docs/features/data-export/draft-plan.md + docs/tmp/data-export/spec-2026-05-05.md + docs/tmp/data-export/erp-integration-requirements.md + docs/tmp/data-marts/contract-2026-05-06.md + docs/tmp/replenishment/spec-2026-05-06.md

> **Статус кодовой базы:** greenfield. В репозитории сейчас только `docs/` и `.claude/`. Все артефакты ниже — для проекта, который будет создаваться **с нуля**: `go.mod`, `cmd/server`, `internal/features/data_export/...`, `migrations/`, `docker-compose.yml`.

---

## Проблема и цель

E-Zoo (зоомагазины) ежедневно генерирует мастер-данные и факты (продажи, остатки, движения, поставки) внутри своей ERP-системы. Без адаптера эти данные «закрыты» внутри ERP и недоступны для X-Flow ETL → витрин → модуля прогноза/реплинишмента. Цель Модуля 1 «Адаптер источников» — единая точка извлечения данных из ERP клиента и предоставления их вышестоящим модулям через стабильный контракт, изолируя их от деталей ERP. Реализация в MVP — одна (pull-выгрузка из ERP E-Zoo раз в сутки), но архитектура (`SourceReader`, DTO, мапперы) должна позволить добавить второй источник (другой ERP, EDI, SFTP) без переписывания вышестоящих модулей.

**Метрики успеха (3 SLA):**
1. **Freshness:** в 99% дней `loads.committed_at < 06:00 Europe/Kyiv` (для текущего суточного снапшота).
2. **Completeness:** в успешном load-е ≥99.5% строк `receipt_line` за сутки прошли валидацию severity=critical (т.е. `lines_failed/lines_total < 0.5%`; жёсткий порог fail load = >1%, см. ниже).
3. **Mart availability:** витрины `mart_*` обновлены к 07:00 Europe/Kyiv в ≥95% дней (метрика разделяется с командой ETL/витрин, но для адаптера это входной SLA).

**Не нужна, когда:** нет источника (новый клиент без ERP-интеграции), либо включена ручная выгрузка в обход MVP.

---

## Пользователи (потребители контракта)

| Тип | Что делает | Через что |
|---|---|---|
| **X-Flow ETL** | Читает мастер-данные и факты, строит `mart_*` | `GET /v1/{entity}` (NDJSON inline или Parquet через signed URL/local-FS), `GET /v1/snapshots/current` |
| **Модуль Replenishment (косвенно)** | Зависит от свежести `mart_*`, на адаптер напрямую не ходит | через витрины |
| **DevOps / on-call X-Flow** | Перезапуск load-а, просмотр reject-log, проверка lock-а | `POST /admin/loads`, `POST /admin/loads/{id}/retry`, `GET /admin/loads/{id}`, `GET /admin/reject-log`, Prometheus, Grafana |
| **IT E-Zoo** | Read-only audit — кто и когда дёргал admin-endpoints | `audit_access` (read-only dashboard вне MVP) |

**Owner модуля:** команда X-Flow (код, ревью severity-правил, on-call). E-Zoo IT отвечает только за креды и контракт ERP.

---

## Сценарии использования

### Happy path (суточная выгрузка)
1. Внутрипроцессный cron срабатывает по расписанию (configurable, default `02:00 Europe/Kyiv`, см. Q-005).
2. Сервис берёт PG advisory lock на ключ `daily-load`. Если lock занят — выходит без ошибки (другой инстанс уже работает).
3. Создаёт запись `loads(load_id, started_at, status='running', source='erp_e_zoo')`.
4. По очереди вызывает `SourceReader.Read{Entity}(ctx, since)` для каждой сущности (master сначала, факты потом). На каждой пагинированной странице:
   - Маппит ERP DTO → внутренний домен через `mapper`.
   - Гонит через severity-валидатор (`validation_rules.yaml`).
   - Строки `severity=critical` пишутся в `reject_log`, не в основную таблицу.
   - Валидные строки — UPSERT в staging-таблицы под `load_id`.
5. После всех сущностей: проверяет `lines_failed / lines_total < 1%` (см. порог ниже). Если ОК — атомарно flip-ает `snapshot_pointer.current_load_id = load_id` в одной транзакции.
6. Помечает `loads.status='committed', loads.committed_at=now()`. Освобождает lock. Метрика `source_adapter_load_success_total` инкрементируется.

### Edge cases
| # | Случай | Поведение |
|---|---|---|
| E1 | ERP недоступен (timeout/5xx) во время чтения сущности | Текущий load переходит в `status='failed', failed_reason='erp_unavailable'`. Snapshot не flip-ается. Alert. Никакого auto-retry; восстановление — оператор через `POST /admin/loads/{id}/retry` (запускает **новый** load с нуля). |
| E2 | Параллельный `POST /admin/loads` пока крон уже идёт | 409 Conflict + ссылка на текущий `load_id`. `force=true` — **не реализуется в MVP** (отклонено, см. ниже). |
| E3 | Доля critical-строк >1% от total | Load fail (`status='failed', failed_reason='quality_threshold_exceeded'`). Snapshot не flip-ается. Alert. |
| E4 | Дубликат `product_id` внутри одного load-а | Critical row → `reject_log`. Если общий процент critical <1%, load всё равно может зафиксироваться. |
| E5 | `event_time > now()+15min` | Critical row → `reject_log`. |
| E6 | Отрицательные остатки / qty в чеке | Critical row → `reject_log`. |
| E7 | VPN flapping в середине выгрузки | Сетевая ошибка → весь load failed. Backoff/retry **на уровне HTTP-клиента внутри одного запроса** — да (cap см. Q-007). На уровне load-а — рестарт всего load-а оператором. |
| E8 | Размер ответа экспортного запроса >50 MB | Endpoint возвращает 202 + `export_id`; данные пишутся на local FS, далее `GET /v1/exports/{id}` отдаёт redirect/stream через Fiber static. См. Q-009. |
| E9 | `supplier_stock_snapshot` отсутствует у ERP | Сущность скипается в чтении, endpoint возвращает пустой набор. Не fail load. Q-010. |

### Прерванный сценарий
- Процесс убит (SIGTERM, OOM) во время выгрузки → `loads.status` остаётся `running` (стейл). При следующем запуске `cron` берётся advisory lock, видит стейл-запись (>1ч `started_at`) → помечает её `status='aborted'` и стартует новый load с нуля. Snapshot не флипался — потребители продолжают читать прошлый committed snapshot.

---

## Технические ограничения

| Ограничение | Решение |
|---|---|
| **Синхронность** | Cron-job — асинхронный (внутрипроцессный go-cron). API чтения — синхронный (read из PG, NDJSON streaming). API экспорта >50 MB — асинхронный (202 + export_id). |
| **Производительность** | Inline NDJSON для ответов <50 MB. Larger — async export на local FS + signed URL/static. Cursor-пагинация. ETag/`If-Modified-Since` на master-сущности. |
| **Идемпотентность** | PG advisory lock на ключ `daily-load`. Параллельный `POST /admin/loads` без force → 409 + ссылка на текущий `load_id`. |
| **Retry** | Рестарт **всего load-а** (упрощение MVP, не per-entity checkpoint). HTTP-уровневый retry с backoff внутри одного запроса к ERP — да. Cap: см. Q-007. |
| **Snapshot consistency** | Atomic flip `snapshot_pointer.current_load_id` в одной транзакции после успеха ВСЕХ сущностей. Потребители всегда видят консистентный снапшот. |
| **Quality threshold** | `lines_failed/lines_total > 1%` → load failed (строже стандартного 5%). |
| **Расписание** | Configurable env-var `SOURCE_ADAPTER_CRON_SCHEDULE` (cron-формат) + `SOURCE_ADAPTER_TZ` (IANA TZ). Default фиксируется на этапе design (см. Q-005). |
| **Объём данных** | Если `receipt_line/сутки > 10M` — pull-режим неприменим. См. Q-004 (требует CDC, не MVP). |
| **Внешние зависимости** | PG18 (обязательно), local FS (для exports). Никакого Redis/Kafka. **S3/MinIO — НЕ используется в MVP** (см. ниже). |
| **Retention** | Только PG, hot-данные. Cold-слой 365d Parquet/S3 — отложен (см. Q-011). |
| **Module path** | `github.com/Kitavrus/e_zoo` |
| **CI/CD** | В MVP CI **не настраивается**. Только локальный `docker-compose up`. См. Q-012. |
| **Hosting** | Локальная разработка через docker-compose. Целевой prod-deploy — Q-012. |

---

## Безопасность и доступ

| Что | Как |
|---|---|
| **Auth API адаптера (потребители)** | Service-to-service JWT (HS256 для MVP, RS256 опционально). Middleware на всех `/v1/*` и `/admin/*`. JWT issuer/secret через env. Issuer claim определяет роль (`x-flow-etl`, `admin-cli`). |
| **Auth к ERP клиента** | Не определён в MVP (ERP не выбран). См. Q-001, Q-002. Реализация — pluggable через интерфейс `SourceAuth` в HTTP-клиенте. |
| **Чувствительные данные** | Креды ERP — только через env-vars, никогда в коде/логах. Бизнес-данные (qty, цены) — не PII, special handling не нужен. |
| **Audit** | Пишем в `audit_access` ТОЛЬКО вызовы `/admin/*` (POST /admin/loads, /retry, GET /admin/reject-log). `/v1/*` — без audit (иначе взрыв объёма). |
| **Несанкционированный доступ** | 401 при отсутствии/невалидном JWT. 403 при валидном JWT, но недостаточной роли (например `x-flow-etl` пытается дёрнуть `/admin/loads`). 404 не используется как маскировка. |

---

## Обработка ошибок

| Ошибка | HTTP | code (errorspkg) | Что видит потребитель | Действие системы |
|---|---|---|---|---|
| Нет JWT / невалидный | 401 | `auth_invalid_token` | `{code, message: "auth required"}` | Логируется access denied. Не пишет в audit. |
| Недостаточно прав | 403 | `auth_forbidden` | `{code, message: "forbidden"}` | Audit (для admin endpoints). |
| Snapshot не готов (первый запуск, ещё не было успешного load-а) | 503 | `snapshot_not_ready` | `{code, message: "no committed snapshot yet"}` + `Retry-After: 60` | Метрика `snapshot_not_ready_total`. |
| Несуществующий entity / load_id | 404 | `not_found` | `{code, message}` | — |
| Неверные параметры запроса | 400 | `bad_request` | `{code, message, details: [{field, error}]}` | — |
| Параллельный запуск load-а | 409 | `load_already_running` | `{code, message, currentLoadId}` | — |
| ERP недоступен (внутри cron-выгрузки) | n/a | n/a (внутренняя) | потребитель не видит — load failed | `loads.status='failed'`, alert, метрика `load_failed_total{reason="erp_unavailable"}`. |
| Quality threshold exceeded | n/a | n/a (внутренняя) | потребитель не видит | `loads.status='failed'`, alert, метрика `load_failed_total{reason="quality_threshold_exceeded"}`. |
| Внутренняя 5xx | 500 | `internal` | `{code, message: "internal error", traceId}` | Логируется stacktrace. |

Формат ответа единый: `pkg/errorspkg.ErrorResponseJSON` с полями `code`, `message`, `supportMessage`, `traceId`.

---

## Принятые компромиссы (MVP-упрощения)

1. **Только один источник** (`erp_e_zoo_reader`). Интерфейс `SourceReader` закладывается, но registry/plugins — не MVP.
2. **Retry = рестарт всего load-а**, не per-entity checkpoint. Заплатим временем при сбое поздних сущностей. Исправить в next iter, когда станут известны реальные объёмы.
3. **Без cold-слоя retention.** S3/Parquet, 365d cold — не MVP. Только hot PG (30 дней `event_date`-партиций для фактов). Cold-слой — отдельная итерация.
4. **Без CI.** Локальная разработка через docker-compose. Pipeline GitHub Actions/GitLab CI — отдельная задача.
5. **Без Web UI / dashboards.** Только Prometheus + Grafana и admin-CLI (curl/httpie).
6. **Без CDC/streaming.** Только pull раз в сутки. Если объёмы превысят 10M `receipt_line/сутки` — переход на CDC отдельной задачей.
7. **Без multi-tenant.** Один клиент (E-Zoo). Tenant-изоляция — отдельная задача.
8. **Async exports на local FS**, не на S3. Single-replica деплой. При переходе на multi-replica — заменить на S3 (один env-var + `Storage` интерфейс).
9. **`force=true` для POST /admin/loads — не реализуется.** При параллельном запуске — всегда 409. Если нужен принудительный — сначала вручную abort через DB (или будущий `POST /admin/loads/{id}/abort`).
10. **`supplier_stock_snapshot` берём из ERP**, если есть. Если ERP не отдаёт — таблица создаётся, но остаётся пустой; endpoint отдаёт `[]`. Альтернативный источник (EDI INVRPT, SFTP поставщика) — не MVP.

---

## Отклонённые решения

| Что | Почему отклонено |
|---|---|
| `force=true` для перезапуска load-а | Риск race-condition и расходящихся снапшотов. Простое 409 + отдельная процедура abort безопаснее. |
| Per-entity retry checkpoint | Усложнение на этапе MVP без подтверждённой бизнес-потребности. Рестарт всего load-а проще и достаточен при объёмах <10M строк/сутки. |
| Hybrid snapshot (master atomic, факты per-entity) | Сложнее для потребителей (X-Flow ETL должен учитывать «рваные» снапшоты). Atomic flip всех — единственная семантика для MVP. |
| S3 cold retention в MVP | Лишний инфраструктурный компонент в пилоте. Hot 30d в PG достаточно для проверки концепции. |
| Web UI для admin-операций | Vue/dashboard выносится на v2 (см. spec-replenishment §6.1). |
| CDC/Debezium | Только если pull не справится с объёмом. Премьер-юс-кейс пилота — гарантировано <10M строк/сутки. |
| Multi-tenant архитектура | MVP — один клиент. |
| API key + IP allowlist для API адаптера | JWT даёт лучше управление ролями (потребители разные: ETL, admin, IT-read). |

---

## Открытые вопросы

> Каждый `Q-NNN` будет закрыт одноимённым `ADR-NNN` на стадии Design.

1. **Q-001. ERP auth method.** OAuth2 client_credentials, mTLS или API-key + IP allowlist? Решение ИБ E-Zoo. Контекст: спека `erp-integration-requirements.md` оставляет три варианта; `SourceAuth` интерфейс должен поддержать выбор. **Что нужно от design:** определить интерфейс + первую реализацию (или явное Q открытое до executing).
2. **Q-002. ERP стек клиента.** 1С УТ / 1С Розница / SAP / кастомный? Влияет на тип контракта (REST/SOAP/SFTP). Контекст: research OQ-3 + ответ пользователя «Custom / unknown». **Что нужно:** placeholder-реализация `erp_e_zoo_reader` с тестовым in-memory backend, чтобы разработка не блокировалась.
3. **Q-003. Контракт ERP.** Variant A (REST + cursor + ETag), Variant B (SOAP), Variant C (SFTP CSV/Parquet/JSONL + manifest)? Контекст: erp-integration-requirements.md. **Что нужно:** интерфейс `SourceReader` инвариантен к выбору; HTTP-клиент абстрагирует REST vs SOAP, SFTP — отдельный adapter.
4. **Q-004. Объём данных.** Реальные SKU/locations/`receipt_line/сутки`. Если >10M строк/сутки — pull неприменим. **Что нужно:** Design-агент должен явно зафиксировать в риски и предложить pre-MVP замер у E-Zoo.
5. **Q-005. Cron schedule default.** Время + TZ суточной выгрузки. Контекст: configurable через env-var, но дефолт нужен. **Что нужно:** дефолт `02:00 Europe/Kyiv` (рекомендация) — подтвердить в design.
6. **Q-006. Severity-rules стартовый набор.** Где лежит `validation_rules.yaml`, кто owner правил, как меняется (через PR на YAML?). Контекст: ADR-006 spec-data-export. **Что нужно:** структура YAML, mapping severity→action, базовый набор для master + фактов.
7. **Q-007. Backoff cap для HTTP-клиента к ERP.** Max retries, max delay, jitter. Контекст: VPN flapping (риск §11). **Что нужно:** конкретные числа (например max 3 retries, exponential backoff cap 30s, jitter 10%).
8. **Q-008. snapshot_pointer схема.** Структура таблицы (single-row? per-entity? lock-режим?). Контекст: атомарный flip требует UPDATE...WHERE current_load_id IS NOT NULL FOR UPDATE. **Что нужно:** DDL + транзакционная семантика flip.
9. **Q-009. Local FS exports.** Путь хранения (`/var/exports/{id}.parquet`?), retention (24ч?), cleanup-механизм (cron внутри сервиса?), как отдаётся (Fiber static с подписанным токеном или signed URL?). **Что нужно:** конкретный layout + SourceWriter интерфейс для будущей замены на S3.
10. **Q-010. supplier_stock semantics.** Если ERP не отдаёт `supplier_stock_snapshot` — должен ли load fail, или skip-ать сущность? Решение MVP: skip без fail. **Что нужно:** конфиг `entity.optional=true` в `validation_rules.yaml`.
11. **Q-011. Cold retention timeline.** Когда вводим S3/Parquet 365d? Триггер: объём PG > N GB или просто календарный milestone? **Что нужно:** в design зафиксировать как Q-NNN, не реализовывать.
12. **Q-012. CI/Hosting timeline.** Когда вводим CI и где деплоим в prod? **Что нужно:** в design зафиксировать как Q-NNN, не реализовывать.
13. **Q-013. EDI-профиль для маршрутизации заказов.** Не блокирует адаптер (Модуль 7), но в research OQ-4. **Что нужно:** перенести в research/spec Модуля 7. В spec source-adapter не закрываем.
14. **Q-014. Audit volume budget.** Сколько строк `audit_access` в день ожидается? Cleanup retention (365d? 90d?). **Что нужно:** оценка + DDL retention policy.
15. **Q-015. Stale load detection.** Через сколько часов `loads.status='running'` считается стейл-записью и помечается aborted при следующем cron-tick? Default 1ч? **Что нужно:** конкретное значение в design + env-var.
16. **Q-016. Lifecycle events для store_assortment.** Контракт `lifecycle_events` endpoint (start/stop/promo) — что именно отдавать. Контекст: contract-2026-05-06.md. **Что нужно:** уточнённая схема DTO в design.
