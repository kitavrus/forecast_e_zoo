package errorspkg

// Support-коды для runbook поддержки и SRE.
// Префикс "SA-" = Source Adapter. Дальше — модуль и порядковый номер.
//
// Коды должны быть стабильными: переименование = breaking change для саппорта.
const (
	// Generic
	SupportBadRequest         = "SA-REQ-001"
	SupportUnauthorized       = "SA-AUTH-001"
	SupportForbidden          = "SA-AUTH-002"
	SupportNotFound           = "SA-RES-001"
	SupportConflict           = "SA-CONF-001"
	SupportServiceUnavailable = "SA-SVC-001"
	SupportInternal           = "SA-INT-001"
	SupportNotImplemented     = "SA-INT-002"

	// Request validators (phase 07)
	SupportInvalidCursor       = "SA-REQ-002"
	SupportInvalidQuery        = "SA-REQ-003"
	SupportInvalidExportFormat = "SA-EXP-001"

	// Repository / loads / snapshot (phase 08)
	SupportLoadNotFound       = "SA-LOAD-001"
	SupportLoadAlreadyRunning = "SA-LOAD-002"
	SupportCannotRetry        = "SA-LOAD-003"
	SupportSnapshotNotFound   = "SA-SNAP-001"
	SupportSnapshotNotReady   = "SA-SNAP-002"
	SupportAlreadyExists      = "SA-CONF-002"

	// Loader (phase 10, internal sentinels — не доходят до клиента)
	SupportQualityThresholdExceeded = "SA-LOAD-010"
	SupportERPUnavailable           = "SA-LOAD-011"

	// ETL/Marts module (Module 2 etl-validation, EV-*)
	SupportEtlRunAlreadyRunning    = "EV-001"
	SupportEtlRunNotFound          = "EV-002"
	SupportCannotRetryEtl          = "EV-003"
	SupportSourceUnavailable       = "EV-004"
	SupportMartRefreshNotSupported = "EV-005"

	// KPI module (Module 4 kpi-calibration, KPI-*)
	SupportKpiSnapshotNotFound    = "KPI-001"
	SupportKpiCalibrationNotFound = "KPI-002"
	SupportInvalidKpiName         = "KPI-003"

	// Forecast Engine module (Module 5 forecast-engine, FCT-*)
	SupportForecastRunNotFound          = "FCT-001"
	SupportForecastRunInProgress        = "FCT-002"
	SupportPlanNotFound                 = "FCT-003"
	SupportPlanNotDraft                 = "FCT-004"
	SupportInvalidHorizon               = "FCT-005"
	SupportInvalidPlanStatus            = "FCT-006"
	SupportForecastSchedulerUnavailable = "FCT-007"
)

// SupportMessageCodes — общий список всех допустимых support-кодов.
// Используется в sync-тесте и (опционально) в OpenAPI enums-теге.
var SupportMessageCodes = []string{
	SupportBadRequest,
	SupportUnauthorized,
	SupportForbidden,
	SupportNotFound,
	SupportConflict,
	SupportServiceUnavailable,
	SupportInternal,
	SupportNotImplemented,
	SupportInvalidCursor,
	SupportInvalidQuery,
	SupportInvalidExportFormat,
	SupportLoadNotFound,
	SupportLoadAlreadyRunning,
	SupportCannotRetry,
	SupportSnapshotNotFound,
	SupportSnapshotNotReady,
	SupportAlreadyExists,
	SupportQualityThresholdExceeded,
	SupportERPUnavailable,
	SupportEtlRunAlreadyRunning,
	SupportEtlRunNotFound,
	SupportCannotRetryEtl,
	SupportSourceUnavailable,
	SupportMartRefreshNotSupported,
	SupportKpiSnapshotNotFound,
	SupportKpiCalibrationNotFound,
	SupportInvalidKpiName,
	SupportForecastRunNotFound,
	SupportForecastRunInProgress,
	SupportPlanNotFound,
	SupportPlanNotDraft,
	SupportInvalidHorizon,
	SupportInvalidPlanStatus,
	SupportForecastSchedulerUnavailable,
}
