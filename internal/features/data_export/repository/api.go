package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
)

// LoaderAPI — суженный интерфейс для loader-а. Реальная *Repository
// автоматически удовлетворяет этому интерфейсу. Mock используется в
// loader-тестах для проверки последовательности вызовов.
//
//nolint:interfacebloat // intentionally one mock target for loader pipeline
type LoaderAPI interface {
	InsertRunning(ctx context.Context, source string) (models.Load, error)
	MarkCommitted(ctx context.Context, tx pgx.Tx, loadID uuid.UUID, linesTotal, linesFailed int64, entityStatsJSON []byte) error
	MarkFailed(ctx context.Context, loadID uuid.UUID, reason string) error

	Flip(ctx context.Context, tx pgx.Tx, loadID uuid.UUID) (models.SnapshotPointer, error)

	InsertReject(ctx context.Context, in RejectInput) error

	UpsertCategory(ctx context.Context, tx pgx.Tx, c CategoryRow, loadID uuid.UUID) error
	UpsertSupplier(ctx context.Context, tx pgx.Tx, s SupplierRow, loadID uuid.UUID) error
	UpsertLocation(ctx context.Context, tx pgx.Tx, l LocationRow, loadID uuid.UUID) error
	UpsertProduct(ctx context.Context, tx pgx.Tx, p ProductRow, loadID uuid.UUID) error
	UpsertOrderRule(ctx context.Context, tx pgx.Tx, row OrderRuleRow, loadID uuid.UUID) error
	UpsertSupplySpec(ctx context.Context, tx pgx.Tx, row SupplySpecRow, loadID uuid.UUID) error
	UpsertLocationStockSnapshot(ctx context.Context, tx pgx.Tx, row LocationStockSnapshotRow, loadID uuid.UUID) error
	InsertReceiptLineBatch(ctx context.Context, tx pgx.Tx, batch []ReceiptLineRow, loadID uuid.UUID) error

	// Phase 13 — 8 missing entity loaders.
	UpsertProductBarcode(ctx context.Context, tx pgx.Tx, row ProductBarcodeRow, loadID uuid.UUID) error
	UpsertPromo(ctx context.Context, tx pgx.Tx, row PromoRow, loadID uuid.UUID) error
	UpsertSupplyPlan(ctx context.Context, tx pgx.Tx, row SupplyPlanRow, loadID uuid.UUID) error
	UpsertStoreAssortment(ctx context.Context, tx pgx.Tx, row StoreAssortmentRow, loadID uuid.UUID) error
	InsertLifecycleEventBatch(ctx context.Context, tx pgx.Tx, batch []LifecycleEventRow, loadID uuid.UUID) error
	InsertMasterChangeLogBatch(ctx context.Context, tx pgx.Tx, batch []MasterChangeLogRow, loadID uuid.UUID) error
	InsertStockMovementBatch(ctx context.Context, tx pgx.Tx, batch []StockMovementRow, loadID uuid.UUID) error
	InsertSupplierStockSnapshotBatch(ctx context.Context, tx pgx.Tx, batch []SupplierStockSnapshotRow, loadID uuid.UUID) error

	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// BeginTx — обёртка над pool.Begin для удобства мока.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, pgx.TxOptions{})
}

// compile-time check.
var _ LoaderAPI = (*Repository)(nil)

// _ usage of time/uuid silenced — keep imports stable.
var _ = time.Time{}
var _ = uuid.Nil
