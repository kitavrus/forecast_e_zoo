package dto

import (
	"time"

	"github.com/google/uuid"
)

// PostExportRequest — POST /v1/exports.
type PostExportRequest struct {
	Entity     string            `json:"entity" validate:"required,oneof=products product_barcodes category location supplier supply_spec promo order_rule supply_plan store_assortment master_change_log receipt_line location_stock_snapshot stock_movement supplier_stock_snapshot"`
	Format     string            `json:"format" validate:"required,oneof=ndjson parquet"`
	Filters    map[string]string `json:"filters,omitempty"`
	SnapshotID uuid.UUID         `json:"snapshot_id" validate:"required"`
}

// PostExportResponse — POST /v1/exports response.
type PostExportResponse struct {
	ExportID uuid.UUID `json:"export_id"`
	Status   string    `json:"status"`
	Location string    `json:"location,omitempty"`
}

// GetExportResponse — GET /v1/exports/{id}.
type GetExportResponse struct {
	ID         uuid.UUID  `json:"id"`
	Entity     string     `json:"entity"`
	SnapshotID uuid.UUID  `json:"snapshot_id"`
	Format     string     `json:"format"`
	Status     string     `json:"status"`
	Location   *string    `json:"location,omitempty"`
	SizeBytes  *int64     `json:"size_bytes,omitempty"`
	Error      *string    `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}
