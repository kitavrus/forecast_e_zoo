// Package constants — константы фичи forecast (Forecast Engine, Module 5).
package constants

// Run statuses (forecast.forecast_runs.status).
const (
	RunStatusRunning   = "running"
	RunStatusCommitted = "committed"
	RunStatusFailed    = "failed"
)

// RunStatuses — все валидные значения для validators/swagger enums.
var RunStatuses = []string{RunStatusRunning, RunStatusCommitted, RunStatusFailed}

// IsKnownRunStatus — true если значение допустимо.
func IsKnownRunStatus(s string) bool {
	for _, v := range RunStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// Plan statuses (forecast.replenishment_plans.status).
const (
	PlanStatusDraft     = "draft"
	PlanStatusApproved  = "approved"
	PlanStatusCancelled = "cancelled"
)

// PlanStatuses — все валидные значения.
var PlanStatuses = []string{PlanStatusDraft, PlanStatusApproved, PlanStatusCancelled}

// IsKnownPlanStatus — true если значение допустимо.
func IsKnownPlanStatus(s string) bool {
	for _, v := range PlanStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// Forecaster model names.
const (
	ModelSMASeasonal = "sma_seasonal"
)

// AdvisoryLockKey — ключ pg_advisory_lock для forecast engine run.
// Значение взято из ASCII bytes "FCTERGNE" (Forecast Engine).
// Должно быть стабильным: переименование = пересечение с другими scheduler'ами.
const AdvisoryLockKey int64 = 0x4643544552474E45 // "FCTERGNE"

// Параметры запросов и пагинации.
const (
	LimitDefault = 100
	LimitMax     = 1000

	// HorizonDefault — дефолтный горизонт прогноза в днях.
	HorizonDefault = 14
	// HorizonMin / HorizonMax — допустимый диапазон.
	HorizonMin = 1
	HorizonMax = 60

	// LookbackDays — окно истории для SMA (в днях).
	LookbackDays = 30

	// LeadTimeDefault — fallback lead time в днях, если supplier scorecard отсутствует.
	LeadTimeDefault = 7

	// SafetyStockZ95 — z-score для 95% service level.
	SafetyStockZ95 = 1.96

	// CycleStockDays — длина order cycle для cycle stock (дней).
	CycleStockDays = 7

	// ConfidenceMVP — placeholder confidence для MVP forecaster.
	ConfidenceMVP = 0.8

	// ConfidenceBoundDelta — relative ± bound от forecast (0.2 = ±20%).
	ConfidenceBoundDelta = 0.2
)
