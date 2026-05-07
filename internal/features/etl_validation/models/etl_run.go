// Package models содержит domain-структуры feature etl_validation.
// Поля 1:1 соответствуют колонкам таблиц marts.* (фазы 03/04).
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EtlRun — строка marts.etl_runs.
type EtlRun struct {
	ID            uuid.UUID       `db:"id"`
	StartedAt     time.Time       `db:"started_at"`
	FinishedAt    *time.Time      `db:"finished_at"`
	CommittedAt   *time.Time      `db:"committed_at"`
	Status        string          `db:"status"`  // running|committed|failed|aborted
	Kind          string          `db:"kind"`    // full|mart_refresh
	TargetMart    *string         `db:"target_mart"`
	SourceLoadID  *uuid.UUID      `db:"source_load_id"`
	ParentRunID   *uuid.UUID      `db:"parent_run_id"`
	Trigger       string          `db:"trigger"` // cron|admin|retry
	Requester     *string         `db:"requester"`
	MartsSummary  json.RawMessage `db:"marts_summary"`
	FailureReason *string         `db:"failure_reason"`
	LinesTotal    int64           `db:"lines_total"`
	LinesFailed   int64           `db:"lines_failed"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

// EtlRunFilter — фильтр для пагинированного списка.
type EtlRunFilter struct {
	Status     string
	Kind       string
	BeforeTime *time.Time // курсор: started_at < BeforeTime
	Limit      int
}

// MartsSummary — структура поля marts_summary.
//
// Хранится в БД как JSONB. Каждый ключ — имя mart, значение — счётчики.
type MartsSummary map[string]MartSummaryEntry

// MartSummaryEntry — детали по конкретному mart.
type MartSummaryEntry struct {
	Rows         int64 `json:"rows"`
	RowsRejected int64 `json:"rows_rejected,omitempty"`
}
