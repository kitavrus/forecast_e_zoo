// Package errorspkg содержит sentinel-ошибки уровня домена и helper для отдачи
// унифицированного JSON-ответа клиенту.
//
// Базовый тип Error реализует error и errors.Is через сравнение Code.
// Каждая ошибка имеет HTTP-код по умолчанию и стабильный supportMessage,
// чтобы поддержка/SRE могли быстро искать причину в логах и runbook.
package errorspkg

import (
	"errors"
	"fmt"
	"net/http"
)

// Detail — деталь валидации/ошибки (на поле, на правило и т.п.).
type Detail struct {
	Field   string `json:"field,omitempty"`
	Rule    string `json:"rule,omitempty"`
	Message string `json:"message,omitempty"`
}

// Error — базовая структура доменной ошибки.
// Code — стабильный машиночитаемый код (snake_case).
// Message — человекочитаемое сообщение (можно показать клиенту).
// SupportMessage — стабильный support-код (например, "SA-AUTH-001"),
// который мапится в runbook и используется для поиска в логах.
// HTTP — рекомендуемый HTTP-статус (используется WriteJSON).
// Wrapped — обёрнутая нижележащая ошибка (для errors.Unwrap).
type Error struct {
	Code           string
	Message        string
	SupportMessage string
	HTTP           int
	Details        []Detail
	Wrapped        error
}

// Error реализует интерфейс error.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Wrapped)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap возвращает обёрнутую ошибку (для errors.Is/As).
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Wrapped
}

// Is сравнивает по Code — позволяет errors.Is(target, ErrNotFound) работать,
// даже если конкретный экземпляр обёрнут (через Wrap/WithDetails).
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e != nil && t != nil && e.Code == t.Code
}

// Wrap возвращает копию ошибки с добавленной обёрткой (cause).
func (e *Error) Wrap(cause error) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Wrapped = cause
	return &cp
}

// WithDetails возвращает копию ошибки с доп. details.
// Используется для накопления подробностей валидации.
func (e *Error) WithDetails(details ...Detail) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Details = append(append([]Detail(nil), e.Details...), details...)
	return &cp
}

// WithMessage возвращает копию ошибки с заменённым Message (Code/HTTP сохраняются).
func (e *Error) WithMessage(msg string) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Message = msg
	return &cp
}

// --- Конструкторы (для ad-hoc создания ошибок в обработчиках/сервисах) ---

// NewBadRequest создаёт ошибку 400 на основе sentinel ErrBadRequest.
func NewBadRequest(msg string) *Error {
	return ErrBadRequest.WithMessage(msg)
}

// NewNotFound создаёт ошибку 404 на основе sentinel ErrNotFound.
func NewNotFound(msg string) *Error {
	return ErrNotFound.WithMessage(msg)
}

// NewConflict создаёт ошибку 409 на основе sentinel ErrConflict.
func NewConflict(msg string) *Error {
	return ErrConflict.WithMessage(msg)
}

// NewUnauthorized создаёт ошибку 401 на основе sentinel ErrUnauthorized.
func NewUnauthorized(msg string) *Error {
	return ErrUnauthorized.WithMessage(msg)
}

// NewForbidden создаёт ошибку 403 на основе sentinel ErrForbidden.
func NewForbidden(msg string) *Error {
	return ErrForbidden.WithMessage(msg)
}

// NewServiceUnavailable создаёт ошибку 503 на основе sentinel ErrServiceUnavailable.
func NewServiceUnavailable(msg string) *Error {
	return ErrServiceUnavailable.WithMessage(msg)
}

// NewInternal создаёт ошибку 500 на основе sentinel ErrInternal.
func NewInternal(msg string) *Error {
	return ErrInternal.WithMessage(msg)
}

// --- Sentinel-ошибки фазы 01 (минимальный набор) ---
//
// Остальные доменные sentinel'ы добавляются в своих фазах.

