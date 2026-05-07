package errorspkg

import "net/http"

// --- Sentinel-ошибки Модуля 2 (etl-validation, EV-*) ---
//
// См. docs/features/etl-validation/design-errors.md §4.

var (
	// ErrEtlRunAlreadyRunning — 409, попытка запустить ETL пока другой run выполняется.
	ErrEtlRunAlreadyRunning = &Error{
		Code:           "etl_run_already_running",
		Message:        "another etl run is already running",
		SupportMessage: SupportEtlRunAlreadyRunning,
		HTTP:           http.StatusConflict,
	}

	// ErrEtlRunNotFound — 404, GET /admin/etl/runs/{id} → run не найден.
	ErrEtlRunNotFound = &Error{
		Code:           "etl_run_not_found",
		Message:        "etl run not found",
		SupportMessage: SupportEtlRunNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrCannotRetryEtl — 409, retry над run со status != failed.
	ErrCannotRetryEtl = &Error{
		Code:           "cannot_retry_etl",
		Message:        "etl run cannot be retried",
		SupportMessage: SupportCannotRetryEtl,
		HTTP:           http.StatusConflict,
	}

	// ErrSourceUnavailable — 502, source-adapter недоступен / вернул 5xx.
	ErrSourceUnavailable = &Error{
		Code:           "source_unavailable",
		Message:        "source adapter unavailable",
		SupportMessage: SupportSourceUnavailable,
		HTTP:           http.StatusBadGateway,
	}

	// ErrMartRefreshNotSupported — 400, попытка refresh поддерживаемого только в полном run mart'а.
	ErrMartRefreshNotSupported = &Error{
		Code:           "mart_refresh_not_supported",
		Message:        "mart refresh not supported",
		SupportMessage: SupportMartRefreshNotSupported,
		HTTP:           http.StatusBadRequest,
	}
)
