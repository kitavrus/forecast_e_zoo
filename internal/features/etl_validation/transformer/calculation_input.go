package transformer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// CalculationInputBuilder — TRUNCATE + INSERT с CTE rule_priority
// (order_rule > supply_spec, ADR-024). См. SQL phase 07.
type CalculationInputBuilder struct{ repo MartUpserter }

// NewCalculationInputBuilder — DI-конструктор.
func NewCalculationInputBuilder(repo MartUpserter) *CalculationInputBuilder {
	return &CalculationInputBuilder{repo: repo}
}

// Name возвращает имя mart.
func (CalculationInputBuilder) Name() string { return constants.MartCalculationInput }

// OnDemandOnly — false.
func (CalculationInputBuilder) OnDemandOnly() bool { return false }

// Build делегирует RebuildCalculationInput.
func (b *CalculationInputBuilder) Build(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	n, err := b.repo.RebuildCalculationInput(ctx, tx, runID, sourceLoadID)
	if err != nil {
		return 0, fmt.Errorf("transformer: %s: %w", b.Name(), err)
	}
	return n, nil
}
