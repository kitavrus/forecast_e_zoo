# Spec: etl-validation
**Дата интервью:** 2026-05-07
**На основе:** docs/features/etl-validation/research/output.md + docs/features/source-adapter/spec-interview/output.md + docs/features/source-adapter/design.md (паттерны) + docs/tmp/data-marts/contract-2026-05-06.md + docs/tmp/data-export/spec-2026-05-05.md + docs/tmp/replenishment/spec-2026-05-06.md

> **Контекст:** Модуль 2 «ETL и валидация» из MVP-пайплайна e_zoo. Реализуется в репозитории `github.com/Kitavrus/e_zoo` поверх Модуля 1 (`source-adapter`, уже завершён). Стек тот же: Go 1.26 + Fiber v3 + pgx/v5 + go:embed + golang-migrate + dockertest. Frontend не применим (как и в Модуле 1).

---

## Проблема и цель

Модуль 1 (`source-adapter`) сохраняет «сырые» сущности из ERP в hot PG (схема `public`), отдавая их через REST/NDJSON. Витрины (Модуль 3) и replenishment-калькулятор (Модуль 5) НЕ должны напрямую читать сырые сущности — им нужны:
- агрегации (`mart_demand_history`),
- денормализация (`mart_calculation_input` с pre-resolved supply_spec/order_rule),
- KPI-дайджест (`mart_kpi_daily`),
- master-snapshot для калькулятора (`mart_master_current`),
- supplier scorecard (`mart_supplier_scorecard`).

**Цель Модуля 2** — превратить «сырые» сущности source-adapter в 5 mart-таблиц, гарантировать консистентность через cross-entity validation, фиксировать каждую строку с `etl_run_id` и `source_load_id` (provenance). Запускается отдельным cron'ом сразу после успешного daily load source-adapter.

**Метрики успеха (3 SLA):**
1. **ETL freshness:** `etl_runs.committed_at < 03:30 Europe/Kyiv` в 99% дней (после source-adapter SLA 06:00 — реально 02:30 + ETL run ≤ 30 минут).
2. **Marts completeness:** `mart_demand_history`, `mart_calculation_input`, `mart_kpi_daily`, `mart_master_current` обновлены к 04:00 Europe/Kyiv в ≥98% дней.
3. **Cross-entity quality:** ETL `lines_failed/lines_total < 0.5%` (жёсткий fail >1%, как у Модуля 1).

---

## Пользователи (потребители контракта)

| Тип | Что делает | Через что |
|---|---|---|
| **Replenishment калькулятор (Модуль 5)** | Читает `mart_calculation_input`, `mart_master_current`, `mart_demand_history` | прямой SQL-доступ к PG (read-only role) |
| **KPI-модуль (Модуль 4)** | Читает `mart_kpi_daily`, `mart_supplier_scorecard` | прямой SQL |
| **DevOps / on-call X-Flow** | Перезапуск failed ETL run, ondemand refresh `mart_supplier_scorecard` | `POST /admin/etl-runs`, `POST /admin/etl-runs/{id}/retry`, `POST /admin/marts/{name}/refresh`, `GET /admin/etl-runs/{id}` |
| **IT E-Zoo (read-only)** | audit просмотра ETL действий | `audit_access` (отдельная таблица или общая с Модулем 1) |

**Owner модуля:** команда X-Flow.

---

## Сценарии использования

