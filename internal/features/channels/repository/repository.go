// Package repository — pgx-based persistence фичи channels (Module 7).
//
// Источники:
//   - channels.supplier_channel_config / send_attempts — write+read
//   - orders.purchase_orders — read (FOR UPDATE SKIP LOCKED) + UPDATE status='sent'
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
