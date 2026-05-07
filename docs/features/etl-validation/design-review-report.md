# Design Review Report: etl-validation

**Дата ревью:** 2026-05-07
**Ревьюер:** design-reviewer (Module 2)
**Артефакты:** `docs/features/etl-validation/`

---

## Блокеры

Нет.

---

## Серьёзные замечания

Нет.

---

## Незначительные замечания

1. **`design-c4.md` L4 §4.3 «validation (reuse Модуля 1)»** — текст ссылается на validation engine reuse, но не указывает явный путь импорта пакета. ADR-103 уточняет, что engine импортируется как пакет Модуля 1; рекомендуется в C4 L4 §4.3 добавить однострочную ссылку «import path: `internal/features/data_export/validation`» для синхронизации с ADR-103. Не блокер.

2. **`design-sql.md` §4** — упоминает rolling 14-month window партиций, но не даёт формальной формулы для `partition_maintenance` Job (только в `design-adr.md` ADR-105 «partition_maintenance в Go-коде»). Достаточно для Plan-стадии; уточнить в Plan/Phase.

3. **`design-errors.md` §1 «Reuse существующих»** — `ErrSnapshotNotReady` и `ErrQualityThresholdExceeded` помечены как reuse, но в Модуле 1 (`pkg/errorspkg`) их фактический присвоенный support_code не упомянут (SA-prefix vs EV-prefix). Уточнить при имплементации, чтобы не было коллизий с support_codes Модуля 1.

4. **`design-go-layers.md` §2.4 `Transformer`** — интерфейс описан, но не перечислены явно 5 mart-builders (есть только обобщённый метод). В §2.5 `Loader.MartName` enum указывает 5 имён, что компенсирует. Для строгости предложить сделать 5 концретных методов на `Transformer` или константу-список.

5. **Согласование env var `ETL_CRON_SCHEDULE`:** в spec и design.md написано «02:30 Europe/Kyiv», в `design-infrastructure.md` §2 default = `30 2 * * *`. Это эквивалент. Подтверждено.

---

## Покрытие открытых вопросов (Q-NNN ↔ ADR-NNN)

Полное 1:1 покрытие 25/25.

| Q | Тема | ADR | Статус |
|---|---|---|---|
| Q-001 | Binary | ADR-001 | Принято |
| Q-002 | Feature folder | ADR-002 | Принято |
| Q-003 | Trigger | ADR-003 | Принято |
| Q-004 | Idempotency | ADR-004 | Принято |
| Q-005 | Atomic snapshot read | ADR-005 | Принято |
| Q-006 | Marts location | ADR-006 | Принято |
| Q-007 | Partitioning | ADR-007 | Принято |
| Q-008 | Retention | ADR-008 | Принято |
| Q-009 | Metrics set | ADR-009 | Принято |
| Q-010 | Failure handling | ADR-010 | Принято |
| Q-011 | Validation engine | ADR-011 | Принято |
| Q-012 | Bi-temporal recompute | ADR-012 | Отложено |
| Q-013 | applicable_rule_id ownership | ADR-013 | Принято |
| Q-014 | etl_run_id + registry | ADR-014 | Принято |
| Q-015 | Quality threshold (1%) | ADR-015 | Принято |
| Q-016 | JWT для ETL (x-flow-etl) | ADR-016 | Принято |
| Q-017 | Read mode (full read) | ADR-017 | Принято |
| Q-018 | Logs/observability | ADR-018 | Принято |
| Q-019 | Tests | ADR-019 | Принято |
| Q-020 | Migrations ownership | ADR-020 | Принято |
| Q-021 | supplier_scorecard ondemand | ADR-021 | Принято |
| Q-022 | Admin auth (admin-cli) | ADR-022 | Принято |
| Q-023 | PG read-only role mart_reader | ADR-023 | Принято |
| Q-024 | applicable_rule_id resolution priority | ADR-024 | Принято |
| Q-025 | Stale ETL run timeout (1h) | ADR-025 | Принято |

**Мета-ADR (100+):** ADR-100 (стек), ADR-101 (Dockerfile), ADR-102 (DI-корень `etlapp`), ADR-103 (validation reuse), ADR-104 (scheduler L1+L2), ADR-105 (partition maintenance в Go), ADR-106 (DSN), ADR-107 (sentinel-prefix `EV-`). Всего 8.

