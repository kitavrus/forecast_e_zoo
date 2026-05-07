package models

import "time"

// AuditAccessEntry — строка marts.audit_access.
type AuditAccessEntry struct {
	ID         int64     `db:"id"`
	OccurredAt time.Time `db:"occurred_at"`
	Method     string    `db:"method"`
	Path       string    `db:"path"`
	Requester  *string   `db:"requester"`
	Role       *string   `db:"role"`
	StatusCode *int      `db:"status_code"`
	RequestID  *string   `db:"request_id"`
}
