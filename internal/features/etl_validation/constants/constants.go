// Package constants содержит общедоступные литералы фичи etl_validation:
// статусы run, kind, trigger, severity reject_log, имена mart-ов.
package constants

// EtlRun statuses.
const (
	StatusRunning   = "running"
	StatusCommitted = "committed"
	StatusFailed    = "failed"
	StatusAborted   = "aborted"
)

// EtlRun kinds.
const (
	KindFull        = "full"
	KindMartRefresh = "mart_refresh"
)

// EtlRun triggers.
const (
	TriggerCron  = "cron"
	TriggerAdmin = "admin"
	TriggerRetry = "retry"
)

// Reject log severities.
const (
	SeverityCritical = "critical"
	SeveritySoft     = "soft"
)

// Mart names — служат target_mart в etl_runs.kind='mart_refresh'.
const (
	MartDemandHistory     = "mart_demand_history"
	MartCalculationInput  = "mart_calculation_input"
	MartKpiDaily          = "mart_kpi_daily"
	MartMasterCurrent     = "mart_master_current"
	MartSupplierScorecard = "mart_supplier_scorecard"
)

// EtlRunStatuses — допустимые значения поля status (см. marts.etl_runs CHECK).
//
//nolint:gochecknoglobals // публичный enum-список, используется в валидаторах и sync-тестах.
var EtlRunStatuses = []string{StatusRunning, StatusCommitted, StatusFailed, StatusAborted}

// EtlRunKinds — допустимые значения поля kind.
//
//nolint:gochecknoglobals // публичный enum-список.
var EtlRunKinds = []string{KindFull, KindMartRefresh}

// EtlRunTriggers — допустимые значения поля trigger.
//
//nolint:gochecknoglobals // публичный enum-список.
var EtlRunTriggers = []string{TriggerCron, TriggerAdmin, TriggerRetry}

// RejectSeverities — допустимые значения reject_log.severity.
//
//nolint:gochecknoglobals // публичный enum-список.
var RejectSeverities = []string{SeverityCritical, SeveritySoft}

// MartNames — допустимые имена mart-ов для mart_refresh.
//
//nolint:gochecknoglobals // публичный enum-список.
var MartNames = []string{
	MartDemandHistory,
	MartCalculationInput,
	MartKpiDaily,
	MartMasterCurrent,
	MartSupplierScorecard,
}

// MartRefreshable — подмножество mart-ов, поддерживающих ondemand refresh.
// Сейчас только supplier_scorecard (E8/Q-021).
//
//nolint:gochecknoglobals // публичный список.
var MartRefreshable = []string{
	MartSupplierScorecard,
}
