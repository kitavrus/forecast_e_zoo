package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// InsertSnapshotInput — входные параметры InsertSnapshot.
type InsertSnapshotInput struct {
	AsOfDate      time.Time
	KpiName       string
	ScopeType     string
	ScopeID       *string
	Value         float64
	CalibrationID *uuid.UUID
	EtlRunID      *uuid.UUID
}

// InsertSnapshot вставляет новую строку kpi_snapshots и возвращает id+computed_at.
func (r *Repository) InsertSnapshot(ctx context.Context, in InsertSnapshotInput) (models.KpiSnapshot, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("insert_snapshot"),
		in.AsOfDate, in.KpiName, in.ScopeType, in.ScopeID,
		in.Value, in.CalibrationID, in.EtlRunID,
	)
	var s models.KpiSnapshot
	if err := row.Scan(&s.ID, &s.AsOfDate, &s.ComputedAt, &s.CreatedAt); err != nil {
		return models.KpiSnapshot{}, fmt.Errorf("kpi: insert snapshot: %w", err)
	}
	s.KpiName = in.KpiName
	s.ScopeType = in.ScopeType
	s.ScopeID = in.ScopeID
	s.Value = in.Value
	s.CalibrationID = in.CalibrationID
	s.EtlRunID = in.EtlRunID
	return s, nil
}

// ListSnapshots возвращает страницу snapshots по filter.
//
// nextCursor — ID последней строки (используется в следующем запросе через
// SnapshotFilter.Cursor). Пустая строка → последняя страница.
func (r *Repository) ListSnapshots(
	ctx context.Context,
	f models.SnapshotFilter,
) ([]models.KpiSnapshot, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	var (
		asOf     interface{} = nil
		kpiName  interface{} = nil
		stype    interface{} = nil
		sid      interface{} = nil
		cursorID interface{} = nil
	)
	if f.AsOfDate != nil {
		asOf = *f.AsOfDate
	}
	if f.KpiName != nil {
		kpiName = *f.KpiName
	}
	if f.ScopeType != nil {
		stype = *f.ScopeType
	}
	if f.ScopeID != nil {
		sid = *f.ScopeID
	}
	if f.Cursor != "" {
		id, err := uuid.Parse(f.Cursor)
		if err != nil {
			return nil, "", errorspkg.ErrBadRequest.WithMessage("invalid cursor")
		}
		cursorID = id
	}

	rows, err := r.pool.Query(ctx, queries.MustGet("select_snapshots"),
		asOf, kpiName, stype, sid, cursorID, limit,
	)
	if err != nil {
		return nil, "", fmt.Errorf("kpi: list snapshots query: %w", err)
	}
	defer rows.Close()

	out := make([]models.KpiSnapshot, 0, limit)
	for rows.Next() {
		var s models.KpiSnapshot
		if scanErr := rows.Scan(
			&s.ID, &s.AsOfDate, &s.KpiName, &s.ScopeType, &s.ScopeID,
			&s.Value, &s.CalibrationID, &s.ComputedAt, &s.EtlRunID,
		); scanErr != nil {
			return nil, "", fmt.Errorf("kpi: list snapshots scan: %w", scanErr)
		}
		out = append(out, s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, "", fmt.Errorf("kpi: list snapshots rows.Err: %w", rowsErr)
	}

	nextCursor := ""
	if len(out) == limit {
		nextCursor = out[len(out)-1].ID.String()
	}
	return out, nextCursor, nil
}

// GetSnapshotByID возвращает snapshot по UUID или ErrKpiSnapshotNotFound.
func (r *Repository) GetSnapshotByID(ctx context.Context, id uuid.UUID) (models.KpiSnapshot, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_snapshot_by_id"), id)
	var s models.KpiSnapshot
	err := row.Scan(
		&s.ID, &s.AsOfDate, &s.KpiName, &s.ScopeType, &s.ScopeID,
		&s.Value, &s.CalibrationID, &s.ComputedAt, &s.EtlRunID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.KpiSnapshot{}, errorspkg.ErrKpiSnapshotNotFound
		}
		return models.KpiSnapshot{}, fmt.Errorf("kpi: get snapshot by id: %w", err)
	}
	return s, nil
}

// DeleteSnapshotsForDate удаляет все snapshots за дату, опционально для KPI.
//
// kpiNames=nil → удалить все KPI.
func (r *Repository) DeleteSnapshotsForDate(
	ctx context.Context, date time.Time, kpiNames []string,
) (int64, error) {
	var arg interface{} = nil
	if len(kpiNames) > 0 {
		arg = kpiNames
	}
	tag, err := r.pool.Exec(ctx, queries.MustGet("delete_snapshots_for_refresh"), date, arg)
	if err != nil {
		return 0, fmt.Errorf("kpi: delete snapshots: %w", err)
	}
	return tag.RowsAffected(), nil
}
