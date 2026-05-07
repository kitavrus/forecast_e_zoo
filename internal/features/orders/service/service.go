// Package service — бизнес-логика поверх repository, builder и numbering.
//
// Орchestrate'ит:
//   - читает approved plans (FOR UPDATE SKIP LOCKED) внутри tx;
//   - per plan: подгружает supplier+product masters, генерирует PO number,
//     зовёт builder.Build, INSERT-ит PO+lines+history, помечает plan converted;
//   - транзакция per plan (independent failure isolation).
package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Repo — узкий интерфейс repository для service (DI seam).
type Repo interface {
	// Build path
	SelectApprovedPlansForUpdate(ctx context.Context, tx pgx.Tx, limit int) ([]models.ApprovedPlan, error)
	SelectPlanLinesForBuild(ctx context.Context, runID uuid.UUID, supplier, location string) ([]models.PlanLine, error)
	GetSupplierMaster(ctx context.Context, supplierID string) (models.SupplierMaster, error)
	GetProductMaster(ctx context.Context, productID string) (models.ProductMaster, error)
	NextSequenceTx(ctx context.Context, tx pgx.Tx) (int64, error)
	InsertPurchaseOrderTx(ctx context.Context, tx pgx.Tx, in repository.InsertPOInput) (models.PurchaseOrder, error)
	InsertPOLinesBulkTx(ctx context.Context, tx pgx.Tx, poID uuid.UUID, lines []repository.BulkLine) error
	InsertPOStatusHistoryTx(ctx context.Context, tx pgx.Tx, poID uuid.UUID, fromStatus *string, toStatus string, reason, changedBy *string) error
	MarkPlanConvertedTx(ctx context.Context, tx pgx.Tx, planID uuid.UUID) error

	// Query path
	GetPOByID(ctx context.Context, id uuid.UUID) (models.PurchaseOrder, error)
	GetPOLines(ctx context.Context, poID uuid.UUID) ([]models.POLine, error)
	GetPOHistory(ctx context.Context, poID uuid.UUID) ([]models.POStatusHistory, error)
	ListPOs(ctx context.Context, f models.POFilter) ([]models.PurchaseOrder, string, error)
	CancelPO(ctx context.Context, id uuid.UUID, reason string) (models.PurchaseOrder, error)
	CancelPOTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, reason string) (models.PurchaseOrder, error)
}

// Trigger — interface scheduler-а для on-demand build.
type Trigger interface {
	TryTrigger(ctx context.Context, maxPlans int) (uuid.UUID, bool, error)
}

// Service — orchestrator для handlers.
type Service struct {
	repo    Repo
	pool    *pgxpool.Pool
	trigger Trigger
	logger  *slog.Logger
}

// New собирает Service.
func New(repo Repo, pool *pgxpool.Pool, trigger Trigger, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, pool: pool, trigger: trigger, logger: logger}
}

// ListPOs — pass-through.
func (s *Service) ListPOs(
	ctx context.Context, f models.POFilter,
) ([]models.PurchaseOrder, string, error) {
	return s.repo.ListPOs(ctx, f) //nolint:wrapcheck
}

// GetPOWithDetails — PO + lines + history.
func (s *Service) GetPOWithDetails(
	ctx context.Context, id uuid.UUID,
) (models.POWithDetails, error) {
	po, err := s.repo.GetPOByID(ctx, id)
	if err != nil {
		return models.POWithDetails{}, err //nolint:wrapcheck
	}
	lines, err := s.repo.GetPOLines(ctx, id)
	if err != nil {
		return models.POWithDetails{}, fmt.Errorf("orders service: get PO lines: %w", err)
	}
	history, err := s.repo.GetPOHistory(ctx, id)
	if err != nil {
		return models.POWithDetails{}, fmt.Errorf("orders service: get PO history: %w", err)
	}
	return models.POWithDetails{Order: po, Lines: lines, History: history}, nil
}

// TriggerBuild — POST /v1/orders/purchase-orders/build.
func (s *Service) TriggerBuild(ctx context.Context, maxPlans int) (uuid.UUID, bool, error) {
	if s.trigger == nil {
		return uuid.Nil, false, errorspkg.ErrOrderBuilderUnavailable
	}
	id, started, err := s.trigger.TryTrigger(ctx, maxPlans)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("orders service: trigger build: %w", err)
	}
	return id, started, nil
}

// Cancel — переводит PO в cancelled с audit-записью.
//
//nolint:cyclop // last-status fetch + tx flow
func (s *Service) Cancel(
	ctx context.Context, in models.CancelInput,
) (models.PurchaseOrder, error) {
	if s.pool == nil {
		// fallback без транзакции (нет audit-записи); используется только в unit-тестах сервиса
		// без pool. В prod pool всегда выставлен.
		return s.repo.CancelPO(ctx, in.POID, in.Reason) //nolint:wrapcheck
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.PurchaseOrder{}, fmt.Errorf("orders service: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Получаем текущее состояние для from_status в audit log.
	current, err := s.repo.GetPOByID(ctx, in.POID)
	if err != nil {
		return models.PurchaseOrder{}, err //nolint:wrapcheck
	}
	if !constants.IsCancellableStatus(current.Status) {
		return models.PurchaseOrder{}, errorspkg.ErrPONotCancellable
	}
	po, err := s.repo.CancelPOTx(ctx, tx, in.POID, in.Reason)
	if err != nil {
		return models.PurchaseOrder{}, err //nolint:wrapcheck
	}
	from := current.Status
	reason := in.Reason
	by := in.ChangedBy
	if err := s.repo.InsertPOStatusHistoryTx(ctx, tx, po.ID,
		&from, po.Status, &reason, &by); err != nil {
		return models.PurchaseOrder{}, fmt.Errorf("orders service: cancel history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.PurchaseOrder{}, fmt.Errorf("orders service: commit cancel: %w", err)
	}
	return po, nil
}