### Happy path (суточный ETL run)
1. Cron срабатывает 02:30 Europe/Kyiv (configurable через `ETL_CRON_SCHEDULE`).
2. ETL берёт PG advisory lock на ключ `etl-run`. Если занят — выходит.
3. Создаёт запись `etl_runs(id=uuidv4, started_at, status='running', source_load_id=NULL)`.
4. Вызывает `GET /v1/snapshots/current` API source-adapter (JWT с ролью `x-flow-etl`). Получает `current_load_id` — фиксирует в `etl_runs.source_load_id` (atomic read).
5. Скачивает все master-сущности через `GET /v1/{entity}` (NDJSON streaming, ETag, JWT) и пишет во временные staging-таблицы (`stg_*`).
6. Прогоняет `cross_validation_rules.yaml` (severity engine — переиспользуется из Модуля 1, но с другим YAML и набором builtin checks: `fk_exists`, `unique_business_key`, `aggregate_sum_matches`, `referential_integrity`).
7. Строит mart-таблицы (5 штук) через SQL-INSERT-FROM-SELECT с агрегациями. Каждая строка получает `etl_run_id`, `source_load_id` (provenance).
8. Atomic flip: в одной транзакции пометить новые партиции как «текущие» / удалить старые `etl_run_id` строки (или просто INSERT — append-семантика per контракту §3.1).
9. Проверка quality threshold: `lines_failed/lines_total < 1%`. Если ОК — `etl_runs.status='committed', committed_at=now()`. Освобождает lock.

### Edge cases
| # | Случай | Поведение |
|---|---|---|
| E1 | source-adapter API недоступен (503/таймаут) | ETL run failed, alert. Ручной retry через admin. |
| E2 | source-adapter snapshot не готов (503 snapshot_not_ready) | ETL run skipped (не failed). Метрика `etl_skipped_no_snapshot_total++`. Cron повторит на следующий tick. |
| E3 | Параллельный `POST /admin/etl-runs` пока крон уже идёт | 409 Conflict + ссылка на текущий `etl_run_id` (паттерн Модуля 1). |
| E4 | Доля cross-validation critical-строк >1% от total | ETL run failed, marts не flip-аются. Старые витрины остаются актуальными. |
| E5 | Дубликат business-key (например `(product_id, location_id, as_of_date)` в `mart_demand_history`) | Critical row → ETL `reject_log`. |
| E6 | FK violation: `mart_calculation_input.product_id` не существует в `products` | Critical row, fail если >1% от total. |
| E7 | Bi-temporal recompute: вчерашний день переоткрылся (запоздалый чек) | Q-NNN — отложен. MVP: append-only, recompute прошлых партиций — следующая итерация. |
| E8 | `mart_supplier_scorecard` ondemand refresh пока daily ETL run идёт | 409, ondemand ждёт окончания daily run. |

### Прерванный сценарий
- Процесс убит во время ETL run → `etl_runs.status='running'` стейл. При следующем cron tick — стейл-запись (>1ч) помечается `aborted`, новый run с нуля.

---

## Технические ограничения

