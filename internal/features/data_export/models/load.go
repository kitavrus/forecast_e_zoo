package models

import (
	"time"

	"github.com/google/uuid"
)

// LoadStatus — статус загрузки (см. table loads).
type LoadStatus string

const (
	LoadStatusRunning   LoadStatus = "running"
	LoadStatusCommitted LoadStatus = "committed"
	LoadStatusFailed    LoadStatus = "failed"
	LoadStatusAborted   LoadStatus = "aborted"
)

// Load — метаданные одной загрузки (job).
type Load struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	StartedAt       time.Time  `db:"started_at" json:"started_at"`
	FinishedAt      *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	Status          LoadStatus `db:"status" json:"status"`
	Source          string     `db:"source" json:"source"`
	EntitiesSummary []byte     `db:"entities_summary" json:"entities_summary,omitempty"`
	FailureReason   *string    `db:"failure_reason" json:"failure_reason,omitempty"`
	ParentLoadID    *uuid.UUID `db:"parent_load_id" json:"parent_load_id,omitempty"`
}

// EntityStat — счётчики одной сущности в рамках Load (для entities_summary JSONB).
type EntityStat struct {
	Entity   string `json:"entity"`
	Total    int64  `json:"total"`
	Inserted int64  `json:"inserted"`
	Updated  int64  `json:"updated"`
	Failed   int64  `json:"failed"`
}
