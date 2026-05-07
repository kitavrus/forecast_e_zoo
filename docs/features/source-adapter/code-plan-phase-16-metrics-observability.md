# Phase 16: Prometheus metrics + observability

**Цель:** закрыть наблюдаемость: Prometheus `/metrics`, метрики бизнес-процесса и HTTP, structured-logger middleware (slog), расширенный `healthz` (DB ping + cron alive). После этой фазы сервис готов к оперативной эксплуатации (DevOps подключает Grafana).

**Commit:** `feat(observability): prometheus metrics + slog access middleware + extended healthz`

---

## Files to CREATE

### Метрики

- `internal/observability/metrics.go`:
  - `var (LoadSuccessTotal, LoadFailedTotal *prometheus.CounterVec; SnapshotNotReadyTotal prometheus.Counter; HTTPRequestsTotal *prometheus.CounterVec; HTTPRequestDuration *prometheus.HistogramVec; LoadDuration *prometheus.HistogramVec; LinesProcessedTotal, LinesFailedTotal *prometheus.CounterVec; AdvisoryLockBusyTotal prometheus.Counter; SchedulerTickTotal *prometheus.CounterVec)`.
  - `func Init() *prometheus.Registry` — регистрирует все метрики.
  - `func Handler() fiber.Handler` — обёртка над `promhttp.HandlerFor` для Fiber v3.
- `internal/observability/middleware.go`:
  - `HTTPMetricsMiddleware()` — измеряет latency, инкрементит `HTTPRequestsTotal{method, path, status}` и `HTTPRequestDuration`.
  - `AccessLogMiddleware(logger *slog.Logger)` — после `c.Next()` пишет access-log: method, path, status, latency, traceId, role.

### Healthz (расширенный)

- `internal/features/data_export/handler/healthz.go` (расширить из фазы 13):
  - Проверки: `db.Ping`, `scheduler.IsRunning`, `exports_storage.RootExists`.
  - Ответ: `{status, db, scheduler, exports_dir}`. 200 если всё ОК, 503 иначе.
  - `GET /healthz/ready` — отдельный readiness (только DB ping).
  - `GET /healthz/live` — liveness (всегда 200).

### Интеграция метрик в loader / scheduler

- `internal/features/data_export/loader/loader.go` — заинжектить метрики:
  - На успехе load → `metrics.LoadSuccessTotal{source}.Inc()`, `metrics.LoadDuration{source}.Observe(...)`, `metrics.LinesProcessedTotal{entity}.Add(...)`.
  - На fail → `metrics.LoadFailedTotal{source, reason}.Inc()`.
- `internal/features/data_export/scheduler/scheduler.go`:
  - `metrics.AdvisoryLockBusyTotal.Inc()` если lock занят.
  - `metrics.SchedulerTickTotal{result}.Inc()`.
- `internal/features/data_export/handler/snapshots.go`:
  - При 503 → `metrics.SnapshotNotReadyTotal.Inc()`.

### Тесты

- `internal/observability/metrics_test.go`:
  - `TestMetrics_AreRegistered` — все метрики появляются в Registry.
  - `TestMetrics_LoadSuccessIncrements`
  - `TestMetrics_HTTPHandlerExposesMetrics` — GET /metrics возвращает text/plain с нужными именами.
- `internal/observability/middleware_test.go`:
  - `TestHTTPMetricsMiddleware_RecordsLatency`
  - `TestAccessLogMiddleware_WritesEntry` (mock-handler slog).
- `test/e2e/observability_test.go` (build tag `integration`):
  - `TestE2E_MetricsEndpoint_ContainsExpected` — после успешного load, `/metrics` содержит `source_adapter_load_success_total{source="erp_e_zoo"}`.

## Files to MODIFY

- `internal/features/data_export/router/router.go` — добавить `/metrics` через `observability.Handler()` (БЕЗ JWT).
- `internal/app/app.go` — `metrics.Init()` в `New`, повесить `HTTPMetricsMiddleware` и `AccessLogMiddleware` глобально.
- `internal/features/data_export/handler/healthz.go` — расширить.
- `go.mod` / `go.sum` — `github.com/prometheus/client_golang`.
- `Makefile` — таргет `metrics-curl` (`curl http://localhost:8080/metrics | grep source_adapter`).

## SQL/Migrations

— нет.

## Run after

```bash
go mod tidy
make build
make test-unit
make test-integration
make lint
make run &
curl -s http://localhost:8080/metrics | grep '^source_adapter_'
curl -s http://localhost:8080/healthz | jq .
```

## Tests in this phase

- 3 unit-теста метрик
- 2 unit-теста middleware
- 1 e2e-тест /metrics

Итого: 6.

## Definition of Done

- [ ] `/metrics` возвращает все 9 метрик (см. список выше).
- [ ] HTTP middleware считает latency и status_code.
- [ ] Access-log пишется в slog JSON.
- [ ] Healthz расширенный (db + scheduler + exports_dir).
- [ ] Loader / Scheduler / Snapshots-handler инкрементят метрики.
- [ ] `make build` / `make test-unit` / `make test-integration` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(observability): ...`.

---

## Готовность к Stage 5 (Executing)

После закрытия фазы 16 в `code-plan-status.md` все 16 строк должны быть `done`. Сервис полностью работоспособен:
- master+facts ETL раз в сутки или по запросу;
- атомарный snapshot для downstream;
- read-only API + async exports;
- полная наблюдаемость;
- e2e-тесты покрывают happy path + ключевые edge cases.

Открытые блокеры (Q-001..Q-011, Q-013..Q-016 из spec-interview) к этому моменту НЕ решены MVP-кодом — они помечены ADR'ами как «Отложено». Реальная ERP-интеграция заменит `erp_e_zoo_reader` (фаза 09) — это уже работа Stage 5+ или v2.
