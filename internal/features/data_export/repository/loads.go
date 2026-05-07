package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// InsertRunning создаёт новую загрузку со status='running'.
// load_id генерится клиентом (uuid.New) — для идемпотентности retry.
func (r *Repository) InsertRunning(ctx context.Context, source string) (models.Load, error) {
	id := uuid.New()
	row := r.pool.QueryRow(ctx, queries.Get("loads_insert_running"), id, source)
	var (
		loadID    uuid.UUID
		startedAt time.Time
		status    string
		src       string
	)
	if err := row.Scan(&loadID, &startedAt, &status, &src); err != nil {
		return models.Load{}, mapError(err)
	}
	return models.Load{
		ID:        loadID,
		StartedAt: startedAt,
		Status:    models.LoadStatus(status),
		Source:    src,
	}, nil
}

// MarkCommitted помечает load как committed; status переходит running→committed.
// entityStatsJSON — сериализованный JSON (jsonb).
func (r *Repository) MarkCommitted(ctx context.Context, tx pgx.Tx, loadID uuid.UUID, linesTotal, linesFailed int64, entityStatsJSON []byte) error {
	exec := r.chooseExec(tx)
	if entityStatsJSON == nil {
		entityStatsJSON = []byte("{}")
	}
	tag, err := exec.Exec(ctx, queries.Get("loads_mark_committed"), loadID, linesTotal, linesFailed, entityStatsJSON)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return errorspkg.ErrCannotRetry.WithMessage("load is not in running state")
	}
	return nil
}

// MarkFailed помечает load как failed.
func (r *Repository) MarkFailed(ctx context.Context, loadID uuid.UUID, reason string) error {
	tag, err := r.pool.Exec(ctx, queries.Get("loads_mark_failed"), loadID, reason)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return errorspkg.ErrCannotRetry.WithMessage("load is not in running state")
	}
	return nil
}

// MarkAborted переводит все running loads со started_at < now() - staleAfter в aborted.
func (r *Repository) MarkAborted(ctx context.Context, staleAfter time.Duration) (int64, error) {
	// staleAfter формируем как PostgreSQL interval string (например, "4 hours").
	intervalStr := staleAfter.String()
	tag, err := r.pool.Exec(ctx, queries.Get("loads_mark_aborted"), intervalStr)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// GetByID возвращает load по id или ErrLoadNotFound.
func (r *Repository) GetByID(ctx context.Context, loadID uuid.UUID) (models.Load, error) {
	row := r.pool.QueryRow(ctx, queries.Get("loads_get_by_id"), loadID)
	var (
		l           models.Load
		committedAt *time.Time
		failedAt    *time.Time
		status      string
		linesTotal  int64
		linesFailed int64
		entStats    []byte
	)
	err := row.Scan(
		&l.ID, &l.StartedAt, &committedAt, &failedAt, &status,
		&l.FailureReason, &l.Source, &linesTotal, &linesFailed, &entStats,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Load{}, errorspkg.ErrLoadNotFound.Wrap(err)
		}
		return models.Load{}, mapError(err)
	}
	l.Status = models.LoadStatus(status)
	if committedAt != nil {
		l.FinishedAt = committedAt
	} else if failedAt != nil {
		l.FinishedAt = failedAt
	}
	l.EntitiesSummary = entStats
	return l, nil
}

// GetRunning возвращает первый running-load (или nil, если нет).
func (r *Repository) GetRunning(ctx context.Context) (*models.Load, error) {
	row := r.pool.QueryRow(ctx, queries.Get("loads_select_running"))
	var (
		l         models.Load
		startedAt time.Time
		source    string
	)
	if err := row.Scan(&l.ID, &startedAt, &source); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapError(err)
	}
	l.StartedAt = startedAt
	l.Source = source
	l.Status = models.LoadStatusRunning
	return &l, nil
}
