// Package dto — request/response DTO фичи forecast.
package dto

import (
	"time"

	"github.com/google/uuid"
)

// RunItem — DTO одной строки forecast_runs для API.
type RunItem struct {
	ID               uuid.UUID  `json:"id"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	Status           string     `json:"status" enums:"running,committed,failed"`
	HorizonDays      int        `json:"horizon_days"`
	SnapshotEtlRunID *uuid.UUID `json:"snapshot_etl_run_id,omitempty"`
	ForecastsCount   int        `json:"forecasts_count"`
	LinesCount       int        `json:"lines_count"`
	PlansCount       int        `json:"plans_count"`
	ErrorMessage     *string    `json:"error_message,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ListRunsResponse — ответ GET /v1/forecast/runs.
type ListRunsResponse struct {
	Items      []RunItem `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// RefreshRunRequest — body POST /v1/forecast/runs/refresh.
type RefreshRunRequest struct {
	HorizonDays *int `json:"horizon_days,omitempty"`
}

// RefreshRunResponse — ответ POST /v1/forecast/runs/refresh.
type RefreshRunResponse struct {
	RunID       uuid.UUID `json:"run_id"`
	Started     bool      `json:"started"`
	HorizonDays int       `json:"horizon_days"`
}

// PlanItem — DTO одной строки replenishment_plans.
type PlanItem struct {
	ID          uuid.UUID  `json:"id"`
	RunID       uuid.UUID  `json:"run_id"`
	SupplierID  string     `json:"supplier_id"`
	LocationID  string     `json:"location_id"`
	PlanDate    string     `json:"plan_date"` // YYYY-MM-DD
	TotalQty    float64    `json:"total_qty"`
	LinesCount  int        `json:"lines_count"`
	Status      string     `json:"status" enums:"draft,approved,cancelled"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	ApprovedBy  *string    `json:"approved_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListPlansResponse — ответ GET /v1/replenishment/plans.
type ListPlansResponse struct {
	Items      []PlanItem `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// CalculationLineItem — DTO одной calculation_line для plan-with-lines.
type CalculationLineItem struct {
	ID            uuid.UUID `json:"id"`
	ProductID     string    `json:"product_id"`
	LocationID    string    `json:"location_id"`
	SupplierID    *string   `json:"supplier_id,omitempty"`
	CurrentStock  float64   `json:"current_stock"`
	InTransit     float64   `json:"in_transit"`
	DailyDemand   float64   `json:"daily_demand"`
	LeadTimeDays  int       `json:"lead_time_days"`
	SafetyStock   float64   `json:"safety_stock"`
	ReorderPoint  float64   `json:"reorder_point"`
	TargetStock   float64   `json:"target_stock"`
	ReorderQty    float64   `json:"reorder_qty"`
}

// PlanWithLinesResponse — ответ GET /v1/replenishment/plans/:id.
type PlanWithLinesResponse struct {
	Plan  PlanItem              `json:"plan"`
	Lines []CalculationLineItem `json:"lines"`
}

// ApprovePlanRequest — body POST /v1/replenishment/plans/:id/approve.
type ApprovePlanRequest struct {
	ApprovedBy string `json:"approved_by"`
}

// ApprovePlanResponse — ответ approve.
type ApprovePlanResponse struct {
	Plan PlanItem `json:"plan"`
}
