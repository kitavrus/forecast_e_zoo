# Design Errors — source-adapter

Sentinel-ошибки и mapping в HTTP-ответы. Пакет `pkg/errorspkg`.

---

## 1. ErrorResponseJSON формат

```go
// pkg/errorspkg/errors.go
package errorspkg

type ErrorResponseJSON struct {
    Code           string         `json:"code"`
    Message        string         `json:"message"`
    SupportMessage string         `json:"supportMessage,omitempty"`
    TraceID        string         `json:"traceId,omitempty"`
    Details        map[string]any `json:"details,omitempty"`
}
```

Все `/v1/*` и `/admin/*` ошибки возвращают этот формат + соответствующий HTTP-статус.

## 2. Sentinel-ошибки

```go
// pkg/errorspkg/errors.go
package errorspkg

import "errors"

var (
    // 400
    ErrBadRequest             = newE("bad_request",             "Invalid request")
    ErrInvalidCursor          = newE("invalid_cursor",          "Invalid cursor")
    ErrInvalidQuery           = newE("invalid_query",           "Invalid query parameter")
    ErrInvalidExportFormat    = newE("invalid_export_format",   "Format must be parquet or ndjson")

    // 401 / 403
    ErrAuthMissingToken       = newE("auth_missing_token",      "Authorization required")
    ErrAuthInvalidToken       = newE("auth_invalid_token",      "Invalid or expired token")
    ErrAuthForbidden          = newE("auth_forbidden",          "Insufficient role")

    // 404
    ErrNotFound               = newE("not_found",               "Resource not found")
    ErrLoadNotFound           = newE("load_not_found",          "Load not found")
    ErrExportNotFound         = newE("export_not_found",        "Export not found")
    ErrSnapshotNotFound       = newE("snapshot_not_found",      "Snapshot not found")

    // 409
    ErrLoadAlreadyRunning     = newE("load_already_running",    "Another load is running")
    ErrCannotRetry            = newE("cannot_retry",            "Load is not in failed state")

    // 422
    ErrEntityValidation       = newE("entity_validation_failed", "Validation failed")

    // 503
    ErrSnapshotNotReady       = newE("snapshot_not_ready",      "No committed snapshot yet")
    ErrDBUnavailable          = newE("db_unavailable",          "Database temporarily unavailable")

    // 500
    ErrInternal               = newE("internal",                "Internal server error")

    // Internal-only (не возвращаются клиенту, только в логи/метрики)
    ErrERPUnavailable         = errors.New("erp_unavailable")
    ErrQualityThresholdExceeded = errors.New("quality_threshold_exceeded")
    ErrShutdownSignal         = errors.New("shutdown_signal")
    ErrPartitionMissing       = errors.New("partition_missing_for_event_date")
)

type DomainError struct {
    Code    string
    Message string
    Wrapped error
    Details map[string]any
}

func (e *DomainError) Error() string { return e.Code + ": " + e.Message }
func (e *DomainError) Unwrap() error { return e.Wrapped }

func newE(code, msg string) *DomainError { return &DomainError{Code: code, Message: msg} }

func (e *DomainError) WithDetails(err error) *DomainError {
    cp := *e
    if err != nil {
        cp.Wrapped = err
        cp.Details = map[string]any{"underlying": err.Error()}
    }
    return &cp
}

func (e *DomainError) WithField(field string, value any) *DomainError {
    cp := *e
    if cp.Details == nil { cp.Details = map[string]any{} }
    cp.Details[field] = value
    return &cp
}
```

## 3. Mapping таблица: sentinel → HTTP + supportMessage

| Sentinel | HTTP | code | supportMessage | Когда |
|---|---|---|---|---|
| `ErrAuthMissingToken` | 401 | `auth_missing_token` | `SA-AUTH-001` | Нет Authorization header |
| `ErrAuthInvalidToken` | 401 | `auth_invalid_token` | `SA-AUTH-002` | Неверная подпись / expired |
| `ErrAuthForbidden` | 403 | `auth_forbidden` | `SA-AUTH-003` | Валидный JWT, но не та роль |
| `ErrBadRequest` | 400 | `bad_request` | `SA-REQ-001` | Bind().Query / Body fail |
| `ErrInvalidCursor` | 400 | `invalid_cursor` | `SA-REQ-002` | cursor не парсится |
| `ErrInvalidQuery` | 400 | `invalid_query` | `SA-REQ-003` | filters противоречат |
| `ErrInvalidExportFormat` | 400 | `invalid_export_format` | `SA-EXP-001` | format ∉ [parquet,ndjson] |
| `ErrNotFound` | 404 | `not_found` | `SA-RES-001` | Generic |
| `ErrLoadNotFound` | 404 | `load_not_found` | `SA-LOAD-001` | GET /admin/loads/{id} |
| `ErrExportNotFound` | 404 | `export_not_found` | `SA-EXP-002` | GET /v1/exports/{id} |
| `ErrSnapshotNotFound` | 404 | `snapshot_not_found` | `SA-SNAP-001` | GET /v1/snapshots/{id} |
| `ErrLoadAlreadyRunning` | 409 | `load_already_running` | `SA-LOAD-002` | POST /admin/loads, advisory lock taken |
| `ErrCannotRetry` | 409 | `cannot_retry` | `SA-LOAD-003` | POST /admin/loads/{id}/retry, status != failed |
| `ErrEntityValidation` | 422 | `entity_validation_failed` | `SA-VAL-001` | (зарезервировано на будущее, на запись через API) |
| `ErrSnapshotNotReady` | 503 + Retry-After:60 | `snapshot_not_ready` | `SA-SNAP-002` | Первый запуск, ещё нет committed snapshot |
| `ErrDBUnavailable` | 503 | `db_unavailable` | `SA-DB-001` | pgxpool ping failed |
| `ErrInternal` | 500 | `internal` | `SA-INT-001` | Любой непредусмотренный |

