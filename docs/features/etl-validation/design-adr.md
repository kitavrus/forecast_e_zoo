# Design ADR — etl-validation

> ADR-001..025 строго 1:1 с Q-NNN из `spec-interview/output.md`. Дополнительные мета-ADR в диапазоне `100+` фиксируют технические решения, которые не отражены напрямую в Q-NNN, но необходимы для проектирования.
>
> Каждый ADR имеет поля: **Статус**, **Отвечает на Q-NNN**, **Контекст**, **Решение**, **Альтернативы**, **Риски**.

---

## Crosswalk Q-NNN ↔ ADR-NNN

| Q-NNN | Тема | ADR | Статус |
|---|---|---|---|
| Q-001 | Binary (отдельный cmd/etl) | ADR-001 | Принято |
| Q-002 | Feature folder (etl_validation) | ADR-002 | Принято |
| Q-003 | Trigger (own cron 02:30) | ADR-003 | Принято |
| Q-004 | Idempotency (advisory lock + etl_runs) | ADR-004 | Принято |
| Q-005 | Atomic snapshot read | ADR-005 | Принято |
| Q-006 | Marts location (та же БД, schema marts) | ADR-006 | Принято |
| Q-007 | Partitioning (RANGE month) | ADR-007 | Принято |
| Q-008 | Retention (365d) | ADR-008 | Принято |
| Q-009 | Metrics set (Prometheus) | ADR-009 | Принято |
| Q-010 | Failure handling (manual retry) | ADR-010 | Принято |
| Q-011 | Validation engine (YAML reuse) | ADR-011 | Принято |
| Q-012 | Bi-temporal recompute | ADR-012 | Отложено |
| Q-013 | applicable_rule_id ownership | ADR-013 | Принято |
| Q-014 | etl_run_id + registry | ADR-014 | Принято |
| Q-015 | Quality threshold (1%) | ADR-015 | Принято |
| Q-016 | JWT для ETL (x-flow-etl) | ADR-016 | Принято |
| Q-017 | Read mode (full read) | ADR-017 | Принято |
| Q-018 | Logs/observability (slog + Prom) | ADR-018 | Принято |
| Q-019 | Tests (dockertest + Suite) | ADR-019 | Принято |
| Q-020 | Migrations ownership (внутри feature) | ADR-020 | Принято |
| Q-021 | supplier_scorecard ondemand trigger | ADR-021 | Принято |
| Q-022 | Admin auth (admin-cli) | ADR-022 | Принято |
| Q-023 | PG read-only role mart_reader | ADR-023 | Принято |
| Q-024 | applicable_rule_id resolution priority | ADR-024 | Принято |
| Q-025 | Stale ETL run timeout (1h) | ADR-025 | Принято |

| Мета-ADR | Тема | Статус |
|---|---|---|
| ADR-100 | Стек: Go 1.26 / Fiber v3 / pgx / net/http (без новых deps) | Принято |
| ADR-101 | Dockerfile: отдельный `Dockerfile.etl` | Принято |
| ADR-102 | DI-корень: `internal/etlapp` (а не общий `internal/app`) | Принято |
| ADR-103 | Validation engine — импорт пакета Модуля 1 (без копии) | Принято |
| ADR-104 | Scheduler: gocron/v2 + advisory lock как защита L2 | Принято |
| ADR-105 | Партиции: maintenance в Go (без pg_partman) | Принято |
| ADR-106 | DSN: единый `ETL_DB_DSN` (или общий `DB_DSN` в compose) | Принято |
| ADR-107 | Sentinel-prefix `EV-` (Etl-Validation) для support_codes | Принято |

---

## ADR-001 — Binary (Q-001)

**Статус:** Принято
**Отвечает на Q-001:** «Где запускается ETL — отдельный binary `cmd/etl/main.go` или в `cmd/source-adapter` как доп.scheduler-job?»

**Контекст:** ETL имеет свой cron schedule (02:30 vs 02:00 у source-adapter), свои метрики, свои migrations и admin-endpoints. Делить процесс — значит мешать observability двух ответственностей.

**Решение:** Отдельный бинарь `cmd/etl/main.go` + DI-корень `internal/etlapp`. Импортирует Модуль 1 как библиотеку (validation engine, errorspkg, middleware). Свой docker-compose service.

