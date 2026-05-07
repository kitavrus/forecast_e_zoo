package models

import (
	"time"

	"github.com/google/uuid"
)

// RejectLogEntry — строка marts.reject_log.
type RejectLogEntry struct {
	ID          int64     `db:"id"`
	EtlRunID    uuid.UUID `db:"etl_run_id"`
	Entity      string    `db:"entity"`
	BusinessKey *string   `db:"business_key"`
	Severity    string    `db:"severity"` // critical|soft
	RuleID      string    `db:"rule_id"`
	Field       *string   `db:"field"`
	Message     string    `db:"message"`
	CreatedAt   time.Time `db:"created_at"`
}

// RejectFilter — фильтр для пагинированного списка reject_log.
type RejectFilter struct {
	EtlRunID *uuid.UUID
	Entity   string
	Severity string
	BeforeID *int64 // cursor по id DESC
	Limit    int
}
