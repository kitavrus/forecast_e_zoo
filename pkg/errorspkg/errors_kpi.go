package errorspkg

import "net/http"

// --- Sentinel-ошибки Модуля 4 (kpi-calibration, KPI-*) ---
//
// См. docs/features/kpi-calibration/design.md §5.

var (
	// ErrKpiSnapshotNotFound — 404, GET /v1/kpi/snapshots/{id} → snapshot не найден.
	ErrKpiSnapshotNotFound = &Error{
		Code:           "kpi_snapshot_not_found",
		Message:        "kpi snapshot not found",
		SupportMessage: SupportKpiSnapshotNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrKpiCalibrationNotFound — 404, GET/PUT /v1/kpi/calibrations/{id} → калибровка не найдена.
	ErrKpiCalibrationNotFound = &Error{
		Code:           "kpi_calibration_not_found",
		Message:        "kpi calibration not found",
		SupportMessage: SupportKpiCalibrationNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrInvalidKpiName — 400, ?kpi_name= ∉ {osa,otif,stock_days}.
	ErrInvalidKpiName = &Error{
		Code:           "invalid_kpi_name",
		Message:        "invalid kpi name",
		SupportMessage: SupportInvalidKpiName,
		HTTP:           http.StatusBadRequest,
	}
)