### Internal sentinels (не доходят до клиента)

| Sentinel | Лог | Метрика | Действие |
|---|---|---|---|
| `ErrERPUnavailable` | ERROR | `load_failed_total{reason="erp_unavailable"}` | `loads.status=failed`, alert |
| `ErrQualityThresholdExceeded` | ERROR | `load_failed_total{reason="quality_threshold_exceeded"}` | `loads.status=failed`, alert |
| `ErrShutdownSignal` | INFO | `load_failed_total{reason="shutdown_signal"}` | Load failed без alert |
| `ErrPartitionMissing` | ERROR | `load_failed_total{reason="partition_missing"}` | Алерт IT, нужна ручная DDL |

## 4. WriteJSON helper

```go
// pkg/errorspkg/write.go
package errorspkg

import (
    "errors"
    "github.com/gofiber/fiber/v3"
)

var supportCodes = map[string]string{
    "auth_missing_token":        "SA-AUTH-001",
    "auth_invalid_token":        "SA-AUTH-002",
    "auth_forbidden":            "SA-AUTH-003",
    "bad_request":               "SA-REQ-001",
    "invalid_cursor":            "SA-REQ-002",
    "invalid_query":             "SA-REQ-003",
    "invalid_export_format":     "SA-EXP-001",
    "not_found":                 "SA-RES-001",
    "load_not_found":            "SA-LOAD-001",
    "export_not_found":          "SA-EXP-002",
    "snapshot_not_found":        "SA-SNAP-001",
    "load_already_running":      "SA-LOAD-002",
    "cannot_retry":              "SA-LOAD-003",
    "entity_validation_failed":  "SA-VAL-001",
    "snapshot_not_ready":        "SA-SNAP-002",
    "db_unavailable":            "SA-DB-001",
    "internal":                  "SA-INT-001",
}

func WriteJSON(c fiber.Ctx, status int, err error) error {
    var de *DomainError
    if !errors.As(err, &de) {
        de = ErrInternal.WithDetails(err)
    }
    traceID, _ := c.Locals("request_id").(string)
    body := ErrorResponseJSON{
        Code:           de.Code,
        Message:        de.Message,
        SupportMessage: supportCodes[de.Code],
        TraceID:        traceID,
        Details:        de.Details,
    }
    return c.Status(status).JSON(body)
}
```

## 5. Fiber ErrorHandler (catch-all)

```go
// internal/middleware/error_handler.go
func FiberErrorHandler(log *slog.Logger) fiber.ErrorHandler {
    return func(c fiber.Ctx, err error) error {
        var de *errorspkg.DomainError
        if errors.As(err, &de) {
            // Уже типизированная — handler сам обернул
            log.Warn("domain error", "code", de.Code, "path", c.Path(), "err", err)
            return errorspkg.WriteJSON(c, statusFromDomain(de), de)
        }

        // Fiber's default
        var fe *fiber.Error
        if errors.As(err, &fe) {
            return errorspkg.WriteJSON(c, fe.Code, errorspkg.ErrInternal.WithDetails(err))
        }

        // Незакаталогизированная — 500
        log.Error("unhandled error", "path", c.Path(), "err", err)
        return errorspkg.WriteJSON(c, fiber.StatusInternalServerError, errorspkg.ErrInternal.WithDetails(err))
    }
}

func statusFromDomain(d *errorspkg.DomainError) int {
    switch d.Code {
    case "auth_missing_token", "auth_invalid_token":         return 401
    case "auth_forbidden":                                    return 403
    case "bad_request", "invalid_cursor", "invalid_query",
         "invalid_export_format":                             return 400
    case "not_found", "load_not_found",
         "export_not_found", "snapshot_not_found":            return 404
    case "load_already_running", "cannot_retry":              return 409
    case "entity_validation_failed":                          return 422
    case "snapshot_not_ready", "db_unavailable":              return 503
    default:                                                  return 500
    }
}
```

## 6. Примеры ответов

### 401 (нет JWT)

```json
{
  "code": "auth_missing_token",
  "message": "Authorization required",
  "supportMessage": "SA-AUTH-001",
  "traceId": "8c1a4d2f-1234-5678-9abc-def012345678"
}
```

### 409 (параллельный load)

```json
{
  "code": "load_already_running",
  "message": "Another load is running",
  "supportMessage": "SA-LOAD-002",
  "traceId": "...",
  "details": {
    "currentLoadId": "8c1a4d2f-..."
  }
}
```

### 503 (нет snapshot)

```
HTTP/1.1 503 Service Unavailable
Retry-After: 60
Content-Type: application/json

{
  "code": "snapshot_not_ready",
  "message": "No committed snapshot yet",
  "supportMessage": "SA-SNAP-002",
  "traceId": "..."
}
```

### 400 (плохой query)

```json
{
  "code": "bad_request",
  "message": "Invalid request",
  "supportMessage": "SA-REQ-001",
  "traceId": "...",
  "details": {
    "underlying": "strconv.Atoi: parsing 'abc': invalid syntax"
  }
}
```

## 7. Логирование ошибок

| Severity | Куда |
|---|---|
| 4xx (кроме 401/403) | WARN, без stacktrace |
| 401/403 | INFO (для статистики атак — Counter `auth_failures_total`) |
| 5xx | ERROR + полный stacktrace через `slog` (`slog.Any("err", err)`) |
| Internal sentinels | ERROR + структурированные поля (`load_id`, `entity`) |
