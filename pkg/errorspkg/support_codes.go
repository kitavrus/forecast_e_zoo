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
)
