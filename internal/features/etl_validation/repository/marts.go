package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/sqls/queries"
)

// MartUpsert описывает контракт upsert-операции в одну mart-таблицу.
//
// На фазе 06 — заглушки, которые ожидают, что соответствующий .sql
// существует в queries (заполняется в фазе 07).
//
// Каждый метод исполняется в рамках транзакции tx (атомарный flip).

// UpsertDemandHistory выполняет INSERT…SELECT в marts.mart_demand_history
// из staging-таблицы pg_temp.stg_receipt_line.
func (r *Repository) UpsertDemandHistory(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	return r.execMartQuery(ctx, tx, "mart_demand_history_upsert", runID, sourceLoadID)
}

// RebuildCalculationInput пересоздаёт mart_calculation_input.
func (r *Repository) RebuildCalculationInput(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	return r.execMartQuery(ctx, tx, "mart_calculation_input_truncate_insert", runID, sourceLoadID)
}

// UpsertKpiDaily вставляет/обновляет mart_kpi_daily.
func (r *Repository) UpsertKpiDaily(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	return r.execMartQuery(ctx, tx, "mart_kpi_daily_insert", runID, sourceLoadID)
}

// RebuildMasterCurrent пересоздаёт mart_master_current (TRUNCATE + INSERT).
func (r *Repository) RebuildMasterCurrent(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	return r.execMartQuery(ctx, tx, "mart_master_current_truncate_insert", runID, sourceLoadID)
}

// UpsertSupplierScorecard агрегирует mart_supplier_scorecard.
func (r *Repository) UpsertSupplierScorecard(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	return r.execMartQuery(ctx, tx, "mart_supplier_scorecard_insert", runID, sourceLoadID)
}

// execMartQuery — общий путь: SQL ожидает $1=run_id, $2=source_load_id;
// возвращает RowsAffected.
func (r *Repository) execMartQuery(ctx context.Context, tx pgx.Tx, name string, runID, sourceLoadID uuid.UUID) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("repository: %s: tx is required", name)
	}
	tag, err := tx.Exec(ctx, queries.MustGet(name), runID, sourceLoadID)
	if err != nil {
		return 0, fmt.Errorf("repository: %s: %w", name, err)
	}
	return tag.RowsAffected(), nil
}
