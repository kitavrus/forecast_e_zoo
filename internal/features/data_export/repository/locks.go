package repository

import (
	"context"
	"hash/fnv"

	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// LockKey вычисляет stable bigint-ключ для pg_try_advisory_lock из строкового тега.
// Используем FNV-64 (детерминированный, без зависимостей).
func LockKey(tag string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(tag))
	// Конвертируем в int64 (могут быть отрицательные — postgres воспринимает как bigint).
	return int64(h.Sum64()) //nolint:gosec // ok for advisory lock key (PG accepts full int64 range)
}

// TryAdvisoryLock — non-blocking. Возвращает true, если лок взят.
// Если tx != nil, лок берётся transaction-scoped (snapshot не виден другим tx).
// Если tx == nil — session-scoped (на pool connection).
func (r *Repository) TryAdvisoryLock(ctx context.Context, tx pgx.Tx, key int64) (bool, error) {
	exec := r.chooseExec(tx)
	row := exec.QueryRow(ctx, queries.Get("advisory_lock_try"), key)
	var locked bool
	if err := row.Scan(&locked); err != nil {
		return false, mapError(err)
	}
	return locked, nil
}

// AdvisoryUnlock освобождает session-scoped лок. Для tx-scoped — освобождается на commit/rollback.
func (r *Repository) AdvisoryUnlock(ctx context.Context, tx pgx.Tx, key int64) error {
	exec := r.chooseExec(tx)
	_, err := exec.Exec(ctx, queries.Get("advisory_unlock"), key)
	return mapError(err)
}
