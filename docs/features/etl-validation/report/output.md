# Pipeline Report: etl-validation
**Дата:** 2026-05-07
**Профиль:** Бизнес-фича
**Scope:** Модуль 2 «ETL и валидация» из MVP-пайплайна реплинишмента e_zoo

## Что сделано

Реализован отдельный сервис ETL поверх готового Модуля 1 (`source-adapter`). ETL ходит в API адаптера через JWT (роль `x-flow-etl`), скачивает все 16 сущностей (NDJSON streaming + ETag), прогоняет cross-entity валидацию по YAML-правилам, строит 5 mart-таблиц в схеме `marts` и атомарно фиксирует через `etl_runs` registry.

- Module path: `github.com/Kitavrus/e_zoo` (тот же монорепо)
- Новый бинарь: `cmd/etl/main.go` + DI-root `internal/etlapp/`
- Новый docker-compose service: `etl`

## Артефакты pipeline (по стадиям)

| Стадия | Файлы | Итог |
|---|---|---|
| Research | research/output.md | 22 OQ |
| Spec Interview | spec-interview/output.md | 25 Q-NNN (3 новых от orchestrator) |
| Design | 12 design-*.md + ADR-001..025 + ADR-100..107 | APPROVED first-pass |
| Swimlane | design-swimlane.html | 5×5 grid, 9 FLOWS |
| Plan | code-plan.md + code-plan-status.md + 16 phase-файлов | 16 фаз ≤ лимита |
| Executing | весь Go-код | 16/16 completed, 17 коммитов (16 фаз + 1 fix S-1/S-2) |
| Code Review | reviewer/output.md | 0 блокеров, 3 серьёзных → 2 закрыты, 1 документирован |
| Validation | validation/output.md | PASSED (unit + integration; полный E2E отложен) |

## Ключевые архитектурные решения (топ ADR)

- ADR-001: Отдельный `cmd/etl/main.go` бинарь
- ADR-003: Own cron 02:30 Europe/Kyiv (configurable)
- ADR-006: Schema `marts` в той же БД, role `mart_reader` NOLOGIN
- ADR-007/008: RANGE month partitioning + 365d retention для `mart_demand_history`, `mart_kpi_daily`
- ADR-011: YAML-driven validation engine (in-memory `Dataset` + 5 builtin rules: fk_exists, unique_business_key, aggregate_sum_matches, referential_integrity, null_required_field)
- ADR-013: ETL вычисляет `applicable_rule_id` (priority order_rule > supply_spec)
- ADR-015: Quality threshold 1%
- ADR-017: Full read per snapshot (не incremental cursor)
- ADR-022: JWT `admin-cli` для /admin/* (не X-Admin-Secret)

## Что реализовано (фазы 1-16)

1. **Bootstrap etl** — cmd/etl, internal/etlapp, Dockerfile.etl, docker-compose etl service, Makefile *-etl
2. **Sentinel errors EV-***  — 5 EV-* + переиспользованы 2 (ErrSnapshotNotReady, ErrQualityThresholdExceeded)
3. **Migrations 1001** — schema marts + 5 mart-таблиц (2 partitioned) + role mart_reader
4. **Migrations 1002** — etl_runs, reject_log, audit_access + GRANT
5. **Models / DTO** — constants/models/dto + swag enum sync-tests
6. **Repository** — pgx + go:embed + 6 integration tests (postgres:18-alpine)
7. **SQL queries** — UPSERT/append/staging/cleanup queries
8. **Validators** — формат запросов admin endpoints
9. **Validation engine** — Dataset + 5 builtin rules + YAML loader + DefaultRegistry + etl_validation_rules.yaml
10. **Extractor** — HTTP клиент с JWT (HS256/RS256), ETag, retry/backoff cap 30s, NDJSON 1MiB scanner
11. **Transformer** — 5 mart builders + Registry + applicable_rule_id resolver
12. **Loader + atomic flip** — single-tx COPY-staging → builders[].Build → UpdateEtlRunStatusTx
13. **EtlPipeline service** — TryStart + advisory lock + runAsync (extract → engine.Run → quality gate → loader.Apply)
14. **Scheduler** — gocron v2 + advisory lock + PartitionMaintainer
15. **Admin handlers + Router + DI** — 7 endpoints + JWT admin-cli/it-read middleware
16. **Prometheus metrics** — etl_run_*, etl_lines_*, mart_rows_*, etl_lag_seconds, etl_runs_skipped_total + /metrics

## Метрики прогона

- Subagent-вызовов: ~10
- Git коммитов: 17 (16 фаз + 1 fix-серьёзных)
- Файлов design: 13 (12 .md + 1 .html)
- Файлов code-plan: 18
- Тестов: ~140 unit + 6 integration

## Открытые / отложенные вопросы

| Q-NNN | Тема | Статус |
|---|---|---|
| Q-012 | Bi-temporal recompute | Отложено (next iter, append-only MVP) |

## Известные ограничения (MVP)

- Полный E2E (запуск source-adapter + etl + cron tick) отложен — текущая валидация через unit + integration. Реальный E2E после интеграции с ERP клиента.
- Audit middleware подключён частично — `marts.audit_access` пишется не из всех endpoint-ов
- Grafana dashboard / Prometheus alert rules / runbook → infra-pipeline (отдельная задача)
- `mart_supplier_scorecard` rolling weekly — refresh-логика только при ondemand POST или когда прошла неделя с last_refresh

## Найденные и исправленные баги в code-review

- **S-1 (HIGH):** Pipeline `runAsync` стартовал с пустым Dataset (extract+stage+validate stub'д). Исправлено: реальный extract → stage (COPY) → engine.Run → quality gate → loader.Apply с PopulateStaging callback.
- **S-2 (HIGH):** Admin auth использовал X-Admin-Secret вместо JWT admin-cli (нарушение ADR-022). Исправлено: JWT + RequireRole(admin-cli) для write, RequireAnyOf(admin-cli, it-read) для read.

## Quality gates (финальные)

- `go build ./...` — OK (оба бинаря)
- `go vet ./...` — OK
- `go test ./... -race -count=1` — OK
- `go test -tags=integration ./internal/features/etl_validation/repository/...` — 6/6 OK
- `golangci-lint run ./...` — 0 issues

## Что дальше (next iter после MVP)

- Полный E2E функциональный тест в docker-compose (source-adapter + etl + cron)
- Audit middleware coverage в etl admin endpoints
- Bi-temporal recompute (Q-012) после стабилизации pipeline
- Grafana dashboard + Prometheus alert rules + runbook (через infra-pipeline)
- Подключение Модуля 3 (Витрины — на стороне X-Flow ETL это уже текущий модуль; Модуль 3 в плане проекта может быть переименован в «Mart consumers» / KPI pre-views)

## Ссылки

- [Research](../research/output.md)
- [Spec](../spec-interview/output.md)
- [Design](../design.md)
- [ADR](../design-adr.md)
- [Swimlane](../design-swimlane.html)
- [Code Plan](../code-plan.md)
- [Status](../code-plan-status.md)
- [Code Review](../reviewer/output.md)
- [Validation](../validation/output.md)
