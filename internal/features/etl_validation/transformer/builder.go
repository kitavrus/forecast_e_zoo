// Package transformer содержит реализации mart-builder-ов для ETL pipeline:
// каждый builder выполняет SQL INSERT-FROM-SELECT/TRUNCATE+INSERT из staging
// в одну mart-таблицу. Все запросы — go:embed из sqls/queries (Phase 07).
package transformer

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MartUpserter — узкий интерфейс к repository, чтобы избежать прямой
// зависимости от *repository.Repository (упрощает unit-тесты builder-ов).
type MartUpserter interface {
	UpsertDemandHistory(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error)
	RebuildCalculationInput(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error)
	UpsertKpiDaily(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error)
	RebuildMasterCurrent(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error)
	UpsertSupplierScorecard(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error)
}

// Builder — единый контракт mart-builder-а.
type Builder interface {
	// Name — имя mart (совпадает с marts.<table>).
	Name() string
	// OnDemandOnly — true для mart-ов, которые НЕ запускаются в полном full run
	// (например, mart_supplier_scorecard — только через POST /admin/marts/.../refresh).
	OnDemandOnly() bool
	// Build выполняет INSERT-FROM-SELECT в mart-таблицу.
	// Возвращает количество inserted/updated rows.
	Build(ctx context.Context, tx pgx.Tx, runID, sourceLoadID uuid.UUID) (int64, error)
}
