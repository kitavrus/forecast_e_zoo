package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetPOByID — single PO.
func (r *Repository) GetPOByID(
	ctx context.Context, id uuid.UUID,
) (models.PurchaseOrder, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_po_by_id"), id)
	po := models.PurchaseOrder{}
	err := row.Scan(
		&po.ID, &po.PONumber, &po.PlanID, &po.SupplierID, &po.LocationID,
		&po.Status, &po.TotalQty, &po.TotalAmount, &po.Currency,
		&po.DeliveryDate, &po.Notes, &po.SentAt, &po.SentToChannel,
		&po.CancelReason, &po.CreatedAt, &po.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.PurchaseOrder{}, errorspkg.ErrPurchaseOrderNotFound
		}
		return models.PurchaseOrder{}, fmt.Errorf("orders: get PO: %w", err)
	}
	return po, nil
}

// GetPOLines — все lines одного PO.
func (r *Repository) GetPOLines(
	ctx context.Context, poID uuid.UUID,
) ([]models.POLine, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("select_po_lines"), poID)
	if err != nil {
		return nil, fmt.Errorf("orders: get PO lines: %w", err)
	}
	defer rows.Close()
	out := make([]models.POLine, 0, 64) //nolint:mnd
	for rows.Next() {
		l := models.POLine{POID: poID}
		if scanErr := rows.Scan(
			&l.ID, &l.ProductID, &l.Qty, &l.UnitPrice, &l.LineAmount,
			&l.PricingSource, &l.Notes, &l.CreatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("orders: scan PO line: %w", scanErr)
		}
		out = append(out, l)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("orders: rows.Err PO lines: %w", rowsErr)
	}
	return out, nil
}

// GetPOHistory — все status_history записи одного PO.
func (r *Repository) GetPOHistory(
	ctx context.Context, poID uuid.UUID,
) ([]models.POStatusHistory, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("select_po_history"), poID)
	if err != nil {
		return nil, fmt.Errorf("orders: get PO history: %w", err)
	}
	defer rows.Close()
	out := make([]models.POStatusHistory, 0, 8) //nolint:mnd
	for rows.Next() {
		h := models.POStatusHistory{POID: poID}
		if scanErr := rows.Scan(
			&h.ID, &h.FromStatus, &h.ToStatus, &h.Reason, &h.ChangedBy, &h.ChangedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("orders: scan PO history: %w", scanErr)
		}
		out = append(out, h)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("orders: rows.Err PO history: %w", rowsErr)
	}
	return out, nil
}

// ListPOs — пагинация PO с фильтрами.
//
//nolint:cyclop // мульти-фильтр.
func (r *Repository) ListPOs(
	ctx context.Context, f models.POFilter,
) ([]models.PurchaseOrder, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = constants.LimitDefault
	}
	if limit > constants.LimitMax {
		limit = constants.LimitMax
	}

	var (
		st       interface{} = nil
		sup      interface{} = nil
		plan     interface{} = nil
		from     interface{} = nil
		to       interface{} = nil
		cursorID interface{} = nil
	)
	if f.Status != nil {
		st = *f.Status
	}
	if f.SupplierID != nil {
		sup = *f.SupplierID
	}
	if f.PlanID != nil {
		plan = *f.PlanID
	}
	if f.From != nil {
		from = *f.From
	}
	if f.To != nil {
		to = *f.To
	}
	if f.Cursor != "" {
		id, err := uuid.Parse(f.Cursor)
		if err != nil {
			return nil, "", errorspkg.ErrBadRequest.WithMessage("invalid cursor")
		}
		cursorID = id
	}

	rows, err := r.pool.Query(ctx, queries.MustGet("select_purchase_orders"),
		st, sup, plan, from, to, cursorID, limit)
	if err != nil {
		return nil, "", fmt.Errorf("orders: list PO query: %w", err)
	}
	defer rows.Close()

	out := make([]models.PurchaseOrder, 0, limit)
	for rows.Next() {
		po := models.PurchaseOrder{}
		if scanErr := rows.Scan(
			&po.ID, &po.PONumber, &po.PlanID, &po.SupplierID, &po.LocationID,
			&po.Status, &po.TotalQty, &po.TotalAmount, &po.Currency,
			&po.DeliveryDate, &po.Notes, &po.SentAt, &po.SentToChannel,
			&po.CancelReason, &po.CreatedAt, &po.UpdatedAt,
		); scanErr != nil {
			return nil, "", fmt.Errorf("orders: scan PO: %w", scanErr)
		}
		out = append(out, po)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, "", fmt.Errorf("orders: list PO rows.Err: %w", rowsErr)
	}

	nextCursor := ""
	if len(out) == limit {
		nextCursor = out[len(out)-1].ID.String()
	}
	return out, nextCursor, nil
}

// CancelPO — переводит PO в cancelled. Если статус некорректный — pgx.ErrNoRows.
//
// Service слой различает not found / not cancellable через дополнительный SELECT.
func (r *Repository) CancelPO(
	ctx context.Context, id uuid.UUID, reason string,
) (models.PurchaseOrder, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("update_po_cancel"), id, reason)
	po := models.PurchaseOrder{}
	err := row.Scan(
		&po.ID, &po.PONumber, &po.PlanID, &po.SupplierID, &po.LocationID,
		&po.Status, &po.TotalQty, &po.TotalAmount, &po.Currency,
		&po.DeliveryDate, &po.Notes, &po.SentAt, &po.SentToChannel,
		&po.CancelReason, &po.CreatedAt, &po.UpdatedAt,
	)
	if err == nil {
		return po, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return models.PurchaseOrder{}, fmt.Errorf("orders: cancel PO: %w", err)
	}
	// 0 rows: либо PO нет, либо статус не в allowlist.
	existing, getErr := r.GetPOByID(ctx, id)
	if getErr != nil {
		return models.PurchaseOrder{}, getErr
	}
	if !constants.IsCancellableStatus(existing.Status) {
		return models.PurchaseOrder{}, errorspkg.ErrPONotCancellable
	}
	// Race: статус был cancellable, кто-то изменил. Возвращаем актуальное.
	return existing, nil
}

// CancelPOTx — версия для transaction (используется regenerate).
func (r *Repository) CancelPOTx(
	ctx context.Context, tx pgx.Tx, id uuid.UUID, reason string,
) (models.PurchaseOrder, error) {
	row := tx.QueryRow(ctx, queries.MustGet("update_po_cancel"), id, reason)
	po := models.PurchaseOrder{}
	err := row.Scan(
		&po.ID, &po.PONumber, &po.PlanID, &po.SupplierID, &po.LocationID,
		&po.Status, &po.TotalQty, &po.TotalAmount, &po.Currency,
		&po.DeliveryDate, &po.Notes, &po.SentAt, &po.SentToChannel,
		&po.CancelReason, &po.CreatedAt, &po.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.PurchaseOrder{}, errorspkg.ErrPONotCancellable
		}
		return models.PurchaseOrder{}, fmt.Errorf("orders: cancel PO (tx): %w", err)
	}
	return po, nil
}
