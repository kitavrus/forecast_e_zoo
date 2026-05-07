// Package repository — pgx-based persistence фичи forecast (Module 5).
//
// Источники:
//   - forecast.forecast_runs / forecasts / calculation_lines / replenishment_plans — write+read
//   - marts.mart_calculation_input / mart_demand_history / mart_supplier_scorecard / etl_runs — read-only
package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository — pgx + go:embed SQL.
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository поверх готового pool.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Pool возвращает pgxpool — нужен сценарию scheduler (advisory lock).
func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}
