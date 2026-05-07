# Phase 16 — Prometheus metrics + observability

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Подключить Prometheus метрики (см. ADR-009 + design-integrations.md §4): `etl_run_duration_seconds`, `etl_run_success_total`, `etl_run_failed_total{reason}`, `etl_lines_processed_total{entity}`, `etl_lines_failed_total{entity, severity}`, `mart_rows_total{mart}`, `etl_lag_seconds`, `etl_runs_skipped_total{reason}`, `etl_extractor_request_seconds`, `etl_advisory_lock_held_seconds`. Расширить Grafana JSON-дашборд X-Flow row-ом «X-Flow ETL». Финализация slog access middleware и audit_access покрытия.

## Commit

```
feat(etl_validation): Prometheus metrics + Grafana panel + extended observability
```

## Files to CREATE

### Production code

- `internal/features/etl_validation/metrics/metrics.go`:
  - `Recorder` интерфейс — методы:
    - `RecordRunDuration(d time.Duration)`
    - `IncSuccess()`
    - `IncFailed(reason string)`  // reason ∈ {quality, source_unavailable, builder_error, internal}
    - `AddLinesProcessed(entity string, n int64)`
    - `AddLinesFailed(entity, severity string, n int64)`
    - `SetMartRows(mart string, n int64)`
    - `SetLagSeconds(s float64)`
    - `IncSkipped(reason string)`  // already_running | snapshot_not_ready
    - `ObserveExtractorRequest(entity string, d time.Duration)`
    - `ObserveAdvisoryLockHeld(d time.Duration)`
  - `New(reg prometheus.Registerer) *Impl` — регистрирует метрики.
- `internal/features/etl_validation/metrics/middleware.go`:
  - HTTP-метрики и slog access log middleware (если не унаследован напрямую от Модуля 1; иначе reuse).

### Grafana

- `infrastructure/dev/grafana/dashboards/x_flow_etl_panels.json` (или patch к существующему `x_flow.json`) — секция «X-Flow ETL» (см. design-infrastructure.md §6):
  - ETL run duration p95 (7d).
  - ETL success rate (1d).
  - ETL lag seconds (gauge).
  - Lines failed % (1d).
  - Mart rows total (per mart).
  - Skipped runs (per reason).
- `infrastructure/dev/prometheus/rules/etl_alerts.yml`:
  - `etl_lag_seconds > 4 * 3600` 5m → critical.
  - `up{job="etl"} == 0` 5m → critical.
  - `increase(etl_run_failed_total{reason="quality"}[1d]) > 0` → warning.
  - `increase(etl_runs_skipped_total{reason="snapshot_not_ready"}[1h]) > 5` → warning (источник постоянно недоступен).

### Docs

- `docs/features/etl-validation/runbook.md` (NEW — обязательный для L-tier per CLAUDE.md §4.5):
  - Команды деплоя (`make build-etl`, `docker-build-etl`, compose up).
  - Smoke-проверки (curl healthz/metrics, ручной POST /admin/etl-runs c admin JWT).
  - Дашборды Grafana (ссылки).
  - Алерты и что делать при срабатывании:
    - `etl_lag_seconds` > 4h → проверить scheduler logs, source-adapter health.
    - `up{job=etl}==0` → перезапуск контейнера, проверить crashloop.
    - `quality threshold exceeded` → инспекция `marts.reject_log` за последний run, manual retry после фикса в источнике.
    - `etl_run_already_running` шум → проверить «висящий» run (status='running' старше 4h) → ручной UPDATE на 'aborted' или дождаться advisory lock авто-release при крэше.

### Tests

- `internal/features/etl_validation/metrics/metrics_test.go` — unit-тест: после серии Inc* / Observe* — `prometheus.Gather()` возвращает ожидаемые семейства.
- `internal/features/etl_validation/metrics/integration_test.go` — `GET /metrics` возвращает все ETL семейства (smoke).

## Files to MODIFY

- `internal/features/etl_validation/service/etl_pipeline.go` — внедрить `metrics.Recorder`:
  - `RecordRunDuration` после finalize.
  - `IncSuccess` / `IncFailed(reason)` в финале.
  - `AddLinesProcessed` / `AddLinesFailed` в фазе validation.
  - `SetLagSeconds = NOW() - source_load_id.created_at` (либо `committed_at` предыдущего run-а).
  - `ObserveAdvisoryLockHeld` от moment lock acquired до released.
- `internal/features/etl_validation/scheduler/scheduler.go` — `IncSkipped(reason)` для `already_running` / `snapshot_not_ready`.
- `internal/features/etl_validation/extractor/client.go` — `ObserveExtractorRequest(entity, duration)` per request.
- `internal/features/etl_validation/loader/loader.go` — `SetMartRows(mart, count)` после INSERT.
- `internal/etlapp/app.go` — wire `metrics.Recorder` во все consumer-ы; `metricsHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})` подключить в router как `/metrics`.
- `CLAUDE.md` §8 — обновить таблицу supportMessage кодов: добавить EV-001..EV-005 (если ещё не добавлены).
- `docs/features/etl-validation/code-plan-status.md` — после успешного завершения этой фазы все строки → `completed`.

## SQL / Migrations

Нет.

## Run after

```bash
go build ./...
go test ./... -race -count=1
go test -tags=integration ./... -race -count=1
golangci-lint run ./...
curl -s http://localhost:8081/metrics | grep -E '^etl_'   # smoke
promtool check rules infrastructure/dev/prometheus/rules/etl_alerts.yml
```

## Tests

| Test | Что проверяет |
|---|---|
| `TestMetrics_RecordRunDuration` | histogram bucket increments |
| `TestMetrics_IncFailed_QualityReason` | `etl_run_failed_total{reason="quality"}` ++ |
| `TestMetrics_AddLinesProcessed` | counter per entity |
| `TestMetrics_SetMartRows` | gauge per mart |
| `TestMetrics_IncSkipped_AlreadyRunning` | counter increment |
| Integration `/metrics` smoke | все семейства exposed |
| `promtool check rules etl_alerts.yml` | alerts валидны |

## Definition of Done

- [ ] Все метрики из ADR-009 экспонируются на `GET /metrics`.
- [ ] Pipeline / scheduler / extractor / loader публикуют метрики.
- [ ] Grafana dashboard расширен (panel X-Flow ETL).
- [ ] Prometheus alerts (`etl_lag_seconds`, `up`, `etl_run_failed_total`) валидируются `promtool`.
- [ ] Runbook (`docs/features/etl-validation/runbook.md`) написан.
- [ ] CLAUDE.md §8 содержит коды EV-001..EV-005.
- [ ] `code-plan-status.md` все 16 фаз → `completed`.
- [ ] `make swagger` обновлён (если новые endpoint-аннотации добавлены).
- [ ] `golangci-lint`/`go vet`/`go test` зелёные.

## Зависимости

Финальная фаза. Требует все предыдущие, особенно фазы 13 (pipeline), 14 (scheduler), 15 (router/DI).
