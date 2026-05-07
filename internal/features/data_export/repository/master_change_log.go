package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// ChangeLogRow — выборочная row для select_master_change_log.
type ChangeLogRow struct {
	ID        int64
	Entity    string
	EntityPK  []byte
	Field     string
	OldValue  []byte
	NewValue  []byte
	ChangedAt time.Time
	LoadID    *uuid.UUID
}

// InsertChangeLog добавляет одну запись в master_change_log в рамках tx.
func (r *Repository) InsertChangeLog(ctx context.Context, tx pgx.Tx, row ChangeLogRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("InsertChangeLog requires tx")
	}
	const sql = `
INSERT INTO master_change_log (entity, entity_pk, field, old_value, new_value, changed_at, load_id)
VALUES ($1, $2::jsonb, $3, $4::jsonb, $5::jsonb, COALESCE($6, now()), $7)
`
	_, err := tx.Exec(ctx, sql, row.Entity, row.EntityPK, row.Field,
		row.OldValue, row.NewValue, nullableTime(row.ChangedAt), loadID)
	return mapError(err)
}

// SelectChangeLog возвращает page строк с курсором по id.
func (r *Repository) SelectChangeLog(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]ChangeLogRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	if afterPK == "" {
		afterPK = "0"
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_master_change_log"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]ChangeLogRow, 0, limit)
	for rows.Next() {
		var c ChangeLogRow
		if err := rows.Scan(&c.ID, &c.Entity, &c.EntityPK, &c.Field,
			&c.OldValue, &c.NewValue, &c.ChangedAt, &c.LoadID); err != nil {
			return nil, mapError(err)
		}
		out = append(out, c)
	}
	return out, mapError(rows.Err())
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
