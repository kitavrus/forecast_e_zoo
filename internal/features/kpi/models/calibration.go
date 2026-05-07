package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// KpiCalibration — одна строка таблицы kpi.kpi_calibrations.
type KpiCalibration struct {
	ID        uuid.UUID
	KpiName   string
	ScopeType string
	ScopeID   *string
	Params    json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CalibrationFilter — фильтр list calibrations.
type CalibrationFilter struct {
	KpiName   *string
	ScopeType *string
}

// Mart-aggregate types — то, что engine читает из marts.* для расчёта KPI.

// DemandHistoryAgg — агрегация marts.mart_demand_history для OSA.
//
// Поля считаются за окно [from, to].
type DemandHistoryAgg struct {
	ProductID   string
	LocationID  string
	DaysObserved int
	DaysOOS     int
}

// CalcInputRow — строка из marts.mart_calculation_input для Stock Days.
type CalcInputRow struct {
	ProductID   string
	LocationID  string
	OnHand      float64
	InTransit   float64
	DailyDemand *float64
	SupplierID  *string
}

// SupplierScorecardRow — строка из marts.mart_supplier_scorecard для OTIF.
type SupplierScorecardRow struct {
	SupplierID      string
	WeekStart       time.Time
	LinesDelivered  int
	LinesLate       int
	QtyShortTotal   float64
	FillRateAvg     *float64
}
