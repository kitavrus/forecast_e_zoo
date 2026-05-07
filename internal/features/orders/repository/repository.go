// Package repository — pgx-based persistence фичи orders (Module 6).
//
// Источники:
//   - orders.purchase_orders / po_lines / po_status_history — write+read
//   - forecast.replenishment_plans / calculation_lines — read + UPDATE plan.status
//   - marts.mart_master_current — read-only (supplier/product master)
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
