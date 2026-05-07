package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TryAdvisoryXactLock пытается захватить advisory-lock в рамках транзакции tx.
// Возвращает true, если lock получен. tx ОБЯЗАТЕЛЕН (xact-уровневый lock).
//
// Используется scheduler-ом для идемпотентного запуска ETL run и admin
// handler-ом — чтобы не запускать второй параллельный run.
func (r *Repository) TryAdvisoryXactLock(ctx context.Context, tx pgx.Tx, key int64) (bool, error) {
	if tx == nil {
		return false, fmt.Errorf("repository: TryAdvisoryXactLock: tx is required")
	}
	var ok bool
	err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", key).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("repository: TryAdvisoryXactLock: %w", err)
	}
	return ok, nil
}
