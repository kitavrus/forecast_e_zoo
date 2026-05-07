package models

import (
	"time"

	"github.com/google/uuid"
)

// Severity — severity-уровень валидации.
type Severity string

const (
	// SeverityCritical — fail load, ничего не коммитим.
	SeverityCritical Severity = "critical"
	// SeveritySoft — пишем в reject_log, но load продолжается.
	SeveritySoft Severity = "soft"
)

// RejectEntry — запись о rejected/soft-fail записи.
type RejectEntry struct {
	ID         uuid.UUID `db:"id" json:"id"`
	LoadID     uuid.UUID `db:"load_id" json:"load_id"`
	Entity     string    `db:"entity" json:"entity"`
	PKValue    []byte    `db:"pk_value" json:"pk_value"`
	Severity   Severity  `db:"severity" json:"severity"`
	Reason     string    `db:"reason" json:"reason"`
	Raw        []byte    `db:"raw" json:"raw"`
	DetectedAt time.Time `db:"detected_at" json:"detected_at"`
}

// RejectSummary — агрегированная статистика отказов по сущности+severity.
type RejectSummary struct {
	Entity   string   `db:"entity" json:"entity"`
	Severity Severity `db:"severity" json:"severity"`
	Count    int64    `db:"count" json:"count"`
}
