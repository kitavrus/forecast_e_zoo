package dto

import "time"

// RejectLogEntryResponse — строка marts.reject_log в формате REST-ответа.
type RejectLogEntryResponse struct {
	ID          int64     `json:"id"`
	EtlRunID    string    `json:"etl_run_id"`
	Entity      string    `json:"entity"`
	BusinessKey *string   `json:"business_key,omitempty"`
	Severity    string    `json:"severity" enums:"critical,soft"`
	RuleID      string    `json:"rule_id"`
	Field       *string   `json:"field,omitempty"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"created_at"`
}

// RejectLogListResponse — пагинированный список reject_log.
type RejectLogListResponse struct {
	Items      []RejectLogEntryResponse `json:"items"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}
