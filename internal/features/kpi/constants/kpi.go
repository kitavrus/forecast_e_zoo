// Package constants — константы фичи kpi (KPI engine, Module 4).
package constants

// KPI имена. Определяют, какой калькулятор использовать.
const (
	KpiOSA        = "osa"
	KpiOTIF       = "otif"
	KpiStockDays  = "stock_days"
)

// KpiNames — все известные KPI (для валидации, OpenAPI enums).
var KpiNames = []string{KpiOSA, KpiOTIF, KpiStockDays}

// IsKnownKpi возвращает true если имя соответствует одному из поддерживаемых KPI.
func IsKnownKpi(name string) bool {
	switch name {
	case KpiOSA, KpiOTIF, KpiStockDays:
		return true
	}
	return false
}

// Scope-types для калибровок и снапшотов.
// Иерархия (от specific к generic): product_location > location > supplier > category > global.
const (
	ScopeTypeGlobal          = "global"
	ScopeTypeCategory        = "category"
	ScopeTypeSupplier        = "supplier"
	ScopeTypeLocation        = "location"
	ScopeTypeProductLocation = "product_location"
)

// ScopeTypes — все валидные scope_type значения (для валидации/OpenAPI enums).
var ScopeTypes = []string{
	ScopeTypeGlobal,
	ScopeTypeCategory,
	ScopeTypeSupplier,
	ScopeTypeLocation,
	ScopeTypeProductLocation,
}

// IsKnownScopeType — true если значение допустимо в kpi_calibrations.scope_type.
func IsKnownScopeType(scope string) bool {
	for _, s := range ScopeTypes {
		if s == scope {
			return true
		}
	}
	return false
}

// AdvisoryLockKey — ключ pg_advisory_lock для KPI engine run.
// Значение взято из sha256("kpi-engine-run") урезанного до int64.
// Должно быть стабильным: переименование = пересечение с другими scheduler'ами.
const AdvisoryLockKey int64 = 0x4B50494552474E45 // "KPIERGNE" — KPI ENGine

// Параметры запросов и пагинации.
const (
	LimitDefault = 100
	LimitMax     = 1000
	// QualityErrorBudget — допустимая доля ошибок (5%) на одном KPI run.
	// Превышение → весь KPI помечается failed.
	QualityErrorBudget = 0.05
)
