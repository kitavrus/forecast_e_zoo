package transformer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// SupplierScorecardBuilder — on-demand mart (запускается только через
// POST /admin/marts/mart_supplier_scorecard/refresh, см. ADR-021).
type SupplierScorecardBuilder struct{ repo MartUpserter }

// NewSupplierScorecardBuilder — DI-конструктор.
func NewSupplierScorecardBuilder(repo MartUpserter) *SupplierScorecardBuilder {
	return &SupplierScorecardBuilder{repo: repo}
}

// Name возвращает имя mart.
func (SupplierScorecardBuilder) Name() string { return constants.MartSupplierScorecard }

// OnDemandOnly — true (НЕ запускается в обычном full run).
func (SupplierScorecardBuilder) OnDemandOnly() bool { return true }

// Build делегирует UpsertSupplierScorecard.
func (b *SupplierScorecardBuilder) Build(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	n, err := b.repo.UpsertSupplierScorecard(ctx, tx, runID, sourceLoadID)
	if err != nil {
		return 0, fmt.Errorf("transformer: %s: %w", b.Name(), err)
	}
	return n, nil
}