**Альтернативы:**
- Подключить как дополнительный scheduler-job в `cmd/source-adapter/main.go` (отклонено: mixed concerns).
- Делать Lambda/serverless (отклонено: out of scope MVP, нужна инфраструктура).

**Риски:** Дополнительная стоимость деплоя (доп. контейнер). Mitigation — общий PG, общий compose.

---

## ADR-002 — Feature folder (Q-002)

**Статус:** Принято
**Отвечает на Q-002:** «Имя feature-папки.»

**Контекст:** Контракт витрин называет модуль «X-Flow ETL» (внешнее), draft-plan — «ETL и валидация». Внутреннее имя должно быть стабильным и уникальным.

**Решение:** `internal/features/etl_validation/` (snake_case консистентно с `data_export`).

**Альтернативы:**
- `data_etl` (отклонено: конфликтует с `data_export` визуально).
- `xflow_etl` (отклонено: внешнее имя ≠ внутренний пакет).

**Риски:** Нет.

---

## ADR-003 — Trigger (Q-003)

**Статус:** Принято
**Отвечает на Q-003:** «Триггер запуска: own cron, polling, webhook, или LISTEN/NOTIFY?»

**Контекст:** source-adapter `committed` примерно к 02:30 (cron 02:00 + ~30 min). Триггер должен запускаться после, без слишком тонкой синхронизации.

**Решение:** Own cron `02:30 Europe/Kyiv` через `gocron/v2`, configurable через env `ETL_CRON_SCHEDULE` и `ETL_TZ`. Если `source-adapter` ещё не committed — extractor получит 503 snapshot_not_ready, run abort'нется, cron повторит на следующий tick.

**Альтернативы:**
- LISTEN/NOTIFY — отклонено: связывает два компонента через PG-канал, повышает coupling.
- Webhook от source-adapter — отклонено: новая зависимость в Модуле 1, не за пределами MVP.
- Polling /v1/snapshots/current каждые N минут — отклонено: лишняя нагрузка.

**Риски:** Если source-adapter SLA сдвинется — нужно подкорректировать `ETL_CRON_SCHEDULE`.

---

## ADR-004 — Idempotency (Q-004)

**Статус:** Принято
**Отвечает на Q-004:** «Использовать ту же advisory-lock семантику и `etl_runs` registry?»

**Контекст:** Защита от двойного запуска: cron-tick совпал с `POST /admin/etl-runs`, или admin вызвал retry дважды.

**Решение:** PG advisory lock на ключ `'etl-run'` (`pg_try_advisory_lock(hashtextextended('etl-run', 0))`) + таблица `marts.etl_runs(id, status, source_load_id, started_at, ...)` (см. `design-sql.md`). Status ∈ `{running, committed, failed, aborted}`. Lock освобождается на любом завершении.

**Альтернативы:** Redis lock (отклонено: новая инфра-зависимость).

**Риски:** Stale run (см. ADR-025).

---

## ADR-005 — Atomic snapshot read (Q-005)

**Статус:** Принято
**Отвечает на Q-005:** «Зафиксировать `source_load_id` на весь run или допустить разные load_id для entity?»

**Контекст:** Если разные entity читаются с разными load_id — теряется ссылочная целостность. `mart_calculation_input.product_id` может ссылаться на product, не существовавший в snapshot, использованном для receipt_line.

**Решение:** В начале run-а вызвать `GET /v1/snapshots/current`, зафиксировать `current_load_id` в `etl_runs.source_load_id`. Все последующие `GET /v1/{entity}` вызываются с query-параметром `?snapshot=<source_load_id>`. Все mart-строки несут этот же `source_load_id` (provenance).

**Альтернативы:** Per-entity snapshot (отклонено: см. контекст).

**Риски:** Если source-adapter сделает atomic flip пока ETL качает данные — extractor будет читать «старый» snapshot (это и есть желаемое поведение).

---

## ADR-006 — Marts location (Q-006)

**Статус:** Принято
**Отвечает на Q-006:** «Где живут `mart_*` — та же БД (schema), отдельная БД с FDW, отдельный кластер?»

