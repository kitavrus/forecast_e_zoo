# Code Plan — etl-validation (Модуль 2 «X-Flow ETL»)

> **Статусы выполнения — только в [code-plan-status.md](./code-plan-status.md).**
> Файлы фаз содержат описания, без статусов.

```yaml
# Triage
tier: L
touches: {db: true, fe: false, infra: true, external: true}
risk: irreversible
novelty: new-pattern
decisions:
  - cmd/etl как отдельный binary (Q-001, ADR-002)
  - feature folder etl_validation (Q-002)
  - cron 02:30 Europe/Kyiv (Q-003)
  - advisory lock + etl_runs registry (Q-004)
  - YAML-driven validation engine reuse (Q-011, ADR-011)
  - applicable_rule_id вычисляет ETL (Q-013/Q-024)
  - quality threshold 1% (Q-015)
  - admin-cli JWT для /admin/* (Q-022)
```

## Обзор

Модуль 2 строит mart-витрины (`marts.*`) поверх готового Модуля 1 (source-adapter), потребляя его REST API (snapshots + entity NDJSON). Запускается как отдельный binary `cmd/etl` с собственным Fiber-приложением (admin endpoints) и cron-scheduler-ом (gocron).

Reuse из Модуля 1:
- `pkg/errorspkg` (расширяем EV-* sentinel-ами)
- `internal/middleware/jwt` + `internal/middleware/role`
- `pkg/logger` (slog JSON)
- паттерн feature-структуры (handler → service → repository, go:embed SQL)
- validation engine из `internal/features/data_export/validation` (как library)
- Suite через `pkg/dockertestpkg`

## Конвенции

- Каждая фаза = атомарный коммит (см. поле `Commit` в каждом phase-файле).
- Тесты — в той же фазе, что и код (unit/integration).
- После каждой фазы `go build ./...` проходит без ошибок (даже если фича недособрана — добавляем заглушки).
- SQL — только через `go:embed` (никаких raw-строк в Go).
- pgx/v5 + pgxpool. `context.Background()` запрещён внутри handler/service/repository (линтер `noctx`).
- Линтер: `golangci-lint run ./...` (32 линтера, см. `.golangci.yml`).

## Порядок фаз (16)

| #  | Фаза                                                       | Файл                                                               |
|----|------------------------------------------------------------|--------------------------------------------------------------------|
| 01 | Bootstrap etl binary                                       | [code-plan-phase-01-bootstrap-etl.md](./code-plan-phase-01-bootstrap-etl.md) |
| 02 | Sentinel errors EV-*                                       | [code-plan-phase-02-sentinel-errors.md](./code-plan-phase-02-sentinel-errors.md) |
| 03 | Migrations 1001 — schema marts + 5 mart-таблиц             | [code-plan-phase-03-migrations-marts-schema.md](./code-plan-phase-03-migrations-marts-schema.md) |
| 04 | Migrations 1002 — etl_runs + reject_log + audit_access     | [code-plan-phase-04-migrations-etl-runs.md](./code-plan-phase-04-migrations-etl-runs.md) |
| 05 | Models / DTO                                               | [code-plan-phase-05-models-dto.md](./code-plan-phase-05-models-dto.md) |
| 06 | Repository (pgx + go:embed) + integration test             | [code-plan-phase-06-repository.md](./code-plan-phase-06-repository.md) |
| 07 | SQL queries (go:embed)                                     | [code-plan-phase-07-sql-queries.md](./code-plan-phase-07-sql-queries.md) |
| 08 | Validators (формат запросов)                               | [code-plan-phase-08-validators.md](./code-plan-phase-08-validators.md) |
| 09 | Validation engine reuse + etl_validation_rules.yaml        | [code-plan-phase-09-validation-engine.md](./code-plan-phase-09-validation-engine.md) |
| 10 | Extractor (HTTP клиент к source-adapter)                   | [code-plan-phase-10-extractor.md](./code-plan-phase-10-extractor.md) |
| 11 | Transformer (5 mart builders)                              | [code-plan-phase-11-transformer.md](./code-plan-phase-11-transformer.md) |
| 12 | Loader + atomic flip                                       | [code-plan-phase-12-loader.md](./code-plan-phase-12-loader.md) |
| 13 | EtlPipeline service (orchestration)                        | [code-plan-phase-13-pipeline-service.md](./code-plan-phase-13-pipeline-service.md) |
| 14 | Scheduler (gocron + advisory lock)                         | [code-plan-phase-14-scheduler.md](./code-plan-phase-14-scheduler.md) |
| 15 | Admin handlers + Router + DI                               | [code-plan-phase-15-handlers-router-di.md](./code-plan-phase-15-handlers-router-di.md) |
| 16 | Prometheus metrics + observability                         | [code-plan-phase-16-metrics-observability.md](./code-plan-phase-16-metrics-observability.md) |

## Quality Gates (после каждой фазы)

```bash
go build ./...
golangci-lint run ./...
go test ./... -race -count=1
# integration — после фаз с изменениями БД/HTTP/pipeline:
go test -tags=integration ./internal/features/etl_validation/... -count=1
```

## Связь с design

- Архитектура: `design.md`, `design-go-layers.md`, `design-c4.md`, `design-dataflow.md`.
- SQL: `design-sql.md`.
- Errors: `design-errors.md`.
- Tests: `design-tests.md`.
- DI: `design-di.md`.
- Infra: `design-infrastructure.md`.
- Integrations: `design-integrations.md`.
- ADR: `design-adr.md`.
