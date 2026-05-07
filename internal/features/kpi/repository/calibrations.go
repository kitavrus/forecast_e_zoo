package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ListCalibrations возвращает все калибровки + фильтры.
func (r *Repository) ListCalibrations(
	ctx context.Context, f models.CalibrationFilter,
) ([]models.KpiCalibration, error) {
	var (
		kpiName interface{} = nil
		stype   interface{} = nil
	)
	if f.KpiName != nil {
		kpiName = *f.KpiName
	}
	if f.ScopeType != nil {
		stype = *f.ScopeType
	}

	rows, err := r.pool.Query(ctx, queries.MustGet("select_calibrations"), kpiName, stype)
	if err != nil {
		return nil, fmt.Errorf("kpi: list calibrations query: %w", err)
	}
	defer rows.Close()

	out := make([]models.KpiCalibration, 0, 16) //nolint:mnd // pre-alloc реальный потолок
	for rows.Next() {
		var c models.KpiCalibration
		var raw []byte
		if scanErr := rows.Scan(
			&c.ID, &c.KpiName, &c.ScopeType, &c.ScopeID, &raw, &c.CreatedAt, &c.UpdatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("kpi: list calibrations scan: %w", scanErr)
		}
		c.Params = json.RawMessage(raw)
		out = append(out, c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("kpi: list calibrations rows.Err: %w", rowsErr)
	}
	return out, nil
}

// GetCalibrationByID возвращает калибровку по id или ErrKpiCalibrationNotFound.
func (r *Repository) GetCalibrationByID(ctx context.Context, id uuid.UUID) (models.KpiCalibration, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_calibration_by_id"), id)
	var c models.KpiCalibration
	var raw []byte
	err := row.Scan(&c.ID, &c.KpiName, &c.ScopeType, &c.ScopeID, &raw, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.KpiCalibration{}, errorspkg.ErrKpiCalibrationNotFound
		}
		return models.KpiCalibration{}, fmt.Errorf("kpi: get calibration: %w", err)
	}
	c.Params = json.RawMessage(raw)
	return c, nil
}

// UpdateCalibration обновляет params и возвращает обновлённую запись.
func (r *Repository) UpdateCalibration(
	ctx context.Context, id uuid.UUID, params json.RawMessage,
) (models.KpiCalibration, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("update_calibration"), id, []byte(params))
	var c models.KpiCalibration
	var raw []byte
	err := row.Scan(&c.ID, &c.KpiName, &c.ScopeType, &c.ScopeID, &raw, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.KpiCalibration{}, errorspkg.ErrKpiCalibrationNotFound
		}
		return models.KpiCalibration{}, fmt.Errorf("kpi: update calibration: %w", err)
	}
	c.Params = json.RawMessage(raw)
	return c, nil
}