**Контекст:** Контракт §4.5 spec-replenishment допускает «в той же БД либо через FDW/реплику». MVP — минимум инфраструктуры.

**Решение:** Та же БД, отдельная schema `marts`. Read-only role `mart_reader` для Replenishment + KPI Модулей.

**Альтернативы:**
- Отдельная БД с FDW (отклонено для MVP: две БД для on-call, миграции усложняются).
- Отдельный кластер (отклонено: не оправдано до prod-нагрузок).

**Риски:** Хот-данные и аналитика делят один pool. Mitigation — раздельные read-only credentials, мониторинг slow queries.

---

## ADR-007 — Partitioning (Q-007)

**Статус:** Принято
**Отвечает на Q-007:** «Партиционирование mart_*.»

**Решение:** RANGE по `as_of_date`, размер партиции — 1 месяц. Партиционируются `mart_demand_history` и `mart_kpi_daily`. `mart_calculation_input` и `mart_master_current` — current snapshot (не партиционируется). `mart_supplier_scorecard` — rolling weekly (не партиционируется).

**Альтернативы:** Дневные партиции (отклонено: 365 партиций — overkill). LIST по location (отклонено: cardinality слишком высокий).

**Риски:** Партиции создаются Go-кодом (ADR-105). При росте retention partition_maintenance Job должен учесть.

---

## ADR-008 — Retention (Q-008)

**Статус:** Принято
**Отвечает на Q-008:** «Глубина истории mart_demand_history.»

**Решение:** 365 дней (1 год). Partition_maintenance Job DROP'ает партиции старше `now() - 365d`. Cold-archive (Parquet/S3) — out of scope MVP.

**Альтернативы:** 730d (отклонено: удваивает storage; продукт может пересмотреть в фазе 2).

**Риски:** Прогнозные модели Replenishment могут требовать > 1y истории. Mitigation — конфигурируется через env `ETL_RETENTION_DAYS` (TODO ADR-008b при необходимости).

---

## ADR-009 — Metrics set (Q-009)

**Статус:** Принято
**Отвечает на Q-009:** «Какой набор метрик обязателен на MVP.»

**Решение:** см. таблицу в `design-integrations.md` §4. Ключевые: `etl_run_duration_seconds` (histogram), `etl_run_success_total` / `etl_run_failed_total{reason}`, `etl_lines_processed_total{entity}`, `etl_lines_failed_total{entity, severity}`, `mart_rows_total{mart}`, `etl_lag_seconds`, `etl_extractor_request_seconds`, `etl_advisory_lock_held_seconds`.

**Альтернативы:** Только базовые (отклонено: не покрывает SLA freshness).

**Риски:** cardinality — `entity` ограничено известным списком (~10), `severity` ∈ 2.

---

## ADR-010 — Failure handling (Q-010)

**Статус:** Принято
**Отвечает на Q-010:** «Manual retry vs auto retry.»

**Решение:** Manual retry через `POST /admin/etl-runs/{id}/retry` (роль admin-cli). Retry создаёт новый etl_run row с `parent_run_id=<id>` и тем же `source_load_id`. Авто-retry на уровне pipeline отсутствует (только на уровне HTTP-клиента extractor — backoff cap 30s).

