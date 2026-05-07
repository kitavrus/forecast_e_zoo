package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// RejectInput — payload для Insert; отдельный input-struct без id/created_at,
// которые проставляет БД.
type RejectInput struct {
	LoadID   uuid.UUID
	Entity   string
	Payload  []byte // jsonb
	Errors   []byte // jsonb (массив объектов {field,rule,message})
	Severity string // 'error' | 'warn' (см. CHECK migration 0001)
}

// RejectFilter — фильтры для Select.
type RejectFilter struct {
	LoadID   uuid.UUID // uuid.Nil = no filter
	Entity   string    // ""      = no filter
	Severity string    // ""      = no filter
}

// InsertReject записывает одну строку в reject_log.
func (r *Repository) InsertReject(ctx context.Context, in RejectInput) error {
	if in.Payload == nil {
		in.Payload = []byte("{}")
	}
	if in.Errors == nil {
		in.Errors = []byte("[]")
	}
	_, err := r.pool.Exec(ctx, queries.Get("reject_log_insert"),
		in.LoadID, in.Entity, in.Payload, in.Errors, in.Severity)
	return mapError(err)
}

// RejectRow — выходная row для Select.
type RejectRow struct {
	ID       int64
	LoadID   uuid.UUID
	Entity   string
	Payload  []byte
	Errors   []byte
	Severity string
}

// SelectRejects возвращает страницу строк reject_log. afterPK — id последней
// строки (как string, "" для начала).
func (r *Repository) SelectRejects(ctx context.Context, f RejectFilter, afterPK string, limit int) ([]RejectRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("reject_log_select"),
		f.LoadID, f.Entity, f.Severity, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]RejectRow, 0, limit)
	for rows.Next() {
		var (
			row  RejectRow
			_ts  any
		)
		if err := rows.Scan(&row.ID, &row.LoadID, &row.Entity,
			&row.Payload, &row.Errors, &row.Severity, &_ts); err != nil {
			return nil, mapError(err)
		}
		out = append(out, row)
	}
	return out, mapError(rows.Err())
}

// _ helper to silence unused models import
var _ = models.SeverityCritical
