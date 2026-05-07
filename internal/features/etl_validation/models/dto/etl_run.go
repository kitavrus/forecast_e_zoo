// Package dto содержит Request/Response типы handlers фичи etl_validation.
package dto

import (
	"encoding/json"
	"time"
)

// PostEtlRunRequest — POST /admin/etl-runs.
//
// Тело запроса не используется — force-start без параметров.
type PostEtlRunRequest struct{}

// RetryEtlRunRequest — POST /admin/etl-runs/{id}/retry.
//
// Тело запроса не используется — id берётся из path.
type RetryEtlRunRequest struct{}

// MartRefreshRequest — POST /admin/marts/{name}/refresh.
//
// Тело запроса не используется — имя mart берётся из path.
type MartRefreshRequest struct{}

// EtlRunResponse — представление marts.etl_runs для admin endpoints.
type EtlRunResponse struct {
	ID            string          `json:"id"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty"`
	CommittedAt   *time.Time      `json:"committed_at,omitempty"`
	Status        string          `json:"status" enums:"running,committed,failed,aborted"`
	Kind          string          `json:"kind"   enums:"full,mart_refresh"`
	TargetMart    *string         `json:"target_mart,omitempty"`
	SourceLoadID  *string         `json:"source_load_id,omitempty"`
	ParentRunID   *string         `json:"parent_run_id,omitempty"`
	Trigger       string          `json:"trigger" enums:"cron,admin,retry"`
	Requester     *string         `json:"requester,omitempty"`
	MartsSummary  json.RawMessage `json:"marts_summary,omitempty" swaggertype:"object"`
	FailureReason *string         `json:"failure_reason,omitempty"`
	LinesTotal    int64           `json:"lines_total"`
	LinesFailed   int64           `json:"lines_failed"`
}

// EtlRunListResponse — пагинированный список etl_runs.
type EtlRunListResponse struct {
	Items      []EtlRunResponse `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

// MartRefreshResponse — POST /admin/marts/{name}/refresh.
type MartRefreshResponse struct {
	RunID      string `json:"run_id"`
	Status     string `json:"status"      enums:"running,committed,failed,aborted"`
	TargetMart string `json:"target_mart"`
}
