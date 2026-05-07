package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// InsertRunInput — alias на models.InsertRunInput (обратная совместимость).
type InsertRunInput = models.InsertRunInput

// InsertRun создаёт новый forecast_run (status=running).
func (r *Repository) InsertRun(ctx context.Context, in models.InsertRunInput) (models.ForecastRun, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("insert_run"), in.HorizonDays, in.SnapshotEtlRunID)
	var run models.ForecastRun
	err := row.Scan(&run.ID, &run.StartedAt, &run.Status, &run.HorizonDays,
		&run.SnapshotEtlRunID, &run.CreatedAt)
	if err != nil {
		return models.ForecastRun{}, fmt.Errorf("forecast: insert run: %w", err)
	}
	return run, nil
}

// UpdateRunCommitted переводит run в committed с финальными счётчиками.
func (r *Repository) UpdateRunCommitted(
	ctx context.Context, runID uuid.UUID, forecasts, lines, plans int,
) error {
	_, err := r.pool.Exec(ctx, queries.MustGet("update_run_committed"),
		runID, forecasts, lines, plans)
	if err != nil {
		return fmt.Errorf("forecast: update run committed: %w", err)
	}
	return nil
}

// UpdateRunFailed переводит run в failed с error_message.
func (r *Repository) UpdateRunFailed(ctx context.Context, runID uuid.UUID, msg string) error {
	_, err := r.pool.Exec(ctx, queries.MustGet("update_run_failed"), runID, msg)
	if err != nil {
		return fmt.Errorf("forecast: update run failed: %w", err)
	}
	return nil
}

// GetRunByID — runs/{id}.
func (r *Repository) GetRunByID(ctx context.Context, id uuid.UUID) (models.ForecastRun, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_run_by_id"), id)
	var run models.ForecastRun
	err := row.Scan(
		&run.ID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.HorizonDays,
		&run.SnapshotEtlRunID, &run.ForecastsCount, &run.LinesCount, &run.PlansCount,
		&run.ErrorMessage, &run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ForecastRun{}, errorspkg.ErrForecastRunNotFound
		}
		return models.ForecastRun{}, fmt.Errorf("forecast: get run: %w", err)
	}
	return run, nil
}

// ListRuns — пагинация по started_at DESC.
func (r *Repository) ListRuns(
	ctx context.Context, f models.RunFilter,
) ([]models.ForecastRun, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	var (
		st       interface{} = nil
		from     interface{} = nil
		to       interface{} = nil
		cursorID interface{} = nil
	)
	if f.Status != nil {
		st = *f.Status
	}
	if f.From != nil {
		from = *f.From
	}
	if f.To != nil {
		to = *f.To
	}
	if f.Cursor != "" {
		id, err := uuid.Parse(f.Cursor)
		if err != nil {
			return nil, "", errorspkg.ErrBadRequest.WithMessage("invalid cursor")
		}
		cursorID = id
	}

	rows, err := r.pool.Query(ctx, queries.MustGet("select_runs"),
		st, from, to, cursorID, limit)
	if err != nil {
		return nil, "", fmt.Errorf("forecast: list runs query: %w", err)
	}
	defer rows.Close()

	out := make([]models.ForecastRun, 0, limit)
	for rows.Next() {
		var run models.ForecastRun
		if scanErr := rows.Scan(
			&run.ID, &run.StartedAt, &run.FinishedAt, &run.Status, &run.HorizonDays,
			&run.SnapshotEtlRunID, &run.ForecastsCount, &run.LinesCount, &run.PlansCount,
			&run.ErrorMessage, &run.CreatedAt,
		); scanErr != nil {
			return nil, "", fmt.Errorf("forecast: list runs scan: %w", scanErr)
		}
		out = append(out, run)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, "", fmt.Errorf("forecast: list runs rows.Err: %w", rowsErr)
	}

	nextCursor := ""
	if len(out) == limit {
		nextCursor = out[len(out)-1].ID.String()
	}
	return out, nextCursor, nil
}

// GetLatestCommittedEtlRunID — для snapshot_etl_run_id привязки.
func (r *Repository) GetLatestCommittedEtlRunID(ctx context.Context) (*uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_latest_committed_etl_run"))
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // legitimate: marts могут быть пустыми
		}
		return nil, fmt.Errorf("forecast: get latest etl_run: %w", err)
	}
	return &id, nil
}
