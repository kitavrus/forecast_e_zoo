# Code Plan Status — etl-validation (Модуль 2 «X-Flow ETL»)

> Single source of truth для статусов всех фаз. Обновляется в Executing.
> Допустимые значения: `pending`, `in_progress`, `completed`, `blocked`, `skipped`.

| #  | Фаза                                          | Status     | Commit | Заметки |
|----|-----------------------------------------------|------------|--------|---------|
| 01 | Bootstrap etl binary                          | `completed` |       | done 2026-05-07T01:00:00Z |
| 02 | Sentinel errors EV-*                          | `completed` |       | done 2026-05-07T01:15:00Z |
| 03 | Migrations 1001 — schema marts + 5 mart-таблиц | `completed`  |    | done 2026-05-07T01:30:00Z; integration validation deferred to Phase 06 |
| 04 | Migrations 1002 — etl_runs + reject_log + audit_access | `completed` |   | done 2026-05-07T01:40:00Z |
| 05 | Models / DTO                                  | `completed` |       | done 2026-05-07T01:55:00Z |
| 06 | Repository (pgx + go:embed) + integration test | `pending` |        |         |
| 07 | SQL queries (go:embed)                        | `pending`  |        |         |
| 08 | Validators (формат запросов)                  | `pending`  |        |         |
| 09 | Validation engine reuse + etl_validation_rules.yaml | `pending` |   |         |
| 10 | Extractor (HTTP клиент к source-adapter)      | `pending`  |        |         |
| 11 | Transformer (5 mart builders)                 | `pending`  |        |         |
| 12 | Loader + atomic flip                          | `pending`  |        |         |
| 13 | EtlPipeline service (orchestration)           | `pending`  |        |         |
| 14 | Scheduler (gocron + advisory lock)            | `pending`  |        |         |
| 15 | Admin handlers + Router + DI                  | `pending`  |        |         |
| 16 | Prometheus metrics + observability            | `pending`  |        |         |

---

## Правила обновления

- Перед стартом фазы: статус → `in_progress`.
- После атомарного коммита: статус → `completed`, в столбец `Commit` — короткий SHA.
- Блокер: статус → `blocked` + комментарий в «Заметки».
- НЕ редактировать статусы в `code-plan.md` или `code-plan-phase-*.md` — только здесь.
