package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/sqls/queries"
)

// CreateStagingTables создаёт TEMP TABLE pg_temp.stg_* в рамках tx.
//
// SQL — staging_create_temp_tables.sql (CREATE TEMP TABLE ... ON COMMIT DROP).
// Транзакция обязательна: TEMP TABLE привязана к session/tx; ON COMMIT DROP
// гарантирует, что таблицы исчезнут вместе с tx.
func (r *Repository) CreateStagingTables(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return fmt.Errorf("repository: CreateStagingTables: tx is required")
	}
	if _, err := tx.Exec(ctx, queries.MustGet("staging_create_temp_tables")); err != nil {
		return fmt.Errorf("repository: CreateStagingTables: %w", err)
	}
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
