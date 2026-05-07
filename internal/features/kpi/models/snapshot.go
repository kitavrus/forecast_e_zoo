// Package models — domain types фичи kpi.
package models

import (
	"time"

	"github.com/google/uuid"
)

// KpiSnapshot — одна строка таблицы kpi.kpi_snapshots.
type KpiSnapshot struct {
	ID            uuid.UUID
	AsOfDate      time.Time
	KpiName       string
	ScopeType     string
	ScopeID       *string
	Value         float64
	CalibrationID *uuid.UUID
	ComputedAt    time.Time
	EtlRunID      *uuid.UUID
	CreatedAt     time.Time
}

// SnapshotFilter — фильтр для list-запроса.
//
// Все поля optional. Пагинация через cursor (id::text последней строки).
type SnapshotFilter struct {
	AsOfDate  *time.Time
	KpiName   *string
	ScopeType *string
	ScopeID   *string
	Limit     int
	Cursor    string
}
