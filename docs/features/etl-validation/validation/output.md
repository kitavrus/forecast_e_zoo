# Validation: etl-validation
**Дата:** 2026-05-07
**Окружение:** локальная сборка (без поднятия сервисов)

## Scope валидации

Полный E2E (запуск source-adapter + etl + cron tick → ETL run → витрины) **отложен** — будет выполнен на следующей итерации после интеграции с реальным ERP клиентом.

Текущая валидация ограничена static-проверками + unit/integration тестами:

## Подготовка
- [x] `go build ./cmd/source-adapter ./cmd/etl` — оба бинаря собираются (exit 0)
- [x] `go vet ./...` — без ошибок
- [x] `go test ./... -race -count=1` — все unit-тесты зелёные (после S-1/S-2 фиксов)
- [x] `go test -tags=integration ./internal/features/etl_validation/repository/...` — 6 integration-тестов на postgres:18-alpine (включая advisory lock contention)
- [x] `golangci-lint run ./...` — 0 issues

## Покрытые сценарии (через unit + integration)

### Pipeline (TestRunAsync_*)
| # | Сценарий | Результат |
|---|---|---|
| 1 | Happy path: snapshot → extract → engine → loader.Apply с PopulateStaging callback | ✅ |
| 2 | Quality threshold violation (>1% lines failed) → markFailed | ✅ |
| 3 | Snapshot error → markFailed с reason | ✅ |
| 4 | Bad source_load_id → reject | ✅ |
| 5 | Loader error → markFailed, метрики корректны | ✅ |
| 6 | Violations записаны в reject_log | ✅ |

### Repository (integration, dockertest)
| # | Сценарий | Результат |
|---|---|---|
| 1 | etl_runs CRUD | ✅ |
| 2 | reject_log INSERT | ✅ |
| 3 | audit_access INSERT | ✅ |
| 4 | Advisory lock contention (две конкурирующие попытки) | ✅ |
| 5 | Staging tables CREATE TEMP | ✅ |
| 6 | UpdateEtlRunStatusTx atomic flip | ✅ |

### Validation engine
| # | Сценарий | Результат |
|---|---|---|
| 1 | fk_exists rule | ✅ |
| 2 | unique_business_key rule | ✅ |
| 3 | aggregate_sum_matches rule | ✅ |
| 4 | referential_integrity rule | ✅ |
| 5 | null_required_field rule | ✅ |
| 6 | YAML loader | ✅ |

### Extractor (HTTP клиент)
| # | Сценарий | Результат |
|---|---|---|
| 1 | NDJSON streaming с ETag | ✅ |
| 2 | JWT bearer (HS256/RS256) | ✅ |
| 3 | Retry с backoff cap 30s | ✅ |
| 4 | StaticTokenSource | ✅ |

### Admin auth (ADR-022)
| # | Сценарий | Результат |
|---|---|---|
| 1 | POST /admin/etl-runs без JWT → 401 | unit-тест в JWT middleware |
| 2 | POST /admin/etl-runs с x-flow-etl JWT (не admin-cli) → 403 | unit-тест в role middleware |
| 3 | GET /admin/etl-runs с it-read JWT → 200 (RequireAnyOf) | unit-тест в role middleware |

## Найденные проблемы
Нет (после фикса S-1 + S-2 в e2090ef и последующих коммитах).

## Документированные ограничения (НЕ блокеры)

1. **Полный E2E с реальным запуском двух сервисов отложен** до integration с реальным ERP. Текущий ETL pipeline протестирован через mock-extractor + dockertest postgres:18-alpine. Для prod-deploy требуется развернуть оба binary в одном docker-compose и провести функциональный тест cron+pipeline.
2. **Audit middleware (запись в `marts.audit_access`)** — Phase 16 design предусматривает middleware, но реализация подключена не во всех endpoint-ах. Отмечено в code-plan-status.md как тех.долг.
3. **Grafana dashboard + alert rules YAML + runbook** — относятся к infra-pipeline, выносятся отдельной задачей.

## Итог
**PASSED** (с документированными ограничениями).

Готов к Stage 10 (Report). Полный E2E запуск отложен на следующую итерацию после готовности Модулей 3+ и реальной ERP-интеграции.
