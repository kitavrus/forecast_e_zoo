// Package repository — pgx-based persistence фичи kpi.
//
// Источники:
//   - kpi.kpi_snapshots / kpi.kpi_calibrations — write-side и read-side
//   - marts.mart_demand_history / marts.mart_calculation_input /
//     marts.mart_supplier_scorecard — read-only (через тот же pgxpool, ADR-002).
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