**Итого:** 25 ADR (Q-001..Q-025) + 8 мета-ADR = 33 ADR. Каждый Q закрыт ADR.

---

## Полнота файлов

12 design-*.md + 1 swimlane.html — все на месте.

| Файл | Размер | Статус |
|---|---|---|
| `design.md` | 16 709 байт | ✅ |
| `design-c4.md` | 7 803 | ✅ (L1+L2+L3+L4) |
| `design-dataflow.md` | 5 798 | ✅ |
| `design-sequence-diagrams.md` | 8 596 | ✅ (8 диаграмм) |
| `design-go-layers.md` | 12 210 | ✅ |
| `design-sql.md` | 21 671 | ✅ (миграции 1001/1002 + 12 SQL queries) |
| `design-tests.md` | 9 889 | ✅ (10 разделов) |
| `design-di.md` | 10 509 | ✅ |
| `design-integrations.md` | 9 478 | ✅ |
| `design-errors.md` | 5 672 | ✅ |
| `design-infrastructure.md` | 7 768 | ✅ |
| `design-adr.md` | 33 202 | ✅ (33 ADR) |
| `design-swimlane.html` | 30 999 | ✅ (744 строк, ETL & Validation) |

---

## Консистентность

### 1. Решения spec явно отражены

| Требование spec | Где зафиксировано | OK |
|---|---|---|
| `cmd/etl/main.go` отдельный binary | ADR-001, design.md §0/§4, design-di.md §1, design-c4.md L2 | ✅ |
| Cron 02:30 Europe/Kyiv | ADR-003, design-infrastructure.md §2 (`ETL_CRON_SCHEDULE=30 2 * * *`, `ETL_TZ=Europe/Kyiv`) | ✅ |
| Schema `marts` (та же БД) | ADR-006, design-sql.md §1 (`CREATE SCHEMA marts`), design-c4.md L2 | ✅ |
| Full read per snapshot | ADR-017, design-dataflow.md §1, design.md §0 | ✅ |
| YAML validation engine reuse Модуля 1 | ADR-011, ADR-103, design-go-layers.md §5, design-c4.md L4 §4.3 | ✅ |
| Manual retry | ADR-010, design.md §7 (`POST /admin/etl-runs/{id}/retry`), design-sequence-diagrams.md §2 | ✅ |
| `applicable_rule_id` вычисляет ETL | ADR-013, ADR-024 (priority `order_rule > supply_spec`), design-sql.md §3.8 | ✅ |
| Quality threshold 1% | ADR-015, design-infrastructure.md §2 (`ETL_QUALITY_THRESHOLD_PCT=1.0`), design-dataflow.md §3 | ✅ |
| JWT `x-flow-etl` для запросов к source-adapter | ADR-016, design-integrations.md §2.1 (claims `role: x-flow-etl`) | ✅ |
| Audit только `/admin/*` | design.md §2 + design-sequence-diagrams.md §1 (audit middleware на admin endpoints), design-sql.md §2 (`audit_access` table) | ✅ |
| 5 mart-таблиц с правильным партиционированием | design-sql.md §1: `mart_demand_history` (RANGE month), `mart_kpi_daily` (RANGE month), `mart_calculation_input` (current), `mart_master_current` (current), `mart_supplier_scorecard` (rolling weekly) | ✅ |

### 2. DTO в go-layers ↔ sequence-diagrams

`EtlPipeline.Run/RunOptions`, `Snapshot`, `Violation` — присутствуют в `design-go-layers.md` §2 и используются в sequence-diagrams §0/§1/§2 с теми же названиями. ✅

### 3. SQL columns ↔ migrations

Все 5 mart-таблиц в `design-sql.md` §1 содержат обязательные provenance-поля `etl_run_id` + `source_load_id`, индексы по ним созданы. SQL queries §3.7–3.11 ссылаются на те же колонки. ✅

### 4. Sentinel errors ↔ handlers ↔ tests

