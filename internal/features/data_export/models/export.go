package models

import (
	"time"

	"github.com/google/uuid"
)

// ExportStatus — статус экспорта файла (NDJSON/Parquet).
type ExportStatus string

const (
	ExportStatusQueued  ExportStatus = "queued"
	ExportStatusRunning ExportStatus = "running"
	ExportStatusReady   ExportStatus = "ready"
	ExportStatusFailed  ExportStatus = "failed"
)

// Export — заявка/результат экспорта одной сущности на снапшоте.
type Export struct {
	ID         uuid.UUID    `db:"id" json:"id"`
	Entity     string       `db:"entity" json:"entity"`
	SnapshotID uuid.UUID    `db:"snapshot_id" json:"snapshot_id"`
	Format     string       `db:"format" json:"format"`
	Status     ExportStatus `db:"status" json:"status"`
	Path       *string      `db:"path" json:"path,omitempty"`
	SizeBytes  *int64       `db:"size_bytes" json:"size_bytes,omitempty"`
	Error      *string      `db:"error" json:"error,omitempty"`
	Requester  string       `db:"requester" json:"requester"`
	CreatedAt  time.Time    `db:"created_at" json:"created_at"`
	StartedAt  *time.Time   `db:"started_at" json:"started_at,omitempty"`
	FinishedAt *time.Time   `db:"finished_at" json:"finished_at,omitempty"`
}
