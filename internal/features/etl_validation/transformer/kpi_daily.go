package transformer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// KpiDailyBuilder — append-семантика per-day rows.
type KpiDailyBuilder struct{ repo MartUpserter }

// NewKpiDailyBuilder — DI-конструктор.
func NewKpiDailyBuilder(repo MartUpserter) *KpiDailyBuilder {
	return &KpiDailyBuilder{repo: repo}
}

// Name возвращает имя mart.
func (KpiDailyBuilder) Name() string { return constants.MartKpiDaily }

// OnDemandOnly — false.
func (KpiDailyBuilder) OnDemandOnly() bool { return false }

// Build делегирует UpsertKpiDaily.
func (b *KpiDailyBuilder) Build(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	n, err := b.repo.UpsertKpiDaily(ctx, tx, runID, sourceLoadID)
	if err != nil {
		return 0, fmt.Errorf("transformer: %s: %w", b.Name(), err)
	}
	return n, nil
}