**Альтернативы:** Auto-retry до 3 раз (отклонено для MVP: auto-retry без обсервабилити приводит к молчаливым deadlock'ам).

**Риски:** On-call должен мониторить `etl_run_failed_total` метрику.

---

## ADR-011 — Validation engine (Q-011)

**Статус:** Принято
**Отвечает на Q-011:** «YAML-driven engine — переиспользуем engine из Модуля 1?»

**Решение:** Импортируем `internal/features/data_export/validation` как библиотечный пакет. ETL-builtin-чеки (`fk_exists`, `unique_business_key`, `aggregate_sum_matches`, `referential_integrity`, `null_required_field`) реализуются в `internal/features/etl_validation/validation/builtin_*.go` и регистрируются адаптером `engine_adapter.go`. Файл правил — `configs/etl_validation_rules.yaml`.

**Альтернативы:**
- Скопировать engine в `pkg/validation` (отклонено: дрейф двух копий неизбежен).
- Написать новый engine с нуля (отклонено: повторное изобретение).

**Риски:** Если в Модуле 1 захотят несовместимое изменение — нужно вынести engine в `pkg/validation` (новый ADR).

---

## ADR-012 — Bi-temporal recompute (Q-012)

**Статус:** Отложено
**Отвечает на Q-012:** «Recompute прошлых партиций при коррекции.»

**Контекст:** Контракт допускает запоздалые `master_change_log` / `receipt_line` за прошлые даты. MVP: append-only.

**Решение в design (заглушка):**
- Mart-таблицы спроектированы append-only с PK `(business_key)`. ON CONFLICT UPDATE будет обновлять текущие строки, но *прошлые* партиции — нет.
- Recompute прошлых дней — задача следующей итерации (нужно будет: range repartition + delete старых rows + новый INSERT, idempotent).

**Эскалация:** Продукт E-Zoo решает приоритет recompute vs. forecast accuracy (после первой итерации с реальными данными).

**Риски:** Если коррекций много — точность прогноза деградирует до recompute-итерации. Mitigation — мониторим метрику `etl_late_corrections_total{entity}` (TODO).

---

## ADR-013 — applicable_rule_id ownership (Q-013)

**Статус:** Принято
**Отвечает на Q-013:** «Кто вычисляет applicable_rule_id — source-adapter или ETL?»

**Решение:** ETL вычисляет в `transformer.calculation_input.go`. Логика — приоритет `order_rule > supply_spec` (см. ADR-024). Source-adapter отдаёт сырые `order_rule` и `supply_spec` без резолюции.

**Альтернативы:** Резолюция в source-adapter (отклонено: source-adapter — pure pass-through; бизнес-логика витрин не должна там жить).

**Риски:** Дублирование, если ещё какой-то клиент будет резолвить — но MVP только Replenishment, и он читает уже резолвленное `mart_calculation_input.applicable_rule_id`.

---

## ADR-014 — etl_run_id + registry (Q-014)

**Статус:** Принято
**Отвечает на Q-014:** «Идентификатор и registry для ETL run.»

**Решение:** `etl_runs.id UUID v4 PK`. Каждая mart-строка несёт `etl_run_id`. JSONB `marts_summary` агрегирует `{mart_name: {rows: N}}` — заполняется в финале commit.

**Альтернативы:** ULID/SnowflakeID (отклонено: UUID v4 — стандарт проекта, см. Модуль 1).

**Риски:** UUID 16 байт vs INT — небольшой overhead, окупается переносимостью.

---

## ADR-015 — Quality threshold (Q-015)

**Статус:** Принято
**Отвечает на Q-015:** «Порог fail для cross-entity validation.»

**Решение:** 1% (consistent с Модулем 1). `lines_failed_critical / lines_total > 0.01` → run failed, marts не flip-аются. Configurable через env `ETL_QUALITY_THRESHOLD_PCT`.

**Альтернативы:** 0.5% (отклонено: слишком жёстко для MVP). 5% (отклонено: данные Replenishment теряют надёжность).

**Риски:** Если ERP клиента имеет «грязные» данные >1% — все runs будут fail. Mitigation — soft-severity для редких/некритичных rules.

---

## ADR-016 — JWT для ETL (Q-016)

**Статус:** Принято
**Отвечает на Q-016:** «JWT-аутентификация для ETL → source-adapter API.»

**Решение:** ETL использует JWT с claim `role=x-flow-etl`. Подпись HS256 (default) с симметричным ключом `ETL_JWT_SIGNING_KEY` (env, либо secret-manager в prod). Возможен RS256 через `ETL_JWT_PUBLIC_KEY_PATH`/`ETL_JWT_PRIVATE_KEY_PATH`. `iss=x-flow-etl`, `aud=source-adapter`. Срок 5 минут, обновляется на каждый retry-цикл.

**Альтернативы:** mTLS (отклонено для MVP — больше операционной работы; можно добавить позже).

**Риски:** Утечка `ETL_JWT_SIGNING_KEY` ⇒ полный доступ к source-adapter API. Mitigation — short-lived токен, ротация ключа.

---

## ADR-017 — Read mode (Q-017)

**Статус:** Принято
**Отвечает на Q-017:** «Full read vs incremental.»

**Решение:** Full read каждый run (все entity полностью). Mart-таблицы пересобираются (`mart_calculation_input` и `mart_master_current` через TRUNCATE+INSERT, остальные через INSERT ON CONFLICT). Incremental по cursor — out of scope MVP.

**Альтернативы:** Incremental (отклонено: усложняет и source-adapter, и ETL; объёмы пока умеренные).

**Риски:** Время run при росте объёма данных. Mitigation: SLA freshness <30 min проверяется метрикой; если деградирует — incremental в фазе 2.

---

## ADR-018 — Logs/observability (Q-018)

**Статус:** Принято
**Отвечает на Q-018:** «slog JSON + Prometheus + Grafana.»

**Решение:** `slog` JSON handler. Контекстные поля: `request_id`, `etl_run_id`, `source_load_id`, `entity`, `mart`, `status_code`, `duration_ms`, `requester`. Prometheus exposition на отдельном порту `:9091`. Grafana дашборд X-Flow расширяется секцией ETL (см. `design-infrastructure.md` §6).

**Альтернативы:** Логи в файл (отклонено: stdout — стандарт контейнеров).

**Риски:** PII в логах — ETL читает мастер-данные клиентов; надо избегать логирования payload-ов целиком (только id + status). Mitigation — `engine_adapter` логгит только `business_key` и `rule_id`, не value.

---

## ADR-019 — Tests (Q-019)

**Статус:** Принято
**Отвечает на Q-019:** «Тесты — dockertest, общий Suite через `pkg/dockertestpkg`.»

**Решение:** см. `design-tests.md`. Unit + integration (build tag `integration`) + golden + e2e in-process (httptest для source-adapter, dockertest PG).

**Альтернативы:** Только unit (отклонено: ETL — про SQL и интеграции, unit-mock не покрывает баги).

**Риски:** dockertest требует Docker daemon в CI. Mitigation — `make test-integration` пропускается при отсутствии `DOCKER_HOST`.

---

## ADR-020 — Migrations ownership (Q-020)

**Статус:** Принято
**Отвечает на Q-020:** «Где живут migrations.»

**Решение:** В `internal/features/etl_validation/sqls/migrations/` (внутри feature). Нумерация с `1001` чтобы не конфликтовать с миграциями Модуля 1 (`0001..0099`). Запуск — `make migrate-up-etl` (см. `design-infrastructure.md` §4). Migrations НЕ применяются автоматически при старте бинаря — ops запускает их явно.

**Альтернативы:**
- Общая папка `migrations/` (отклонено: feature должна быть автономна).
- Auto-apply при старте (отклонено: безопасность; ops должны контролировать когда мигрировать).

**Риски:** Координация двух наборов миграций (Модуль 1 + Модуль 2) — но т.к. они в разных номерах и schema (`public` vs `marts`), конфликтов нет.

---

## ADR-021 — supplier_scorecard ondemand trigger (Q-021)

**Статус:** Принято
**Отвечает на Q-021:** «Как триггерить ondemand refresh `mart_supplier_scorecard`.»

**Решение:** `POST /admin/marts/{name}/refresh` (роль `admin-cli`). Поддерживается ТОЛЬКО `name=mart_supplier_scorecard`. Для остальных — `ErrMartRefreshNotSupported` (HTTP 400). Endpoint берёт тот же advisory lock; если daily run уже идёт — 409 Conflict.

**Альтернативы:** Отдельный cron 06:00 (rolling weekly + ondemand) — отклонено: ondemand нужен после receiving_details обновлений, не привязан к расписанию.

**Риски:** Спам ondemand refresh — Mitigation: rate-limit на nginx (out of scope MVP), либо метрика `etl_ondemand_refresh_total` для алертинга.

---

## ADR-022 — Admin auth (Q-022)

**Статус:** Принято
**Отвечает на Q-022:** «JWT role для admin-endpoint-ов ETL.»

**Решение:** `admin-cli` для write-endpoints (`POST /admin/etl-runs`, `POST /admin/etl-runs/{id}/retry`, `POST /admin/marts/{name}/refresh`, `GET /admin/reject-log`). `admin-cli` ИЛИ `it-read` для read-endpoints (`GET /admin/etl-runs/{id}`, `GET /admin/etl-runs`). Реализация — reuse `internal/middleware/jwt` + `internal/middleware/role`.

**Альтернативы:** Отдельная роль `etl-admin` (отклонено: множить роли, когда `admin-cli` уже покрывает).

**Риски:** `admin-cli` — широкая роль; в фазе 2 можно сузить.

---

## ADR-023 — PG read-only role mart_reader (Q-023, новый)

**Статус:** Принято
**Отвечает на Q-023:** «PG read-only role `mart_reader` для Replenishment + KPI.»

**Решение:** Migration 1001 создаёт `mart_reader` (`NOLOGIN` — только grant target). `GRANT USAGE ON SCHEMA marts`, `GRANT SELECT` на 5 mart-таблиц + `etl_runs`. **НЕ** даёт grant на `reject_log` и `audit_access` (это операционные данные ETL, не для аналитики). `ALTER DEFAULT PRIVILEGES IN SCHEMA marts GRANT SELECT ON TABLES` — чтобы новые партиции автоматически inherit грант.

Owner role — superuser PG-кластера (например, `e_zoo_admin`). `mart_reader` сам никому ничего не grant'ит.

Конкретные пользователи (например, `replen_app`) создаются ops-командой и получают `GRANT mart_reader TO replen_app`.

**Альтернативы:** Прямой grant SELECT на каждую таблицу (отклонено: не инкапсулируется, новые таблицы выпадают из набора).

**Риски:** Нет — `NOLOGIN` role.

---

## ADR-024 — applicable_rule_id resolution priority (Q-024, новый)

**Статус:** Принято
**Отвечает на Q-024:** «Приоритет резолюции rule в `mart_calculation_input`.»

**Решение:** Приоритет `order_rule > supply_spec`. Если у пары `(product_id, location_id)` есть `order_rule` — берём его (с полями `formula`, `min_qty`, `max_qty`, `safety_stock`, …). Иначе — `supply_spec` (только `safety_stock`, `supplier_id`, `lead_time_days`). Если нет ни того, ни другого — `applicable_rule_kind='none'`, остальные поля NULL. Реализация — `mart_calculation_input_truncate_insert.sql` с CTE `rule_priority` + `DISTINCT ON` (см. `design-sql.md` §3.8).

**Альтернативы:** `supply_spec > order_rule` (отклонено: order_rule содержит конкретную formula, supply_spec — лишь договорные поля).

**Риски:** Если у одной пары есть несколько `order_rule` (collision) — `DISTINCT ON` выберет первый по сортировке `(product_id, location_id, prio)`, что недетерминировано без дополнительного `valid_from`. Mitigation — добавить `valid_from DESC` в ORDER BY когда source-adapter будет отдавать temporal-поле.

---

## ADR-025 — Stale ETL run timeout (Q-025, новый)

**Статус:** Принято
**Отвечает на Q-025:** «Таймаут зависшего running-run-а.»

**Решение:** Default `1h`, configurable через env `ETL_STALE_RUN_TIMEOUT`. Отдельный cron-job (каждые `5m`) сканирует `etl_runs WHERE status='running' AND started_at < now() - threshold` и помечает их как `aborted` (с `failure_reason='stale_timeout'`). Освобождает advisory lock через `pg_advisory_unlock_all()` в выделенной session (если возможно) или просто через `pg_terminate_backend(pid)` если зомби-pid известен.

**Альтернативы:** Auto-kill через PG `idle_in_transaction_session_timeout` (отклонено: ETL-run может быть законно длинным, не хочется убивать активный).

**Риски:** Если ETL run законно идёт > 1h — будет преждевременно abort'нут. Mitigation — мониторить и поднимать `ETL_STALE_RUN_TIMEOUT`.

---

# Мета-ADR

## ADR-100 — Стек: Go 1.26 / Fiber v3 / pgx / net/http без новых deps

**Статус:** Принято

**Контекст:** Модуль 1 фиксирует стек. Модуль 2 не должен расширять список зависимостей без необходимости.

**Решение:** Никаких новых deps относительно Модуля 1. HTTP-клиент — `net/http` (без `resty`/`heimdall`/etc).

**Альтернативы:** `resty` (отклонено: net/http достаточен).

**Риски:** Нет.

---

## ADR-101 — Dockerfile: отдельный `Dockerfile.etl`

**Статус:** Принято

**Контекст:** Два бинаря. Multi-target Dockerfile с ARG возможен, но отдельный файл проще для CI.

**Решение:** `Dockerfile.etl` (см. `design-infrastructure.md` §1).

**Альтернативы:** Multi-target single Dockerfile (приемлемо, но `make docker-build-etl` понятнее).

**Риски:** Дублирование build-инструкций между двумя Dockerfile-ами. Mitigation — общий Makefile.

---

## ADR-102 — DI-корень: `internal/etlapp`

**Статус:** Принято

**Контекст:** Делить `internal/app` Модуля 1 — мешать lifecycle двух бинарей.

**Решение:** Отдельный пакет `internal/etlapp/` (см. `design-di.md`).

**Альтернативы:** Один общий `internal/app` с подкомандами (отклонено: не модульно).

**Риски:** Нет.

---

## ADR-103 — Validation engine — импорт пакета Модуля 1

**Статус:** Принято

**Контекст:** см. ADR-011.

**Решение:** Импорт `internal/features/data_export/validation` как библиотеки. Локально регистрируем ETL-builtin checks через `engine_adapter.go`.

**Альтернативы:** Вынос в `pkg/validation` (откладываем до момента, когда engine начнёт расходиться между модулями).

**Риски:** Если Модуль 1 поменяет API engine — Модуль 2 сломается. Mitigation — стабильный экспорт + интеграционные тесты engine.

---

## ADR-104 — Scheduler: gocron/v2 + advisory lock как L2

**Статус:** Принято

**Контекст:** Один процесс — gocron singleton достаточен. Но при двух репликах процесса (HA) нужен distributed lock.

**Решение:** gocron/v2 (in-process) + PG advisory lock (cluster-wide). Если две реплики — обе тикнут, но только одна возьмёт advisory lock; вторая молча skip-нёт.

**Альтернативы:** Только gocron в одной реплике (отклонено: не выдержит HA).

**Риски:** Advisory lock держится session-wide; на restart освобождается автоматически (плюс).

---

## ADR-105 — Партиции: maintenance в Go (без pg_partman)

**Статус:** Принято

**Контекст:** `pg_partman` — внешний extension PG, требует install в кластере. MVP: не вводить новые extensions.

**Решение:** Job `mart_partition_maintenance` в Go-коде. Создаёт next-month партиции на 14 месяцев вперёд + DROP'ает старше 365d.

**Альтернативы:** pg_partman (отклонено для MVP). Manual via migrations (отклонено: не масштабируется).

**Риски:** Если Job не запустится — партиции «закончатся» через 14 месяцев. Mitigation — alert на отсутствие партиции для текущего месяца.

---

## ADR-106 — DSN: единый `ETL_DB_DSN` (или общий `DB_DSN` в compose)

**Статус:** Принято

**Контекст:** ETL и source-adapter живут в одном PG. Для локальной разработки удобно один DSN, для prod — отдельные пользователи (с разными правами).

**Решение:** Env var `ETL_DB_DSN` — required. В compose можно map'ить общий `DB_DSN` → `ETL_DB_DSN` через `${DB_DSN}`. В prod — отдельный PG-user `etl_writer` (с GRANT на `marts.*` + SELECT на `public.*`).

**Альтернативы:** Один общий env (отклонено: разные права).

**Риски:** Конфигурационная ошибка, если DSN указывает на чужой кластер. Mitigation — health-check на старте + миграционная проверка `SELECT current_database()`.

---

## ADR-107 — Sentinel-prefix `EV-` для support_codes

**Статус:** Принято

**Контекст:** Модуль 1 использует префикс `SA-` (Source-Adapter). Чтобы коды не пересекались.

**Решение:** Префикс `EV-` (Etl-Validation). См. таблицу в `design-errors.md` §1.

**Альтернативы:** Сквозная нумерация (отклонено: модульность).

**Риски:** Нет.
