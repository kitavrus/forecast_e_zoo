// Package dto — request/response DTO фичи kpi.
package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SnapshotItem — DTO одной строки kpi_snapshots для API.
type SnapshotItem struct {
	ID            uuid.UUID  `json:"id"`
	AsOfDate      string     `json:"as_of_date"` // YYYY-MM-DD
	KpiName       string     `json:"kpi_name" enums:"osa,otif,stock_days"`
	ScopeType     string     `json:"scope_type" enums:"global,category,supplier,location,product_location"`
	ScopeID       *string    `json:"scope_id,omitempty"`
	Value         float64    `json:"value"`
	CalibrationID *uuid.UUID `json:"calibration_id,omitempty"`
	ComputedAt    time.Time  `json:"computed_at"`
	EtlRunID      *uuid.UUID `json:"etl_run_id,omitempty"`
}

// ListSnapshotsResponse — ответ GET /v1/kpi/snapshots.
type ListSnapshotsResponse struct {
	Items      []SnapshotItem `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// CalibrationItem — DTO одной строки kpi_calibrations.
type CalibrationItem struct {
	ID        uuid.UUID       `json:"id"`
	KpiName   string          `json:"kpi_name" enums:"osa,otif,stock_days"`
	ScopeType string          `json:"scope_type" enums:"global,category,supplier,location,product_location"`
	ScopeID   *string         `json:"scope_id,omitempty"`
	Params    json.RawMessage `json:"params" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ListCalibrationsResponse — ответ GET /v1/kpi/calibrations.
type ListCalibrationsResponse struct {
	Items []CalibrationItem `json:"items"`
}

// UpdateCalibrationRequest — body PUT /v1/kpi/calibrations/:id.
type UpdateCalibrationRequest struct {
	Params json.RawMessage `json:"params" swaggertype:"object"`
}

// RefreshSnapshotsRequest — body POST /v1/kpi/snapshots/refresh.
//
// Все поля optional:
//   - FromDate=null   → as_of_date = today (Europe/Kyiv).
//   - KpiNames=nil    → все три KPI (osa, otif, stock_days).
type RefreshSnapshotsRequest struct {
	FromDate *string  `json:"from_date,omitempty"` // YYYY-MM-DD
	KpiNames []string `json:"kpi_names,omitempty" enums:"osa,otif,stock_days"`
}

// RefreshSnapshotsResponse — ответ POST /v1/kpi/snapshots/refresh.
type RefreshSnapshotsResponse struct {
	RunID    uuid.UUID `json:"run_id"`
	Started  bool      `json:"started"`
	KpiNames []string  `json:"kpi_names"`
	FromDate string    `json:"from_date"`
}
