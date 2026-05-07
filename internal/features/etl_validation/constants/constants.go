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

// Допустимые значения query-параметра entity для GET /admin/reject-log.
// Имена строго совпадают с путями source-adapter (/v1/{entity}, см.
// internal/features/data_export/router/router.go) — single source of truth.
//
// Список из 16 сущностей покрывает полный контракт source-adapter:
// 12 master + 4 fact (см. extractor.FactEntities).
const (
	// Master entities.
	EntityProducts                        = "products"
	EntityProductBarcodes                 = "product_barcodes"
	EntityCategory                        = "category"
	EntityLocation                        = "location"
	EntitySupplier                        = "supplier"
	EntitySupplySpec                      = "supply_spec"
	EntityPromo                           = "promo"
	EntityOrderRule                       = "order_rule"
	EntitySupplyPlan                      = "supply_plan"
	EntityMasterChangeLog                 = "master_change_log"
	EntityStoreAssortment                 = "store_assortment"
	EntityStoreAssortmentLifecycleEvents  = "store_assortment_lifecycle_events"

	// Fact entities (партиционированные, требуют event_date_from/to).
	EntityReceiptLine            = "receipt_line"
	EntityLocationStockSnapshot  = "location_stock_snapshot"
	EntityStockMovement          = "stock_movement"
	EntitySupplierStockSnapshot  = "supplier_stock_snapshot"
)

// AllowedEntities — допустимые значения query-параметра entity для reject_log.
// Полный список из 16 сущностей source-adapter.
//
//nolint:gochecknoglobals // публичный enum-список.
var AllowedEntities = []string{
	// Master.
	EntityProducts,
	EntityProductBarcodes,
	EntityCategory,
	EntityLocation,
	EntitySupplier,
	EntitySupplySpec,
	EntityPromo,
	EntityOrderRule,
	EntitySupplyPlan,
	EntityMasterChangeLog,
	EntityStoreAssortment,
	EntityStoreAssortmentLifecycleEvents,
	// Facts.
	EntityReceiptLine,
	EntityLocationStockSnapshot,
	EntityStockMovement,
	EntitySupplierStockSnapshot,
}

// Лимиты пагинации для admin endpoints.
const (
	EtlRunsListLimitMin     = 1
	EtlRunsListLimitMax     = 100
	EtlRunsListLimitDefault = 50

	RejectLogListLimitMin     = 1
	RejectLogListLimitMax     = 500
	RejectLogListLimitDefault = 100
)
