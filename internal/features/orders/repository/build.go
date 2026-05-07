package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/sqls/queries"
)

// SelectApprovedPlansForUpdate — читает approved plans с FOR UPDATE SKIP LOCKED.
//
// Должен вызываться внутри транзакции — иначе lock не имеет смысла.
// Передаём pgx.Tx через DBTX интерфейс ниже.
func (r *Repository) SelectApprovedPlansForUpdate(
	ctx context.Context, tx pgx.Tx, limit int,
) ([]models.ApprovedPlan, error) {
	rows, err := tx.Query(ctx, queries.MustGet("select_approved_plans_for_update"), limit)
	if err != nil {
		return nil, fmt.Errorf("orders: select approved plans: %w", err)
	}
	defer rows.Close()

	out := make([]models.ApprovedPlan, 0, limit)
	for rows.Next() {
		var p models.ApprovedPlan
		if scanErr := rows.Scan(
			&p.ID, &p.RunID, &p.SupplierID, &p.LocationID, &p.PlanDate,
			&p.TotalQty, &p.LinesCount,
		); scanErr != nil {
			return nil, fmt.Errorf("orders: scan approved plan: %w", scanErr)
		}
		out = append(out, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("orders: rows.Err approved plans: %w", rowsErr)
	}
	return out, nil
}

// SelectPlanLinesForBuild — читает calculation_lines одного plan-а.
func (r *Repository) SelectPlanLinesForBuild(
	ctx context.Context, runID uuid.UUID, supplierID, locationID string,
) ([]models.PlanLine, error) {
	rows, err := r.pool.Query(ctx,
		queries.MustGet("select_plan_lines_for_build"),
		runID, supplierID, locationID)
	if err != nil {
		return nil, fmt.Errorf("orders: select plan lines: %w", err)
	}
	defer rows.Close()

	out := make([]models.PlanLine, 0, 64) //nolint:mnd // pre-alloc
	for rows.Next() {
		var l models.PlanLine
		if scanErr := rows.Scan(&l.ProductID, &l.LocationID, &l.SupplierID, &l.ReorderQty); scanErr != nil {
			return nil, fmt.Errorf("orders: scan plan line: %w", scanErr)
		}
		out = append(out, l)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("orders: rows.Err plan lines: %w", rowsErr)
	}
	return out, nil
}

// InsertPurchaseOrderTx — INSERT PO внутри транзакции.
func (r *Repository) InsertPurchaseOrderTx(
	ctx context.Context, tx pgx.Tx, in InsertPOInput,
) (models.PurchaseOrder, error) {
	row := tx.QueryRow(ctx, queries.MustGet("insert_purchase_order"),
		in.PONumber, in.PlanID, in.SupplierID, in.LocationID,
		in.TotalQty, in.TotalAmount, in.Currency, in.DeliveryDate, in.Notes)
	var po models.PurchaseOrder
	err := row.Scan(
		&po.ID, &po.PONumber, &po.PlanID, &po.SupplierID, &po.LocationID,
		&po.Status, &po.TotalQty, &po.TotalAmount, &po.Currency,
		&po.DeliveryDate, &po.Notes, &po.SentAt, &po.SentToChannel,
		&po.CancelReason, &po.CreatedAt, &po.UpdatedAt,
	)
	if err != nil {
		return models.PurchaseOrder{}, fmt.Errorf("orders: insert PO: %w", err)
	}
	return po, nil
}

// InsertPOInput — параметры InsertPurchaseOrderTx.
type InsertPOInput struct {
	PONumber     string
	PlanID       uuid.UUID
	SupplierID   string
	LocationID   string
	TotalQty     float64
	TotalAmount  *float64
	Currency     string
	DeliveryDate *string // YYYY-MM-DD; nullable
	Notes        *string
}

// InsertPOLinesBulkTx — bulk INSERT po_lines через UNNEST.
func (r *Repository) InsertPOLinesBulkTx(
	ctx context.Context, tx pgx.Tx, poID uuid.UUID, lines []BulkLine,
) error {
	if len(lines) == 0 {
		return nil
	}
	productIDs := make([]string, len(lines))
	qtys := make([]float64, len(lines))
	prices := make([]*float64, len(lines))
	amounts := make([]*float64, len(lines))
	sources := make([]string, len(lines))
	for i, l := range lines {
		productIDs[i] = l.ProductID
		qtys[i] = l.Qty
		prices[i] = l.UnitPrice
		amounts[i] = l.LineAmount
		sources[i] = l.PricingSource
	}
	_, err := tx.Exec(ctx, queries.MustGet("insert_po_lines_bulk"),
		poID, productIDs, qtys, prices, amounts, sources)
	if err != nil {
		return fmt.Errorf("orders: bulk insert po_lines: %w", err)
	}
	return nil
}

// BulkLine — input для InsertPOLinesBulkTx.
type BulkLine struct {
	ProductID     string
	Qty           float64
	UnitPrice     *float64
	LineAmount    *float64
	PricingSource string
}

// InsertPOStatusHistoryTx — запись в audit log.
func (r *Repository) InsertPOStatusHistoryTx(
	ctx context.Context, tx pgx.Tx,
	poID uuid.UUID, fromStatus *string, toStatus string, reason, changedBy *string,
) error {
	_, err := tx.Exec(ctx, queries.MustGet("insert_po_status_history"),
		poID, fromStatus, toStatus, reason, changedBy)
	if err != nil {
		return fmt.Errorf("orders: insert status history: %w", err)
	}
	return nil
}

// MarkPlanConvertedTx — UPDATE plan.status='converted'.
// Возвращает ErrNoRows если plan уже не в approved (race).
func (r *Repository) MarkPlanConvertedTx(
	ctx context.Context, tx pgx.Tx, planID uuid.UUID,
) error {
	var id uuid.UUID
	err := tx.QueryRow(ctx, queries.MustGet("update_plan_to_converted"), planID).Scan(&id)
	if err != nil {
		return fmt.Errorf("orders: mark plan converted: %w", err)
	}
	return nil
}

// NextSequenceTx — nextval внутри транзакции (одна точка для атомарности).
func (r *Repository) NextSequenceTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	var seq int64
	err := tx.QueryRow(ctx, queries.MustGet("next_po_number_seq")).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("orders: nextval po_number_seq (tx): %w", err)
	}
	return seq, nil
}
