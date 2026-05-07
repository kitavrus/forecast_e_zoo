# Design: source-adapter (Модуль 1 MVP — Адаптер источников)

> **Стадия:** 3 / 4 (Design). **Дата:** 2026-05-07.
> **Module path:** `github.com/Kitavrus/e_zoo`. **Feature package:** `data_export` (snake_case).
> **Greenfield:** репозиторий пуст; весь скелет создаётся с нуля.

---

## 0. Назначение документа

Этот файл — **точка входа** в design-пакет для фичи `source-adapter`. Все детали разнесены по
соседним 11 файлам. Здесь — только сводка ключевых решений и навигация.

## 1. Контекст одной строкой

`source-adapter` забирает (pull) суточный срез данных из ERP клиента (E-Zoo), валидирует, кладёт в
PostgreSQL 18 атомарным flip-ом snapshot и отдаёт downstream-потребителям (X-Flow ETL, IT E-Zoo,
Replenishment) через консистентный read-only REST с JWT-аутентификацией.

## 2. Навигация по design-пакету

| # | Файл | Что внутри |
|---|---|---|
| 1 | [design.md](design.md) | Этот файл — обзор и summary решений |
| 2 | [design-c4.md](design-c4.md) | C4 Level 1–4 (Mermaid) — контекст, контейнеры, компоненты, код |
| 3 | [design-dataflow.md](design-dataflow.md) | Общий dataflow + детальный для cron-load и для GET /v1/{entity} |
| 4 | [design-sequence-diagrams.md](design-sequence-diagrams.md) | Sequence для каждого endpoint + cron tick |
| 5 | [design-go-layers.md](design-go-layers.md) | Структура `internal/features/data_export/*` + DTO + интерфейсы |
| 6 | [design-sql.md](design-sql.md) | go:embed SQL + миграции golang-migrate (PG18 partitioning) |
| 7 | [design-tests.md](design-tests.md) | Unit + integration (dockertest postgres:18-alpine) |
| 8 | [design-di.md](design-di.md) | Сборка модуля в `internal/app.go` и `internal/routers/routers.go` |
| 9 | [design-integrations.md](design-integrations.md) | HTTP-клиент к ERP, Prometheus, slog, локальная FS |
| 10 | [design-errors.md](design-errors.md) | Sentinel-ошибки в `pkg/errorspkg`, supportMessage, mapping |
| 11 | [design-infrastructure.md](design-infrastructure.md) | docker-compose, Dockerfile, env-vars, метрики |
| 12 | [design-adr.md](design-adr.md) | ADR-001..016 строго по Q-NNN из spec-interview |

## 3. Summary архитектурных решений (без спорных вариантов)

| Тема | Решение | Источник |
|---|---|---|
| Стек | Go 1.26 + Fiber v3 + pgx/v5 (`pgxpool`) + go:embed + golang-migrate/v4 + dockertest/v3 | ADR-100 (мета) |
| Логгер | `log/slog` (стандартная библиотека, JSON handler) | ADR-100 (мета) |
| HTTP к ERP | `net/http` (стандартный) + кастомный retry-middleware | ADR-100 (мета) |
| Cron в процессе | `github.com/go-co-op/gocron/v2` | ADR-005 |
| Auth API адаптера | JWT HS256 (RS256 опционально) на `/v1/*` и `/admin/*` | ADR-101 (мета) |
| Auth к ERP | Интерфейс `SourceAuth` (impl зависит от Q-001/Q-002) | ADR-001 |
| Параллельный load | PG advisory lock на ключ `daily-load`, конфликт → 409 (без `force=true`) | ADR-006 |
| Snapshot consistency | Atomic flip `snapshot_pointer.current_load_id` в одной транзакции после успеха ВСЕХ сущностей | ADR-102 (мета) |
| Retry стратегии | Рестарт всего load-а оператором; HTTP-retry внутри одного запроса (max 3, exp backoff cap 30s, jitter 10%) | ADR-007 |
| Quality threshold | `lines_failed / lines_total > 1%` → load failed | ADR-003 |
| Inline NDJSON | <50 MB; больше — async export на local FS, выдача через Fiber static | ADR-009 (Q-009) |
| Хранилище exports | Local FS, абстракция через интерфейс `ExportsStorage` (для будущей замены на S3) | ADR-009 (Q-009) |
| Audit | Только `/admin/*` пишется в `audit_access`; `/v1/*` без audit | ADR-014 (Q-014) |
| Cron schedule default | `0 2 * * *` (02:00 Europe/Kyiv), ENV: `SOURCE_ADAPTER_CRON_SCHEDULE` + `SOURCE_ADAPTER_TZ` | ADR-005 |
| supplier_stock | Опциональная сущность; если ERP не отдаёт — load не fail-ается | ADR-010 (Q-010) |
| Multi-tenant / Web UI / S3 cold / CI | НЕ в MVP | ADR-103 (мета), ADR-011, ADR-012 |
| Партиционирование фактов | PG18 declarative partitioning по `event_date` (RANGE, по месяцам) | ADR-100 (мета) |
| `master_change_log` | Append-only diff-журнал по tracked-полям из YAML | ADR-100 (мета) |
| Lifecycle events DTO для store_assortment | `StoreAssortmentLifecycleEventResponse` (см. design-go-layers.md §3.1) | ADR-016 (Q-016) |

## 4. Что считаем «вне scope» MVP (зафиксировано как ADR «Отложено»)

- **CDC/Debezium** — только если объём >10M строк/сутки (ADR-004 / Q-004, эскалация: IT E-Zoo + продукт; pre-MVP замер).
- **S3 cold-слой 365d Parquet** (ADR-011 / Q-011, эскалация: продукт + IT E-Zoo).
- **EDI-профиль маршрутизации заказов** (ADR-013 / Q-013, передан в Модуль 7).
- **CI/CD pipeline** (ADR-012 / Q-012, эскалация: IT E-Zoo).
- **Multi-tenant** (ADR-103 / мета, эскалация: продукт).
- **Web UI для admin-операций** — отклонено в MVP (см. spec §Принятые компромиссы).

## 5. Готовность к Стадии 4 (swimlane HTML)

Все архитектурные слои покрыты. Swimlane-агент (HTML) на следующей стадии берёт за основу:
- роли акторов (cron, ERP, источник, validator, snapshot, потребители) — из [design-c4.md](design-c4.md) §L1
- порядок шагов load-а — из [design-dataflow.md](design-dataflow.md) §2
- состояния `loads.status` (running → committed | failed) — из [design-sql.md](design-sql.md) §`loads`

## 6. Открытые вопросы (16 → ADR-001..016)

См. [design-adr.md](design-adr.md). Кратко: 6 ADR закрыты решениями, 10 ADR помечены «Отложено»
с явной эскалацией ответственному (E-Zoo IT / E-Zoo продукт).
