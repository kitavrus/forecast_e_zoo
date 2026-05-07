package transformer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// MasterCurrentBuilder — TRUNCATE + INSERT текущего snapshot справочников.
type MasterCurrentBuilder struct{ repo MartUpserter }

// NewMasterCurrentBuilder — DI-конструктор.
func NewMasterCurrentBuilder(repo MartUpserter) *MasterCurrentBuilder {
	return &MasterCurrentBuilder{repo: repo}
}

// Name возвращает имя mart.
func (MasterCurrentBuilder) Name() string { return constants.MartMasterCurrent }

// OnDemandOnly — false.
func (MasterCurrentBuilder) OnDemandOnly() bool { return false }

// Build делегирует RebuildMasterCurrent.
func (b *MasterCurrentBuilder) Build(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error) {
	n, err := b.repo.RebuildMasterCurrent(ctx, tx, runID, sourceLoadID)
	if err != nil {
		return 0, fmt.Errorf("transformer: %s: %w", b.Name(), err)
	}
	return n, nil
}
