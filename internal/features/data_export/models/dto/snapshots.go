package dto

import (
	"time"

	"github.com/google/uuid"
)

// EntitySummary — сводка по одной сущности в рамках snapshot.
type EntitySummary struct {
	Entity     string `json:"entity"`
	RowCount   int64  `json:"row_count"`
	BytesTotal int64  `json:"bytes_total,omitempty"`
}

// GetSnapshotsCurrentResponse — GET /v1/snapshots/current.
type GetSnapshotsCurrentResponse struct {
	SnapshotID  uuid.UUID       `json:"snapshot_id"`
	CommittedAt time.Time       `json:"committed_at"`
	Entities    []EntitySummary `json:"entities"`
}