| Ограничение | Решение |
|---|---|
| **Binary** | Отдельный `cmd/etl/main.go` (Q-001). Свой docker-compose service. |
| **Триггер** | Own cron 02:30 Europe/Kyiv (Q-003), env `ETL_CRON_SCHEDULE` configurable. |
| **Идемпотентность** | PG advisory lock + `etl_runs` registry (как `loads` в Модуле 1). |
| **Snapshot read** | Atomic: `source_load_id` фиксируется в начале run и используется для всех `GET /v1/{entity}` запросов (через query-параметр `?snapshot=<load_id>` если adapter поддерживает, иначе клиент-валидация что snapshot не сменился). |
| **Read mode** | Full read (Q-017) каждый run. Никаких cursor-deltas в MVP. |
| **Marts location** | Та же БД source-adapter, schema `marts` (Q-006). Создаётся миграцией Модуля 2 (отдельный набор миграций под префиксом 1xxx). |
| **Marts партиционирование** | RANGE по `as_of_date` месячные. Retention 365d (Q-007/008). 4 partitioned mart: `mart_demand_history`, `mart_kpi_daily`. Не партиционированы: `mart_master_current` (snapshot-only), `mart_calculation_input` (current snapshot), `mart_supplier_scorecard` (rolling weekly). |
| **Validation engine** | YAML-driven (Q-011), переиспользует severity-engine из Модуля 1 как пакет (или копия, если рефакторинг в `pkg/validation` потребует значительных изменений). Новый YAML: `configs/etl_validation_rules.yaml`. Builtin checks: `fk_exists`, `unique_business_key`, `aggregate_sum_matches`, `referential_integrity`, `null_required_field`. |
| **Quality threshold** | 1% (consistent с Модулем 1). |
| **Failure handling** | Ручной retry через `POST /admin/etl-runs/{id}/retry` (Q-010). Retry создаёт новый run с тем же `source_load_id`. |
| **applicable_rule_id** | Вычисляет ETL в `mart_calculation_input` (Q-013). Логика resolution — отдельная задача в design (priority order order_rule > supply_spec). |
| **HTTP клиент к API source-adapter** | net/http + JWT bearer + ETag/If-None-Match + retry с backoff cap 30s (как в Модуле 1). |
| **JWT** | Роль `x-flow-etl`, секрет/ключ через env `ETL_JWT_SIGNING_KEY` (HS256) либо `ETL_JWT_PUBLIC_KEY_PATH` (RS256). |
| **Admin auth** | `admin-cli` role JWT для `/admin/etl-runs/*`. |
| **Метрики** | `etl_run_duration_seconds`, `etl_run_success_total`, `etl_run_failed_total{reason}`, `etl_lines_processed_total{entity}`, `etl_lines_failed_total{entity,severity}`, `mart_rows_total{mart}`, `etl_lag_seconds` (от source-adapter committed → ETL committed), `etl_runs_skipped_total{reason}`. |
| **Логи** | slog JSON, контекст-поля `etl_run_id`, `source_load_id`, `mart`. |
| **Tests** | dockertest postgres:18-alpine, базовый Suite общий с Модулем 1 (вынесен в `pkg/dockertestpkg`). Golden-фикстуры для агрегаций. |
| **Migrations** | `internal/features/etl_validation/sqls/migrations/` (`1001_marts_schema.up.sql`, `1002_etl_runs.up.sql`). `make migrate-up-etl`. |

---

## Безопасность и доступ

| Что | Как |
|---|---|
| **Auth API source-adapter** | JWT с ролью `x-flow-etl`, secret/key через env. Токен подписывается тем же ключом, что и токены потребителей (HS256 общий). |
| **Auth /admin/etl-runs/\*** | JWT с ролью `admin-cli`. Middleware из `internal/middleware/role.go`. |
| **Чувствительные данные** | Нет PII. Креды только через env. |
| **Audit** | `audit_access` пишется ТОЛЬКО для `/admin/etl-runs/*` (как в Модуле 1). |
| **Read-only role в PG** | Создать отдельную PG-role `mart_reader` с GRANT SELECT на `marts.*`. Replenishment подключается этой ролью. |

---

## Обработка ошибок

| Ошибка | HTTP | code | Что видит | Действие системы |
|---|---|---|---|---|
| Нет JWT / невалидный | 401 | `auth_invalid_token` | `{code, message}` | — |
| Недостаточно прав | 403 | `auth_forbidden` | `{code, message}` | audit для admin |
| ETL run уже идёт | 409 | `etl_run_already_running` | `{code, message, currentEtlRunId}` | — |
| Snapshot not ready (source-adapter ещё не сделал load) | 503 | `snapshot_not_ready` | retry-after | skipped run в etl_runs |
| Несуществующий `etl_run_id` | 404 | `not_found` | — | — |
| Quality threshold violation | n/a | n/a (внутренний) | — | `etl_runs.status='failed', failed_reason='quality_threshold_exceeded'`. Marts не flip-аются. |
| ERP/source unavailable | n/a | n/a | — | `etl_runs.status='failed', failed_reason='source_unavailable'`. Alert. |
| Внутренняя 5xx | 500 | `internal` | — | log stacktrace |

Формат: `pkg/errorspkg.WriteJSON` (как в Модуле 1).

---

## Принятые компромиссы (MVP)

