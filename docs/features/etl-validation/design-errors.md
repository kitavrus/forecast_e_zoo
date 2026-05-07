# Design Errors — etl-validation

> Reuse `pkg/errorspkg` (sentinel-set + `WriteJSON` для HTTP-mapping). HTTP-mapping inline в handler-ах через `errorspkg.WriteJSON(c, err)` — отдельного `mappers/errors.go` нет (паттерн Модуля 1).

---

## 1. Новые sentinel (расширение `pkg/errorspkg`)

| Sentinel | code (JSON) | HTTP | Support code (SA-prefix → теперь `EV-`) | Where it's raised |
|---|---|---|---|---|
| `ErrEtlRunAlreadyRunning` | `etl_run_already_running` | 409 Conflict | `EV-001` | `service.EtlPipeline.TryStart` если advisory lock busy |
| `ErrEtlRunNotFound` | `etl_run_not_found` | 404 Not Found | `EV-002` | `repository.EtlRunRepository.GetByID` |
| `ErrCannotRetryEtl` | `cannot_retry_etl` | 409 Conflict | `EV-003` | `service.EtlRun.Retry` если status NOT IN ('failed','aborted') |
| `ErrSourceUnavailable` | `source_unavailable` | 502 Bad Gateway | `EV-004` | `extractor.Client.Do` после исчерпания retry |
| `ErrMartRefreshNotSupported` | `mart_refresh_not_supported` | 400 Bad Request | `EV-005` | `service.MartRefresh.Refresh` для name ≠ `mart_supplier_scorecard` |

**Reuse существующих** (без изменений в `pkg/errorspkg`):

| Sentinel | code | HTTP | Где |
|---|---|---|---|
| `ErrSnapshotNotReady` | `snapshot_not_ready` | 503 | `extractor.SnapshotsClient.GetCurrent` если 503 от source-adapter |
| `ErrQualityThresholdExceeded` | `quality_threshold_exceeded` | 422 (внутри pipeline-а; для admin endpoint выдаётся 200 с marts_summary в случае GET) | `service.EtlPipeline.Run` |
| `ErrUnauthorized` | `unauthorized` | 401 | JWT middleware |
| `ErrForbidden` | `forbidden` | 403 | role middleware |
| `ErrBadRequest` | `bad_request` | 400 | validators |
| `ErrInternal` | `internal` | 500 | unrecoverable, logged |
| `ErrNotFound` | `not_found` | 404 | generic |

---

## 2. JSON-схема ответа (унифицирована с Модулем 1)

```json
{
  "error": {
    "code": "etl_run_already_running",
    "message": "ETL run already in progress",
    "support_code": "EV-001",
    "details": {
      "current_run_id": "...uuid..."
    },
    "request_id": "..."
  }
}
```

Поле `details` — `[]Detail{Field, Rule, Value}` либо `map[string]any` (как в Модуле 1).

---

## 3. Mapping в handler

```go
func (h *AdminEtlRunsHandler) Trigger(c fiber.Ctx) error {
    runID, err := h.svc.TriggerRun(c.UserContext(), requesterFrom(c))
    if err != nil {
        // sentinel-чувствительный mapping; по умолчанию 500
        return errorspkg.WriteJSON(c, err)
    }
    return c.Status(202).JSON(runResponse(runID))
}
```

`errorspkg.WriteJSON` берёт `Error.HTTPStatus` из sentinel, инжектит `request_id` из контекста, формирует JSON.

---

## 4. Сборка sentinel-ов в `pkg/errorspkg/errors.go`

```go
var (
    // … существующие …
    ErrEtlRunAlreadyRunning   = newSentinel("etl_run_already_running", http.StatusConflict, "EV-001")
    ErrEtlRunNotFound         = newSentinel("etl_run_not_found",        http.StatusNotFound, "EV-002")
    ErrCannotRetryEtl         = newSentinel("cannot_retry_etl",         http.StatusConflict, "EV-003")
    ErrSourceUnavailable      = newSentinel("source_unavailable",       http.StatusBadGateway, "EV-004")
    ErrMartRefreshNotSupported = newSentinel("mart_refresh_not_supported", http.StatusBadRequest, "EV-005")
)
```

Существующие (`ErrSnapshotNotReady` и пр.) **не меняются** и не дублируются.

---

## 5. Поведение pipeline при ошибках

| Где | Sentinel | Что делает pipeline |
|---|---|---|
| advisory lock busy | `ErrEtlRunAlreadyRunning` | NO INSERT etl_runs; tick: skip silently + метрика `etl_skipped_lock_taken_total++`; admin endpoint: 409 + currentRunID |
| `extractor.GetCurrentSnapshot` returns `ErrSnapshotNotReady` | reuse | UPDATE etl_runs SET status='aborted', reason='snapshot_not_ready'; release lock; tick: метрика `etl_skipped_no_snapshot_total++`; admin endpoint: 503 |
| `extractor.StreamEntity` returns `ErrSourceUnavailable` | new | UPDATE etl_runs SET status='failed', reason='source_unavailable'; release lock; alert через `etl_run_failed_total{reason="source_unavailable"}` |
| validation: critical/total > threshold | `ErrQualityThresholdExceeded` (reuse) | UPDATE etl_runs SET status='failed', reason='quality_threshold_exceeded'; mart_* НЕ flip-аются |
| internal panic | `ErrInternal` (reuse) | recovered in goroutine; UPDATE etl_runs SET status='failed', reason=err.String() |

---

## 6. Wrapping & matching

- Везде `fmt.Errorf("...: %w", err)` оборачивает sentinel.
- Handler делает `errors.Is(err, errorspkg.ErrXxx)` через `WriteJSON` (внутри уже).
- Service не паникует — все ошибки возвращает.

---

## 7. Структура `Error.Details`

| Sentinel | Поля Details |
|---|---|
| `ErrEtlRunAlreadyRunning` | `{current_run_id: UUID, started_at: RFC3339}` |
| `ErrCannotRetryEtl` | `{run_id: UUID, current_status: "running"}` |
| `ErrSourceUnavailable` | `{entity: string, attempts: int, last_status: int}` |
| `ErrMartRefreshNotSupported` | `{requested: string, supported: ["mart_supplier_scorecard"]}` |
| `ErrQualityThresholdExceeded` | `{lines_total: int64, lines_failed: int64, threshold_pct: float64}` |

---

## 8. Тестирование sentinel'ов

См. [design-tests.md](design-tests.md) §4 (матрица sentinel ↔ test).
