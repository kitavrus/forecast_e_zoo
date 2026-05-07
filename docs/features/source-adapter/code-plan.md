# Code Plan: source-adapter (Модуль 1 MVP — Адаптер источников)

> **Стадия:** 4 / 4 (Planning). **Дата:** 2026-05-07.
> **Module path:** `github.com/Kitavrus/e_zoo`. **Feature package:** `data_export`.
> **Greenfield:** репозиторий пуст; план создаёт скелет с нуля.

> Статусы выполнения — только в [code-plan-status.md](code-plan-status.md). В этом файле статусы НЕ ведутся.

---

## 1. Назначение

Этот документ — **главный обзор** плана реализации фичи `source-adapter`. Каждая фаза вынесена в отдельный файл `code-plan-phase-NN-*.md` и должна заканчиваться **атомарным коммитом**, проходящим `make build`.

**Жёсткий лимит:** 16 фаз. Каждая фаза самодостаточна, проходит компиляцию, не ломает предыдущие.

**Frontend в плане отсутствует** (MVP без UI, см. spec §3 «Users & use cases»).
**CI отсутствует** (Q-012 / ADR-012 «Отложено»). Все гейты — локальные `make`-команды.

---

## 2. Карта фаз

| # | Фаза | Файл | Commit-scope |
|---|---|---|---|
| 01 | Bootstrap проекта (go.mod, cmd, app, errorspkg, docker-compose, Makefile) | [code-plan-phase-01-bootstrap.md](code-plan-phase-01-bootstrap.md) | `chore(bootstrap)` |
| 02 | JWT middleware + role gating (`x-flow-etl`, `admin-cli`, `it-read`) | [code-plan-phase-02-jwt-middleware.md](code-plan-phase-02-jwt-middleware.md) | `feat(middleware)` |
| 03 | Migrations 0001 — master + service tables (17 таблиц, не партиционированные) | [code-plan-phase-03-migrations-master.md](code-plan-phase-03-migrations-master.md) | `feat(migrations)` |
| 04 | Migrations 0002 — facts partitioned by `event_date` (4 таблицы + initial RANGE-партиции) | [code-plan-phase-04-migrations-facts-partitioned.md](code-plan-phase-04-migrations-facts-partitioned.md) | `feat(migrations)` |
| 05 | Models / DTO для всех сущностей и admin-эндпоинтов | [code-plan-phase-05-models-dto.md](code-plan-phase-05-models-dto.md) | `feat(data_export/models)` |
| 06 | SQL-запросы (go:embed) — selects, advisory lock, snapshot flip, reject_log | [code-plan-phase-06-sql-queries.md](code-plan-phase-06-sql-queries.md) | `feat(data_export/sqls)` |
| 07 | Validators (формат-валидаторы запросов) + ValidatorEngine + `validation_rules.yaml` | [code-plan-phase-07-validators-engine-yaml.md](code-plan-phase-07-validators-engine-yaml.md) | `feat(data_export/validation)` |
| 08 | Repository (pgx + go:embed) + integration tests (dockertest postgres:18-alpine) | [code-plan-phase-08-repository.md](code-plan-phase-08-repository.md) | `feat(data_export/repository)` |
| 09 | SourceReader interface + `erp_e_zoo_reader` (in-memory stub MVP) | [code-plan-phase-09-source-reader-stub.md](code-plan-phase-09-source-reader-stub.md) | `feat(data_export/loader)` |
| 10 | Loader service: orchestration load-цикла + unit-тесты с mock SourceReader | [code-plan-phase-10-loader-service.md](code-plan-phase-10-loader-service.md) | `feat(data_export/loader)` |
| 11 | Snapshot service (atomic flip) + Audit access writer (`/admin/*`) | [code-plan-phase-11-snapshot-audit.md](code-plan-phase-11-snapshot-audit.md) | `feat(data_export/snapshot)` |
| 12 | Scheduler (gocron WithSingletonMode) + admin-handlers + integration test advisory-lock | [code-plan-phase-12-scheduler-admin-handlers.md](code-plan-phase-12-scheduler-admin-handlers.md) | `feat(data_export/scheduler)` |
| 13 | HTTP read-handlers `/v1/{entity}` + `/v1/snapshots/current` + healthz + NDJSON streaming + ETag | [code-plan-phase-13-read-handlers.md](code-plan-phase-13-read-handlers.md) | `feat(data_export/handler)` |
| 14 | ExportsStorage (local FS) + `POST /v1/exports` + `GET /v1/exports/{id}` + cleanup-cron | [code-plan-phase-14-exports-storage.md](code-plan-phase-14-exports-storage.md) | `feat(data_export/exports)` |
| 15 | Router + DI registration в `internal/app/app.go` и `internal/routers/routers.go` | [code-plan-phase-15-router-di.md](code-plan-phase-15-router-di.md) | `feat(app)` |
| 16 | Prometheus metrics + observability + structured logger middleware + DB-ping healthz | [code-plan-phase-16-metrics-observability.md](code-plan-phase-16-metrics-observability.md) | `feat(observability)` |

**Итого:** 16 фаз.

---

## 3. Глобальные правила выполнения

1. Каждая фаза = один атомарный коммит (см. memory `feedback_commits_per_change.md`).
2. После каждой фазы `make build` обязан проходить.
3. После фаз с тестами — `make test-unit` и/или `make test-integration` обязаны проходить.
4. Module path жёстко: `github.com/Kitavrus/e_zoo` (ADR-016).
5. Папка фичи: `internal/features/data_export/` (snake_case, как в spec §«Critical files»).
6. CLI-бинарь: `cmd/source-adapter/main.go` (а не `cmd/server/`).
7. Миграции `golang-migrate/v4` — **без auto-apply**, только явный `make migrate-up`.
8. Тесты репозитория — обязательно через dockertest `postgres:18-alpine`.
9. Sentinel-ошибки покрываются согласно матрице из `design-tests.md` §6.
10. Контекст (`ctx` first) — все обращения к БД через `pgxpool` с обязательным `ctx`.

---

## 4. Связанные документы

- Spec: [spec-interview/output.md](spec-interview/output.md)
- Design: [design.md](design.md), [design-go-layers.md](design-go-layers.md), [design-sql.md](design-sql.md), [design-tests.md](design-tests.md), [design-di.md](design-di.md), [design-errors.md](design-errors.md), [design-infrastructure.md](design-infrastructure.md), [design-integrations.md](design-integrations.md), [design-adr.md](design-adr.md)
- Status: [code-plan-status.md](code-plan-status.md)

---

## 5. Готовность к Executing

После фазы 16 сервис умеет:
- читать `/v1/{entity}` для всех 16 сущностей с JWT/role gating;
- запускать суточный load по cron + по `POST /admin/loads`;
- атомарно flip-ать snapshot;
- отдавать reject_log и audit_access на `/admin/*`;
- экспортировать большие наборы async через local FS;
- отдавать Prometheus-метрики на `/metrics`;
- иметь полный набор интеграционных тестов через dockertest.

Все 16 фаз должны быть `done` в [code-plan-status.md](code-plan-status.md) перед переходом в Stage 5 (Executing).
