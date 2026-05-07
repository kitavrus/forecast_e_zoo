package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CreateStagingTables создаёт TEMP TABLE pg_temp.stg_* в рамках tx.
//
// На фазе 06 — заглушка-заголовок: реальные DDL заполняются в фазе 07
// через staging_create_temp_tables.sql.
func (r *Repository) CreateStagingTables(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return fmt.Errorf("repository: CreateStagingTables: tx is required")
	}
	// SQL заполняется в Phase 07; на текущий момент embedded-файл может отсутствовать —
	// этот метод вызывается только из transformer/loader (Phase 11/12), и к тому времени
	// SQL уже будет в queries/.
	return nil
}

// BulkInsertStaging — обёртка над pgx.CopyFrom для массовой загрузки в pg_temp.stg_*.
func (r *Repository) BulkInsertStaging(
	ctx context.Context,
	tx pgx.Tx,
	tableName string,
	columns []string,
	rowSrc pgx.CopyFromSource,
) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("repository: BulkInsertStaging: tx is required")
	}
	n, err := tx.CopyFrom(ctx, pgx.Identifier{tableName}, columns, rowSrc)
	if err != nil {
		return 0, fmt.Errorf("repository: BulkInsertStaging %s: %w", tableName, err)
	}
	return n, nil
}
