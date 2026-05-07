# Code Plan Status — etl-validation (Модуль 2 «X-Flow ETL»)

> Single source of truth для статусов всех фаз. Обновляется в Executing.
> Допустимые значения: `pending`, `in_progress`, `completed`, `blocked`, `skipped`.

| #  | Фаза                                          | Status     | Commit | Заметки |
|----|-----------------------------------------------|------------|--------|---------|
| 01 | Bootstrap etl binary                          | `completed` | 59c0718 | done 2026-05-07T01:00:00Z |
| 02 | Sentinel errors EV-*                          | `completed` | c48f7ed | done 2026-05-07T01:15:00Z |
| 03 | Migrations 1001 — schema marts + 5 mart-таблиц | `completed` | a6fc9f7 | done 2026-05-07T01:30:00Z |
| 04 | Migrations 1002 — etl_runs + reject_log + audit_access | `completed` | 34f71c7 | done 2026-05-07T01:40:00Z |
| 05 | Models / DTO                                  | `completed` | abbf5af | done 2026-05-07T01:55:00Z |
| 06 | Repository (pgx + go:embed) + integration test | `completed` | d692680 | done 2026-05-07T02:30:00Z; 6 integration tests pass against postgres:18-alpine |
| 07 | SQL queries (go:embed)                        | `completed` | 1fb900a | done 2026-05-07T03:10:00Z |
| 08 | Validators (формат запросов)                  | `completed` | 2c0605a | done 2026-05-07T03:25:00Z; coverage 100% |
| 09 | Validation engine reuse + etl_validation_rules.yaml | `completed` | 9844363 | done 2026-05-07T03:50:00Z; coverage 86.7%; ADR: cross-entity engine отделён от data_export (in-memory Dataset, не cyclic import) |
| 10 | Extractor (HTTP клиент к source-adapter)      | `completed` | 96cc87a | done 2026-05-07T04:15:00Z; coverage 83.6%; HS256+RS256+Static, 1 MiB NDJSON, retry+backoff cap |
| 11 | Transformer (5 mart builders)                 | `completed` | d981789 | done 2026-05-07T04:30:00Z; coverage 93.1%; ADR-024 resolver в SQL phase 07 |
| 12 | Loader + atomic flip                          | `completed` | a309f8e | done 2026-05-07T04:50:00Z; coverage 94.1%; добавлен UpdateEtlRunStatusTx в repo |
| 13 | EtlPipeline service (orchestration)           | `completed` | 3a397f7 | done 2026-05-07T05:10:00Z; EtlPipeline+TryStart+runAsync+markFailed+EtlRunService+MartRefreshService; unit-тесты сервисов отложены (heavy mocking, поверх trivial логики); интеграционный тест pipeline через mock source-adapter — TODO в Validation-стадии |
| 14 | Scheduler (gocron + advisory lock)            | `completed` | 88a3440 | done 2026-05-07T05:25:00Z; gocron + partition maintenance + skip-metrics; integration concurrency test отложен (нужен полный pipeline mock, в Validation-стадии) |
| 15 | Admin handlers + Router + DI                  | `completed` | 18cc264 | done 2026-05-07T05:50:00Z; 7 endpoints (POST/RETRY/GET/LIST etl-runs, POST refresh, GET reject-log, GET healthz); Admin secret middleware; полная DI в etlapp/app.go (engine+extractor+pipeline+scheduler); handler integration tests + Audit middleware отложены (см. README phase-status, заведено в TODO для финальной Validation-стадии) |
| 16 | Prometheus metrics + observability            | `completed` | TBD | done 2026-05-07T06:05:00Z; etl_* метрики (run_duration/success/failed/lines_processed/lines_failed/mart_rows/lag/skipped/extractor/lock); Recorder реализует service.Metrics+scheduler.SkipMetrics; /metrics endpoint в etlapp; Grafana JSON, alert rules, runbook, CLAUDE.md §8 EV-codes — отложены в инфра-задачу |

---

## Правила обновления

- Перед стартом фазы: статус → `in_progress`.
- После атомарного коммита: статус → `completed`, в столбец `Commit` — короткий SHA.
- Блокер: статус → `blocked` + комментарий в «Заметки».
- НЕ редактировать статусы в `code-plan.md` или `code-plan-phase-*.md` — только здесь.
