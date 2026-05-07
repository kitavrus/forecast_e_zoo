package errorspkg

import "net/http"

// --- Sentinel-ошибки Модуля 5 (forecast-engine, FCT-*) ---
//
// См. docs/features/forecast-engine/design.md §4.

var (
	// ErrForecastRunNotFound — 404, GET /v1/forecast/runs/{id} → run не найден.
	ErrForecastRunNotFound = &Error{
		Code:           "forecast_run_not_found",
		Message:        "forecast run not found",
		SupportMessage: SupportForecastRunNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrForecastRunInProgress — 409, POST refresh → advisory lock busy.
	ErrForecastRunInProgress = &Error{
		Code:           "forecast_run_in_progress",
		Message:        "another forecast run is in progress",
		SupportMessage: SupportForecastRunInProgress,
		HTTP:           http.StatusConflict,
	}

	// ErrPlanNotFound — 404, GET plan/{id} → отсутствует.
	ErrPlanNotFound = &Error{
		Code:           "plan_not_found",
		Message:        "replenishment plan not found",
		SupportMessage: SupportPlanNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrPlanNotDraft — 409, approve/cancel при неподходящем статусе.
	ErrPlanNotDraft = &Error{
		Code:           "plan_not_draft",
		Message:        "plan is not in draft status",
		SupportMessage: SupportPlanNotDraft,
		HTTP:           http.StatusConflict,
	}

	// ErrInvalidHorizon — 400, horizon_days вне допустимого диапазона.
	ErrInvalidHorizon = &Error{
		Code:           "invalid_horizon",
		Message:        "horizon_days must be in [1, 60]",
		SupportMessage: SupportInvalidHorizon,
		HTTP:           http.StatusBadRequest,
	}

	// ErrInvalidPlanStatus — 400, invalid query ?status=.
	ErrInvalidPlanStatus = &Error{
		Code:           "invalid_plan_status",
		Message:        "invalid plan status (allowed: draft|approved|cancelled)",
		SupportMessage: SupportInvalidPlanStatus,
		HTTP:           http.StatusBadRequest,
	}

	// ErrForecastSchedulerUnavailable — 503, scheduler не сконфигурирован.
	ErrForecastSchedulerUnavailable = &Error{
		Code:           "forecast_scheduler_unavailable",
		Message:        "forecast scheduler is not configured",
		SupportMessage: SupportForecastSchedulerUnavailable,
		HTTP:           http.StatusServiceUnavailable,
	}
)
