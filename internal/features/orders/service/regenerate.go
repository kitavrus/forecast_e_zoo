package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/orders/builder"
	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/numbering"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Regenerate — создаёт новый PO из того же plan, старый помечает cancelled.
//
// Обязательные условия:
//   - existing PO в статусе draft|ready_to_send (regeneratable);
//   - plan существует (используем supplier_id/location_id из existing PO).
//
// Алгоритм (одна транзакция):
//   1. SELECT existing PO by id.
//   2. Проверка статуса (regeneratable).
//   3. CancelPOTx → старый PO в cancelled.
//   4. Insert audit history old→cancelled.
//   5. Reload lines + masters → builder.Build с новым PO number.
//   6. Insert new PO + new lines + history nil→ready_to_send.
//
//nolint:funlen,cyclop // pipeline orchestration
func (s *Service) Regenerate(
	ctx context.Context, in models.RegenerateInput,
) (models.RegenerateResult, error) {
	if s.pool == nil {
		return models.RegenerateResult{}, errorspkg.ErrInternal.WithMessage("pool not configured")
	}
	current, err := s.repo.GetPOByID(ctx, in.POID)
	if err != nil {
		return models.RegenerateResult{}, err //nolint:wrapcheck
	}
	if !constants.IsRegeneratableStatus(current.Status) {
		return models.RegenerateResult{}, errorspkg.ErrPOAlreadySent
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.RegenerateResult{}, fmt.Errorf("orders regenerate: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1) Cancel old PO.
	cancelled, err := s.repo.CancelPOTx(ctx, tx, in.POID, in.Reason)
	if err != nil {
		return models.RegenerateResult{}, err //nolint:wrapcheck
	}
	from := current.Status
	cancelReason := in.Reason
	if cancelReason == "" {
		cancelReason = "regenerate"
	}
	by := in.ChangedBy
	if err := s.repo.InsertPOStatusHistoryTx(ctx, tx, cancelled.ID,
		&from, cancelled.Status, &cancelReason, &by); err != nil {
		return models.RegenerateResult{}, fmt.Errorf("orders regenerate: history old: %w", err)
	}

	// 2) Re-build new PO from same plan.
	newPO, newNumber, err := s.rebuildFromExisting(ctx, tx, current)
	if err != nil {
		return models.RegenerateResult{}, err
	}
	regenReason := "regenerated from " + cancelled.PONumber
	if err := s.repo.InsertPOStatusHistoryTx(ctx, tx, newPO.ID,
		nil, newPO.Status, &regenReason, &by); err != nil {
		return models.RegenerateResult{}, fmt.Errorf("orders regenerate: history new: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.RegenerateResult{}, fmt.Errorf("orders regenerate: commit: %w", err)
	}
	return models.RegenerateResult{
		OldPOID:     cancelled.ID,
		NewPOID:     newPO.ID,
		NewPONumber: newNumber,
	}, nil
}

//nolint:funlen,cyclop // pipeline
func (s *Service) rebuildFromExisting(
	ctx context.Context, tx pgx.Tx, current models.PurchaseOrder,
) (models.PurchaseOrder, string, error) {
	plan := models.ApprovedPlan{
		ID:         current.PlanID,
		SupplierID: current.SupplierID,
		LocationID: current.LocationID,
	}
	// При regenerate допустимо использовать тот же набор lines из calculation_lines.
	// run_id неизвестен здесь напрямую (в PurchaseOrder его нет), поэтому fallback —
	// reuse existing po_lines (они идентичны calculation_lines на момент первого build-а).
	legacy, err := s.repo.GetPOLines(ctx, current.ID)
	if err != nil {
		return models.PurchaseOrder{}, "", fmt.Errorf("orders regenerate: legacy lines: %w", err)
	}
	lines := make([]models.PlanLine, 0, len(legacy))
	for _, l := range legacy {
		lines = append(lines, models.PlanLine{
			ProductID:  l.ProductID,
			LocationID: current.LocationID,
			ReorderQty: l.Qty,
		})
	}

	sup, err := s.repo.GetSupplierMaster(ctx, current.SupplierID)
	if err != nil {
		return models.PurchaseOrder{}, "", fmt.Errorf("orders regenerate: supplier master: %w", err)
	}
	productIDs := uniqueProductIDs(lines)
	products := make(map[string]models.ProductMaster, len(productIDs))
	for _, pid := range productIDs {
		pm, err := s.repo.GetProductMaster(ctx, pid)
		if err != nil {
			return models.PurchaseOrder{}, "", fmt.Errorf("orders regenerate: product master %s: %w", pid, err)
		}
		products[pid] = pm
	}

	now := time.Now().UTC()
	seq, err := s.repo.NextSequenceTx(ctx, tx)
	if err != nil {
		return models.PurchaseOrder{}, "", fmt.Errorf("orders regenerate: next seq: %w", err)
	}
	newNumber := numbering.Format(now, seq)
	out := builder.Build(builder.Inputs{
		Plan:      plan,
		Lines:     lines,
		Supplier:  sup,
		Products:  products,
		PONumber:  newNumber,
		CreatedAt: now,
	})
	po, err := s.repo.InsertPurchaseOrderTx(ctx, tx, out.Order)
	if err != nil {
		return models.PurchaseOrder{}, "", fmt.Errorf("orders regenerate: insert PO: %w", err)
	}
	if err := s.repo.InsertPOLinesBulkTx(ctx, tx, po.ID, out.Lines); err != nil {
		return models.PurchaseOrder{}, "", fmt.Errorf("orders regenerate: insert lines: %w", err)
	}
	_ = uuid.Nil // silence import if unused
	return po, newNumber, nil
}
