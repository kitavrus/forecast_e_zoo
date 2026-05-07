package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/orders/builder"
	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/numbering"
)

// BuildAll — основная точка scheduler/handler-а.
//
// За один вызов:
//   1. в одной tx читает approved plans FOR UPDATE SKIP LOCKED;
//   2. независимыми suъ-транзакциями билдит PO для каждого plan;
//   3. возвращает агрегированный результат.
//
//nolint:funlen,cyclop,gocognit // pipeline orchestration по природе длинная
func (s *Service) BuildAll(ctx context.Context, maxPlans int) (models.BuildResult, error) {
	if maxPlans <= 0 || maxPlans > constants.MaxPlansPerBuildBatch {
		maxPlans = constants.MaxPlansPerBuildBatch
	}
	res := models.BuildResult{RunID: uuid.New()}

	// 1) Reserve plans FOR UPDATE SKIP LOCKED в outer tx, удерживаем lock до commit-ов.
	outerTx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("orders build: begin outer tx: %w", err)
	}
	defer func() { _ = outerTx.Rollback(ctx) }()

	plans, err := s.repo.SelectApprovedPlansForUpdate(ctx, outerTx, maxPlans)
	if err != nil {
		return res, fmt.Errorf("orders build: select approved plans: %w", err)
	}
	if len(plans) == 0 {
		_ = outerTx.Commit(ctx)
		return res, nil
	}

	now := time.Now().UTC()
	for _, p := range plans {
		res.PlansProcessed++
		if err := s.buildOne(ctx, outerTx, p, now, res.RunID); err != nil {
			res.Errors++
			s.logger.ErrorContext(ctx, "orders build: plan failed",
				slog.String("plan_id", p.ID.String()),
				slog.Any("error", err),
			)
			continue
		}
		res.POsCreated++
	}
	if err := outerTx.Commit(ctx); err != nil {
		return res, fmt.Errorf("orders build: commit: %w", err)
	}
	s.logger.InfoContext(ctx, "orders build: done",
		slog.String("run_id", res.RunID.String()),
		slog.Int("plans", res.PlansProcessed),
		slog.Int("pos_created", res.POsCreated),
		slog.Int("errors", res.Errors),
	)
	return res, nil
}

// buildOne — обрабатывает один plan внутри outer tx (один и тот же FOR UPDATE lock).
func (s *Service) buildOne(
	ctx context.Context, tx pgx.Tx, p models.ApprovedPlan, now time.Time, runID uuid.UUID,
) error {
	// 1) lines.
	lines, err := s.repo.SelectPlanLinesForBuild(ctx, p.RunID, p.SupplierID, p.LocationID)
	if err != nil {
		return fmt.Errorf("get plan lines: %w", err)
	}
	if len(lines) == 0 {
		s.logger.WarnContext(ctx, "orders build: plan has zero lines, skipping",
			slog.String("plan_id", p.ID.String()))
		return nil
	}
	// 2) supplier.
	sup, err := s.repo.GetSupplierMaster(ctx, p.SupplierID)
	if err != nil {
		return fmt.Errorf("get supplier master: %w", err)
	}
	// 3) products.
	productIDs := uniqueProductIDs(lines)
	products := make(map[string]models.ProductMaster, len(productIDs))
	for _, pid := range productIDs {
		pm, err := s.repo.GetProductMaster(ctx, pid)
		if err != nil {
			return fmt.Errorf("get product master %s: %w", pid, err)
		}
		products[pid] = pm
	}
	// 4) PO number.
	seq, err := s.repo.NextSequenceTx(ctx, tx)
	if err != nil {
		return fmt.Errorf("next po seq: %w", err)
	}
	poNumber := numbering.Format(now, seq)
	// 5) build inputs.
	out := builder.Build(builder.Inputs{
		Plan:      p,
		Lines:     lines,
		Supplier:  sup,
		Products:  products,
		PONumber:  poNumber,
		CreatedAt: now,
	})
	// 6) write.
	po, err := s.repo.InsertPurchaseOrderTx(ctx, tx, out.Order)
	if err != nil {
		return fmt.Errorf("insert PO: %w", err)
	}
	if err := s.repo.InsertPOLinesBulkTx(ctx, tx, po.ID, out.Lines); err != nil {
		return fmt.Errorf("insert PO lines: %w", err)
	}
	to := po.Status
	reason := "built from approved plan"
	if err := s.repo.InsertPOStatusHistoryTx(ctx, tx, po.ID,
		nil, to, &reason, nil); err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	// 7) mark plan converted.
	if err := s.repo.MarkPlanConvertedTx(ctx, tx, p.ID); err != nil {
		return fmt.Errorf("mark plan converted: %w", err)
	}
	_ = runID
	s.logger.InfoContext(ctx, "orders build: PO created",
		slog.String("po_id", po.ID.String()),
		slog.String("po_number", po.PONumber),
		slog.String("plan_id", p.ID.String()),
	)
	return nil
}

func uniqueProductIDs(lines []models.PlanLine) []string {
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if _, ok := seen[l.ProductID]; ok {
			continue
		}
		seen[l.ProductID] = struct{}{}
		out = append(out, l.ProductID)
	}
	return out
}
