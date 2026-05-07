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
//
// Имена полей соответствуют source-adapter контракту (design-sql.md):
// snapshot_pointer.current_load_id / committed_at.
type GetSnapshotsCurrentResponse struct {
	CurrentLoadID uuid.UUID       `json:"current_load_id"`
	CommittedAt   time.Time       `json:"committed_at"`
	Entities      []EntitySummary `json:"entities"`
}