1. **Full read каждый run** (Q-017) — не incremental cursor. Объёмы пилота позволяют.
2. **Retry = рестарт всего run-а** — не per-mart partial restore.
3. **Bi-temporal recompute отложено** (Q-012) — append-only витрины. Recompute прошлых дней — next iter.
4. **5 mart-таблиц** — minimum viable набор по контракту. Дополнительные витрины — next iter.
5. **Без CDC**.
6. **Без отдельного БД-кластера** — schema `marts` в той же БД.
7. **`mart_supplier_scorecard` rolling weekly + ondemand** — refresh-логика только при ondemand POST или при cron тике (если прошла неделя с last_refresh).
8. **YAML validation engine переиспользуется** из Модуля 1 — копия пакета (рефакторинг в `pkg/validation` оставлен на следующий MVP-цикл).
9. **PG read-only role для marts** — создаётся миграцией, но пользователи (replenishment) подключаются вручную в их env.

---

## Отклонённые решения

| Что | Почему |
|---|---|
| Webhook от source-adapter к ETL | Связывает модули, требует endpoint POST /etl/notify. Cron 02:30 проще. |
| LISTEN/NOTIFY PostgreSQL | Завязка на PG-only архитектуру; усложняет тесты. |
| Polling | Лишний трафик на adapter. |
| Incremental cursor | Объёмы MVP позволяют full read; cursor усложняет recompute. |
| Auto-retry бесконечно | Риск infinite loop. Ручной retry безопаснее. |
| Отдельная БД для marts | Overengineering для MVP. |
| Replenishment вычисляет applicable_rule_id | Это бизнес-логика на уровне контракта витрин, лучше в ETL. |

---

## Открытые вопросы

> Каждый Q-NNN закрывается ADR-NNN на стадии Design.

1. **Q-001. Binary** — Принято (отдельный `cmd/etl/main.go`).
2. **Q-002. Feature folder** — Принято (`internal/features/etl_validation`).
3. **Q-003. Trigger** — Принято (own cron 02:30 Europe/Kyiv configurable).
4. **Q-004. Idempotency** — Принято (advisory lock + `etl_runs` registry).
5. **Q-005. Atomic snapshot read** — Принято (фиксация `source_load_id` в начале run).
6. **Q-006. Marts location** — Принято (та же БД, schema `marts`).
7. **Q-007. Partitioning** — Принято (RANGE month, mart_demand_history + mart_kpi_daily).
8. **Q-008. Retention** — Принято (365d).
9. **Q-009. Metrics set** — Принято (см. таблицу выше).
10. **Q-010. Failure handling** — Принято (manual retry).
11. **Q-011. Validation engine** — Принято (YAML-driven, переиспользуем engine из Модуля 1).
12. **Q-012. Bi-temporal recompute** — **Отложено** (append-only MVP, recompute next iter).
13. **Q-013. applicable_rule_id ownership** — Принято (ETL вычисляет).
14. **Q-014. etl_run_id + registry** — Принято (UUID v4 + `etl_runs` table).
15. **Q-015. Quality threshold** — Принято (1%).
16. **Q-016. JWT для ETL** — Принято (`x-flow-etl` role, env secret).
17. **Q-017. Read mode** — Принято (full read).
18. **Q-018. Logs/observability** — Принято (slog JSON + Prometheus + Grafana).
19. **Q-019. Tests** — Принято (dockertest, общий Suite через `pkg/dockertestpkg`).
20. **Q-020. Migrations ownership** — Принято (внутри feature, ops запускает migrate-up-etl).
21. **Q-021. supplier_scorecard ondemand trigger** — Принято (`POST /admin/marts/{name}/refresh` с ролью admin-cli).
22. **Q-022. Admin auth** — Принято (`admin-cli` JWT для всех `/admin/etl-*`).
23. **Q-023 (новый). PG read-only role `mart_reader`** — создаётся миграцией; список grant'ов и owner'а role — задача design.
24. **Q-024 (новый). applicable_rule_id resolution priority** — ETL выбирает rule по приоритету `order_rule > supply_spec` (фиксируется в design + примере SQL в `mart_calculation_input` builder).
25. **Q-025 (новый). Stale ETL run timeout** — default 1ч (`ETL_STALE_RUN_TIMEOUT`), env-var configurable. Аналогично Модулю 1.
