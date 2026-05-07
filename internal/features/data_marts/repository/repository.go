// Package repository — pgx-based чтение mart-таблиц по go:embed SQL.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Repository — pgx + go:embed SQL.
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository поверх готового pool.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// VersionRow — agnostic версия mart'а из marts.etl_runs.
type VersionRow struct {
	EtlRunID    uuid.UUID
	CommittedAt time.Time
}

// MartVersionRow — версия конкретного mart'а в листинге.
type MartVersionRow struct {
	Name        string
	EtlRunID    *uuid.UUID
	CommittedAt *time.Time
}

// GetCurrentVersion возвращает последний committed etl_run_id.
// Если ни одного committed run нет → errorspkg.ErrServiceUnavailable.
func (r *Repository) GetCurrentVersion(ctx context.Context) (VersionRow, error) {
	var v VersionRow
	row := r.pool.QueryRow(ctx, queries.MustGet("current_version"))
	err := row.Scan(&v.EtlRunID, &v.CommittedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VersionRow{}, errorspkg.ErrServiceUnavailable.WithMessage("no committed etl run yet")
		}
		return VersionRow{}, fmt.Errorf("data_marts repo: get current version: %w", err)
	}
	return v, nil
}

// ListMartVersions возвращает per-mart версии (5 строк).
// Если ни одного committed run нет → возвращает 5 строк с nil-полями (mart known but not populated).
func (r *Repository) ListMartVersions(ctx context.Context) ([]MartVersionRow, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("list_marts_versions"))
	if err != nil {
		return nil, fmt.Errorf("data_marts repo: list marts: %w", err)
	}
	defer rows.Close()

	out := make([]MartVersionRow, 0, len(constants.MartNames))
	for rows.Next() {
		var (
			name string
			id   uuid.UUID
			ts   time.Time
		)
		if err := rows.Scan(&name, &id, &ts); err != nil {
			return nil, fmt.Errorf("data_marts repo: scan: %w", err)
		}
		idCopy := id
		tsCopy := ts
		out = append(out, MartVersionRow{Name: name, EtlRunID: &idCopy, CommittedAt: &tsCopy})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("data_marts repo: rows.Err: %w", err)
	}

	// Если committed run отсутствует → CTE last_committed пуст → 0 строк.
	// Возвращаем placeholder (5 mart'ов с nil-полями), чтобы handler знал имена.
	if len(out) == 0 {
		out = make([]MartVersionRow, 0, len(constants.MartNames))
		for _, name := range constants.MartNames {
			out = append(out, MartVersionRow{Name: name})
		}
	}
	return out, nil
}

// SelectMartRows — generic page reader для любого из 5 mart'ов.
// Сам выбирает SQL по name и parsing/binding cursor PK.
//
// Возвращает []models.MartRow (map[string]any) — handler сериализует в NDJSON.
// nextPK — конкатенированная строка PK последней строки в формате "<f1>|<f2>|<f3>"
// (или "" если строк нет / страница последняя).
//
//nolint:funlen,cyclop // switch по 5 mart'ам по природе длинный, без дублирующих helpers.
func (r *Repository) SelectMartRows(
	ctx context.Context,
	name string,
	etlRunID uuid.UUID,
	cursor models.Cursor,
	limit int,
) ([]models.MartRow, string, error) {
	if !constants.IsKnownMart(name) {
		return nil, "", errorspkg.ErrNotFound.WithMessage("mart not found: " + name)
	}
	if limit <= 0 {
		limit = constants.LimitDefault
	}
	if limit > constants.LimitMax {
		limit = constants.LimitMax
	}

	switch name {
	case constants.MartDemandHistory:
		return r.selectDemandHistory(ctx, etlRunID, cursor.LastPK, limit)
	case constants.MartCalculationInput:
		return r.selectCalculationInput(ctx, etlRunID, cursor.LastPK, limit)
	case constants.MartKpiDaily:
		return r.selectKpiDaily(ctx, etlRunID, cursor.LastPK, limit)
	case constants.MartMasterCurrent:
		return r.selectMasterCurrent(ctx, etlRunID, cursor.LastPK, limit)
	case constants.MartSupplierScorecard:
		return r.selectSupplierScorecard(ctx, etlRunID, cursor.LastPK, limit)
	default:
		// Не должно случиться — IsKnownMart выше уже проверил.
		return nil, "", errorspkg.ErrNotFound.WithMessage("mart not found: " + name)
	}
}