var (
	// ErrBadRequest — 400, плохой запрос/валидация.
	ErrBadRequest = &Error{
		Code:           "bad_request",
		Message:        "Bad request",
		SupportMessage: SupportBadRequest,
		HTTP:           http.StatusBadRequest,
	}

	// ErrUnauthorized — 401, отсутствует/невалидный JWT.
	ErrUnauthorized = &Error{
		Code:           "unauthorized",
		Message:        "Unauthorized",
		SupportMessage: SupportUnauthorized,
		HTTP:           http.StatusUnauthorized,
	}

	// ErrForbidden — 403, недостаточно прав.
	ErrForbidden = &Error{
		Code:           "forbidden",
		Message:        "Forbidden",
		SupportMessage: SupportForbidden,
		HTTP:           http.StatusForbidden,
	}

	// ErrNotFound — 404, ресурс не найден.
	ErrNotFound = &Error{
		Code:           "not_found",
		Message:        "Resource not found",
		SupportMessage: SupportNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrConflict — 409, конфликт (повторный create / параллельный load и т.п.).
	ErrConflict = &Error{
		Code:           "conflict",
		Message:        "Conflict",
		SupportMessage: SupportConflict,
		HTTP:           http.StatusConflict,
	}

	// ErrServiceUnavailable — 503, сервис временно недоступен (например, snapshot ещё не готов).
	ErrServiceUnavailable = &Error{
		Code:           "service_unavailable",
		Message:        "Service unavailable",
		SupportMessage: SupportServiceUnavailable,
		HTTP:           http.StatusServiceUnavailable,
	}

	// ErrInternal — 500, внутренняя ошибка.
	ErrInternal = &Error{
		Code:           "internal",
		Message:        "Internal server error",
		SupportMessage: SupportInternal,
		HTTP:           http.StatusInternalServerError,
	}

	// ErrAuthMissingToken — 401, отсутствует Authorization header.
	// Code совпадает с ErrAuthInvalidToken, чтобы не раскрывать клиенту,
	// что именно не так (защита от user-enumeration attacks).
	ErrAuthMissingToken = &Error{
		Code:           "auth_invalid_token",
		Message:        "auth required",
		SupportMessage: SupportUnauthorized,
		HTTP:           http.StatusUnauthorized,
	}

	// ErrAuthInvalidToken — 401, токен есть, но невалидный/просрочен/чужой alg.
	ErrAuthInvalidToken = &Error{
		Code:           "auth_invalid_token",
		Message:        "invalid token",
		SupportMessage: SupportUnauthorized,
		HTTP:           http.StatusUnauthorized,
	}

	// ErrAuthForbidden — 403, токен валиден, но роль не подходит.
	ErrAuthForbidden = &Error{
		Code:           "auth_forbidden",
		Message:        "forbidden",
		SupportMessage: SupportForbidden,
		HTTP:           http.StatusForbidden,
	}

	// ErrInvalidCursor — 400, cursor не парсится / повреждён.
	ErrInvalidCursor = &Error{
		Code:           "invalid_cursor",
		Message:        "invalid pagination cursor",
		SupportMessage: SupportInvalidCursor,
		HTTP:           http.StatusBadRequest,
	}

	// ErrInvalidQuery — 400, query-параметры противоречат друг другу/диапазону.
	ErrInvalidQuery = &Error{
		Code:           "invalid_query",
		Message:        "invalid query parameters",
		SupportMessage: SupportInvalidQuery,
		HTTP:           http.StatusBadRequest,
	}

	// ErrInvalidExportFormat — 400, format ∉ {ndjson, parquet}.
	ErrInvalidExportFormat = &Error{
		Code:           "invalid_export_format",
		Message:        "unsupported export format",
		SupportMessage: SupportInvalidExportFormat,
		HTTP:           http.StatusBadRequest,
	}

	// ErrLoadNotFound — 404, GET /admin/loads/{id} → load не найден.
	ErrLoadNotFound = &Error{
		Code:           "load_not_found",
		Message:        "load not found",
		SupportMessage: SupportLoadNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrSnapshotNotFound — 404, GET /v1/snapshots/{id} → snapshot не найден.
	ErrSnapshotNotFound = &Error{
		Code:           "snapshot_not_found",
		Message:        "snapshot not found",
		SupportMessage: SupportSnapshotNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrSnapshotNotReady — 503 + Retry-After:60. Первый запуск, ещё нет committed snapshot.
	ErrSnapshotNotReady = &Error{
		Code:           "snapshot_not_ready",
		Message:        "snapshot not ready",
		SupportMessage: SupportSnapshotNotReady,
		HTTP:           http.StatusServiceUnavailable,
	}

	// ErrLoadAlreadyRunning — 409, POST /admin/loads пока другой load выполняется.
	ErrLoadAlreadyRunning = &Error{
		Code:           "load_already_running",
		Message:        "another load is already running",
		SupportMessage: SupportLoadAlreadyRunning,
		HTTP:           http.StatusConflict,
	}

	// ErrCannotRetry — 409, POST /admin/loads/{id}/retry над load со status != failed.
	ErrCannotRetry = &Error{
		Code:           "cannot_retry",
		Message:        "load cannot be retried",
		SupportMessage: SupportCannotRetry,
		HTTP:           http.StatusConflict,
	}

	// ErrAlreadyExists — 409, INSERT нарушил unique-constraint (pgconn 23505).
	ErrAlreadyExists = &Error{
		Code:           "already_exists",
		Message:        "resource already exists",
		SupportMessage: SupportAlreadyExists,
		HTTP:           http.StatusConflict,
	}

	// ErrQualityThresholdExceeded — internal sentinel: lines_failed/lines_total > 1%.
	// До клиента не доходит (load mark failed), но используется в логах/тестах.
	ErrQualityThresholdExceeded = &Error{
		Code:           "quality_threshold_exceeded",
		Message:        "quality threshold exceeded (>1% rows critical)",
		SupportMessage: SupportQualityThresholdExceeded,
		HTTP:           http.StatusInternalServerError,
	}

	// ErrERPUnavailable — internal sentinel: SourceReader вернул ошибку.
	ErrERPUnavailable = &Error{
		Code:           "erp_unavailable",
		Message:        "ERP source unavailable",
		SupportMessage: SupportERPUnavailable,
		HTTP:           http.StatusInternalServerError,
	}
)
