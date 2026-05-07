package transformer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// DemandHistoryBuilder реализует mart_demand_history (append-семантика
// через ON CONFLICT DO UPDATE). PK — (product_id, location_id, as_of_date).
type DemandHistoryBuilder struct{ repo MartUpserter }

// NewDemandHistoryBuilder DI-конструктор.
func NewDemandHistoryBuilder(repo MartUpserter) *DemandHistoryBuilder {
	return &DemandHistoryBuilder{repo: repo}
}

// Name возвращает имя mart.
func (DemandHistoryBuilder) Name() string { return constants.MartDemandHistory }

// OnDemandOnly — false (входит в полный run).
func (DemandHistoryBuilder) OnDemandOnly() bool { return false }

// Build выполняет UpsertDemandHistory.
func (b *DemandHistoryBuilder) Build(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	n, err := b.repo.UpsertDemandHistory(ctx, tx, runID, sourceLoadID)
	if err != nil {
		return 0, fmt.Errorf("transformer: %s: %w", b.Name(), err)
	}
	return n, nil
}
