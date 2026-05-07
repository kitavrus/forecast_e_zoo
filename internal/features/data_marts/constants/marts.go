// Package constants содержит константы фичи data_marts:
// набор имён mart'ов (whitelist), параметры пагинации и TTL кэша.
package constants

// Имена 5 mart-таблиц в schema marts.
// Должны совпадать с миграцией Модуля 2 (1001_marts_schema.up.sql).
const (
	MartDemandHistory     = "mart_demand_history"
	MartCalculationInput  = "mart_calculation_input"
	MartKpiDaily          = "mart_kpi_daily"
	MartMasterCurrent     = "mart_master_current"
	MartSupplierScorecard = "mart_supplier_scorecard"
)

// MartNames — whitelist всех mart'ов.
// Используется для validation `:name` в handler'ах.
var MartNames = []string{
	MartDemandHistory,
	MartCalculationInput,
	MartKpiDaily,
	MartMasterCurrent,
	MartSupplierScorecard,
}

// IsKnownMart возвращает true, если name в whitelist.
func IsKnownMart(name string) bool {
	for _, m := range MartNames {
		if m == name {
			return true
		}
	}
	return false
}

// Параметры пагинации.
const (
	LimitDefault = 1000
	LimitMax     = 10000
)

// CacheTTLSeconds — TTL для in-memory cache версий mart'ов.
const CacheTTLSeconds = 60
