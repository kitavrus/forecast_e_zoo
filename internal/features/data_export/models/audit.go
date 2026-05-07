package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditAccessEntry — запись аудита доступа (только для /admin/* эндпоинтов).
type AuditAccessEntry struct {
	ID         uuid.UUID `db:"id" json:"id"`
	Requester  string    `db:"requester" json:"requester"`
	Endpoint   string    `db:"endpoint" json:"endpoint"`
	Method     string    `db:"method" json:"method"`
	Query      []byte    `db:"query" json:"query"`
	BytesOut   int64     `db:"bytes_out" json:"bytes_out"`
	StatusCode int       `db:"status_code" json:"status_code"`
	Ts         time.Time `db:"ts" json:"ts"`
}
