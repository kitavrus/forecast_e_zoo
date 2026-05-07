package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Seed гарантирует наличие single-row snapshot_pointer (id=1). Идемпотентен.
func (r *Repository) Seed(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, queries.Get("snapshot_seed"))
	return mapError(err)
}

// GetCurrent возвращает текущий snapshot_pointer.
// Если current_load_id IS NULL — возвращает ErrSnapshotNotReady (503).
func (r *Repository) GetCurrent(ctx context.Context) (models.SnapshotPointer, error) {
	row := r.pool.QueryRow(ctx, queries.Get("snapshot_select_current"))
	var sp models.SnapshotPointer
	if err := row.Scan(&sp.CurrentLoadID, &sp.PreviousLoadID, &sp.CommittedAt); err != nil {
		return models.SnapshotPointer{}, mapError(err)
	}
	if sp.CurrentLoadID == nil {
		return sp, errorspkg.ErrSnapshotNotReady
	}
	return sp, nil
}

// Flip атомарно переключает snapshot_pointer на новый load_id. Должен вызываться
// внутри транзакции loader-а (после успешного UPSERT всех сущностей).
func (r *Repository) Flip(ctx context.Context, tx pgx.Tx, loadID uuid.UUID) (models.SnapshotPointer, error) {
	if tx == nil {
		return models.SnapshotPointer{}, errorspkg.ErrInternal.WithMessage("Flip requires non-nil tx")
	}
	row := tx.QueryRow(ctx, queries.Get("snapshot_flip"), loadID)
	var (
		current   *uuid.UUID
		previous  *uuid.UUID
		committed *time.Time
	)
	if err := row.Scan(&current, &previous, &committed); err != nil {
		return models.SnapshotPointer{}, mapError(err)
	}
	return models.SnapshotPointer{
		CurrentLoadID:  current,
		PreviousLoadID: previous,
		CommittedAt:    committed,
	}, nil
}
