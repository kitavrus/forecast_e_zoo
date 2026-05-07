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
)