| Sentinel | design-errors.md | handler mapping | tests матрица §4 |
|---|---|---|---|
| `ErrEtlRunAlreadyRunning` | ✅ EV-001/409 | ✅ `errorspkg.WriteJSON` | ✅ |
| `ErrEtlRunNotFound` | ✅ EV-002/404 | ✅ | ✅ |
| `ErrCannotRetryEtl` | ✅ EV-003/409 | ✅ | ✅ |
| `ErrSourceUnavailable` | ✅ EV-004/502 | ✅ | ✅ |
| `ErrMartRefreshNotSupported` | ✅ EV-005/400 | ✅ | ✅ |
| `ErrSnapshotNotReady` (reuse) | ✅ 503 | ✅ | ✅ |
| `ErrQualityThresholdExceeded` (reuse) | ✅ внутрипроцессно | ✅ | ✅ |

5 новых EV-* sentinels + 2 reused — полное покрытие. ✅

### 5. Env vars (ETL_*) в infrastructure ↔ integrations

`ETL_DB_DSN`, `ETL_API_BASE_URL`, `ETL_HTTP_TIMEOUT`, `ETL_RETRY_MAX`, `ETL_RETRY_BACKOFF_CAP`, `ETL_CRON_SCHEDULE`, `ETL_TZ`, `ETL_STALE_RUN_TIMEOUT`, `ETL_QUALITY_THRESHOLD_PCT`, `ETL_VALIDATION_RULES_PATH`, `ETL_JWT_*`, `ETL_METRICS_ADDR`, `ETL_LOG_LEVEL`, `ETL_HTTP_ADDR` — синхронизированы в `design-infrastructure.md` §2 и `design-integrations.md` §1.4/§2.1/§5/§6. ✅

### 6. Интерфейсы (extractor / transformer / loader / validator)

| Интерфейс | Где описан | Состав |
|---|---|---|
| `extractor.Extractor` (SnapshotsClient + EntitiesClient) | design-go-layers.md §2.2 | `GetCurrentSnapshot`, `Stream(entity, snapshotID)` ✅ |
| `validation.Engine` (reuse) | design-go-layers.md §2.3, ADR-103 | `Check(entity, payload, state)` ✅ |
| `transformer.Transformer` | design-go-layers.md §2.4 | один метод `Build(martName, ...)` — компенсируется `Loader.MartName` enum (5 значений) ✅ |
| `loader.Loader` | design-go-layers.md §2.5 | `Load(martName, etlRunID, sourceLoadID)` + `MartName` enum (5 шт) ✅ |
| `service.AdvisoryLock` | design-go-layers.md §2.6 | `Try(key)`, `Unlock(key)` ✅ |
| `service.EtlPipeline` | design-go-layers.md §2.7 | `Run(ctx, RunOptions)` ✅ |

5 mart-builders покрыты через `Loader.MartName` enum: `mart_demand_history`, `mart_calculation_input`, `mart_kpi_daily`, `mart_master_current`, `mart_supplier_scorecard`. ✅

### 7. Tests sentinel↔test матрица

`design-tests.md` §4 покрывает 5 новых EV-* sentinels (по 1–2 уровня тестов: service + handler) и 2 reused (`ErrSnapshotNotReady`, `ErrQualityThresholdExceeded`). Golden фикстуры для агрегаций (`design-tests.md` §3) присутствуют. ✅

### 8. Migrations

`design-sql.md` §1–§2 явно указывают:
- `internal/features/etl_validation/sqls/migrations/1001_marts_schema.up.sql` (schema marts + 5 mart-таблиц + RANGE partitions)
- `internal/features/etl_validation/sqls/migrations/1002_etl_runs.up.sql` (etl_runs + reject_log + audit_access + GRANTs для mart_reader)
- Префикс `1xxx` (Модуль 1 занимает `0001..0099`) ✅
- Запуск через `make migrate-up-etl` (design-infrastructure.md §4) ✅

---

## Итог: APPROVED

Все 25 Q-NNN покрыты ADR 1:1. Все 11 ключевых решений spec-interview явно зафиксированы. 12 design-*.md + 1 swimlane.html присутствуют. Sentinel errors (5 новых EV-* + 2 reused), DTO, SQL columns, env vars, интерфейсы, миграции — консистентны между файлами. Незначительные замечания (5 шт) не блокируют переход в Plan-стадию.

**Готовность к Plan: высокая.**
