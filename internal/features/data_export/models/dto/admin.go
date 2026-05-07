package dto

import (
	"time"

	"github.com/google/uuid"
)

// PostLoadRequest — POST /admin/loads.
type PostLoadRequest struct {
	Source string `json:"source" validate:"required,oneof=erp_e_zoo manual retry"`
}

// PostLoadResponse — POST /admin/loads response.
type PostLoadResponse struct {
	LoadID    uuid.UUID `json:"load_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// PostLoadRetryRequest — POST /admin/loads/{id}/retry.
type PostLoadRetryRequest struct {
	LoadID uuid.UUID `json:"load_id" validate:"required"`
}

// PostLoadRetryResponse — POST /admin/loads/{id}/retry response.
type PostLoadRetryResponse struct {
	NewLoadID      uuid.UUID `json:"new_load_id"`
	OriginalLoadID uuid.UUID `json:"original_load_id"`
}

// EntityProgress — прогресс по одной сущности в рамках Load (для GET /admin/loads/{id}).
type EntityProgress struct {
	Entity   string `json:"entity"`
	Total    int64  `json:"total"`
	Inserted int64  `json:"inserted"`
	Updated  int64  `json:"updated"`
	Failed   int64  `json:"failed"`
}

// LoadResponse — flat response Load + entities для GET /admin/loads/{id}.
type LoadResponse struct {
	ID              uuid.UUID        `json:"id"`
	StartedAt       time.Time        `json:"started_at"`
	FinishedAt      *time.Time       `json:"finished_at,omitempty"`
	Status          string           `json:"status"`
	Source          string           `json:"source"`
	FailureReason   *string          `json:"failure_reason,omitempty"`
	ParentLoadID    *uuid.UUID       `json:"parent_load_id,omitempty"`
	EntityProgress  []EntityProgress `json:"entity_progress"`
}

// GetLoadResponse — alias на LoadResponse для совместимости.
type GetLoadResponse = LoadResponse

// GetRejectLogRequest — query GET /admin/reject-log.
type GetRejectLogRequest struct {
	LoadID   string `query:"load_id" validate:"omitempty,uuid"`
	Entity   string `query:"entity" validate:"omitempty"`
	Severity string `query:"severity" validate:"omitempty,oneof=critical soft"`
	Limit    int    `query:"limit" validate:"min=1,max=10000"`
	Cursor   string `query:"cursor" validate:"omitempty,max=1024"`
}

// RejectLogEntry — DTO записи reject_log.
type RejectLogEntry struct {
	ID         uuid.UUID `db:"id" json:"id"`
	LoadID     uuid.UUID `db:"load_id" json:"load_id"`
	Entity     string    `db:"entity" json:"entity"`
	PKValue    []byte    `db:"pk_value" json:"pk_value"`
	Severity   string    `db:"severity" json:"severity"`
	Reason     string    `db:"reason" json:"reason"`
	Raw        []byte    `db:"raw" json:"raw"`
	DetectedAt time.Time `db:"detected_at" json:"detected_at"`
}

// GetRejectLogResponse — response GET /admin/reject-log.
type GetRejectLogResponse = PageResponse[RejectLogEntry]
