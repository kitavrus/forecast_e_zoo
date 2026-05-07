# Code Plan Status: source-adapter

> **Single source of truth статусов для фичи `source-adapter`.**
> Главный обзор плана: [code-plan.md](code-plan.md). Здесь — только текущие статусы.

**Допустимые значения:** `pending` | `in_progress` | `done` | `blocked` | `skipped`.

---

## Статусы фаз

| # | Фаза | Файл | Status | Updated | Note |
|---|---|---|---|---|---|
| 01 | Bootstrap | [code-plan-phase-01-bootstrap.md](code-plan-phase-01-bootstrap.md) | done | 2026-05-07T01:00:00Z | build/test/vet/lint зелёные; pgx/uuid/migrate/gocron — добавятся в фазах 8/12 при первом импорте |
| 02 | JWT middleware | [code-plan-phase-02-jwt-middleware.md](code-plan-phase-02-jwt-middleware.md) | done | 2026-05-07T01:30:00Z | build/test/lint зелёные; 13+ тестов в middleware |
| 03 | Migrations master | [code-plan-phase-03-migrations-master.md](code-plan-phase-03-migrations-master.md) | done | 2026-05-07T01:50:00Z | 17 таблиц; integration-тесты на dockertest зелёные |
| 04 | Migrations facts (partitioned) | [code-plan-phase-04-migrations-facts-partitioned.md](code-plan-phase-04-migrations-facts-partitioned.md) | done | 2026-05-07T02:05:00Z | 4 partitioned facts по event_date с initial 4 месячных партиций; integration зелёные |
| 05 | Models / DTO | [code-plan-phase-05-models-dto.md](code-plan-phase-05-models-dto.md) | done | 2026-05-07T02:30:00Z | build/test/lint зелёные; cursor + dto валидаторы покрыты тестами |
| 06 | SQL queries (go:embed) | [code-plan-phase-06-sql-queries.md](code-plan-phase-06-sql-queries.md) | done | 2026-05-07T02:50:00Z | 32 SQL-файла + embed.go; build/test/lint зелёные; SQL подогнаны под фактические migrations 0001/0002 |
| 07 | Validators + Engine + YAML | [code-plan-phase-07-validators-engine-yaml.md](code-plan-phase-07-validators-engine-yaml.md) | done | 2026-05-07T03:10:00Z | 7 правил YAML; engine + 7 builtin checks; формальные validators; 3 новых sentinel; build/test/lint зелёные |
| 08 | Repository + integration tests | [code-plan-phase-08-repository.md](code-plan-phase-08-repository.md) | done | 2026-05-07T03:35:00Z | pgx Repository + 15 integration-тестов на dockertest postgres:18-alpine; 6 новых sentinel; build/lint зелёные |
| 09 | SourceReader interface + stub | [code-plan-phase-09-source-reader-stub.md](code-plan-phase-09-source-reader-stub.md) | done | 2026-05-07T03:55:00Z | SourceReader generic interface (16 entities) + ErpEZooReader stub + 16 fixtures + 7 unit-тестов; build/lint зелёные |
| 10 | Loader service | [code-plan-phase-10-loader-service.md](code-plan-phase-10-loader-service.md) | done | 2026-05-07T04:10:00Z | Loader pipeline (master → facts → flip + quality threshold); LoaderAPI interface для mock; 7 unit-тестов; 2 internal sentinel; build/lint зелёные |
| 11 | Snapshot + Audit | [code-plan-phase-11-snapshot-audit.md](code-plan-phase-11-snapshot-audit.md) | pending | 2026-05-07 | — |
| 12 | Scheduler + admin handlers | [code-plan-phase-12-scheduler-admin-handlers.md](code-plan-phase-12-scheduler-admin-handlers.md) | pending | 2026-05-07 | — |
| 13 | Read handlers | [code-plan-phase-13-read-handlers.md](code-plan-phase-13-read-handlers.md) | pending | 2026-05-07 | — |
| 14 | Exports storage | [code-plan-phase-14-exports-storage.md](code-plan-phase-14-exports-storage.md) | pending | 2026-05-07 | — |
| 15 | Router + DI | [code-plan-phase-15-router-di.md](code-plan-phase-15-router-di.md) | pending | 2026-05-07 | — |
| 16 | Metrics + observability | [code-plan-phase-16-metrics-observability.md](code-plan-phase-16-metrics-observability.md) | pending | 2026-05-07 | — |

---

## Сводка

- Всего фаз: **16**
- pending: **6**
- in_progress: **0**
- done: **10**
- blocked: **0**
- skipped: **0**

> При смене статуса меняем строку фазы и обновляем сводку. Главный план НЕ трогаем.
