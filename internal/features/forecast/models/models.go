// Package models — domain types фичи forecast (Module 5).
package models

import (
	"time"

	"github.com/google/uuid"
)

// InsertRunInput — параметры InsertRun (общий тип для repository ↔ engine).
type InsertRunInput struct {
	HorizonDays      int
	SnapshotEtlRunID *uuid.UUID
}

// ForecastRun — одна строка таблицы forecast.forecast_runs.
type ForecastRun struct {
	ID                 uuid.UUID
	StartedAt          time.Time
	FinishedAt         *time.Time
	Status             string
	HorizonDays        int
	SnapshotEtlRunID   *uuid.UUID
	ForecastsCount     int
	LinesCount         int
	PlansCount         int
	ErrorMessage       *string
	CreatedAt          time.Time
}

// RunFilter — фильтр для list-запроса runs.
type RunFilter struct {
	Status *string
	From   *time.Time
	To     *time.Time
	Limit  int
	Cursor string
}

// Forecast — одна строка таблицы forecast.forecasts.
type Forecast struct {
	RunID        uuid.UUID
	ProductID    string
	LocationID   string
	ForecastDate time.Time
	ForecastQty  float64
	LowerBound   *float64
	UpperBound   *float64
	ModelName    string
	Confidence   *float64
	CreatedAt    time.Time
}

// CalculationLine — одна строка таблицы forecast.calculation_lines.
type CalculationLine struct {
	ID            uuid.UUID
	RunID         uuid.UUID
	ProductID     string
	LocationID    string
	SupplierID    *string
	CurrentStock  float64
	InTransit     float64
	DailyDemand   float64
	LeadTimeDays  int
	SafetyStock   float64
	ReorderPoint  float64
	TargetStock   float64
	ReorderQty    float64
	CalculatedAt  time.Time
}

// ReplenishmentPlan — одна строка таблицы forecast.replenishment_plans.
type ReplenishmentPlan struct {
	ID          uuid.UUID
	RunID       uuid.UUID
	SupplierID  string
	LocationID  string
	PlanDate    time.Time
	TotalQty    float64
	LinesCount  int
	Status      string
	ApprovedAt  *time.Time
	ApprovedBy  *string
	CreatedAt   time.Time
}

// PlanFilter — фильтр для list-запроса plans.
type PlanFilter struct {
	SupplierID *string
	LocationID *string
	PlanDate   *time.Time
	Status     *string
	Limit      int
	Cursor     string
}

// PlanWithLines — план + рассчитанные строки.
type PlanWithLines struct {
	Plan  ReplenishmentPlan
	Lines []CalculationLine
}

// --- Mart-aggregate types — то, что engine читает из marts.* ---

// CalcInputRow — строка из marts.mart_calculation_input.
type CalcInputRow struct {
	ProductID     string
	LocationID    string
	OnHand        float64
	InTransit     float64
	DailyDemand   *float64
	SupplierID    *string
	LeadTimeDays  *int
	SafetyStock   *float64
	MinQty        *float64
	MaxQty        *float64
}

// DemandPoint — одна точка истории спроса для Forecaster.
type DemandPoint struct {
	ProductID  string
	LocationID string
	AsOfDate   time.Time
	QtySold    float64
}

// SupplierScore — агрегат supplier scorecard для lead_time fallback.
type SupplierScore struct {
	SupplierID         string
	LeadTimeActualAvg  *float64
	FillRateAvg        *float64
}
