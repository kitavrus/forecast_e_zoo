package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// EtlRunStatusPatch — поля для частичного UPDATE marts.etl_runs.
type EtlRunStatusPatch struct {
	Status         string
	FinishedAt     *time.Time
	CommittedAt    *time.Time
	SourceLoadID   *uuid.UUID
	MartsSummary   []byte // JSONB raw, передаётся как []byte
	FailureReason  *string
	LinesTotal     *int64
	LinesFailed    *int64
}

// InsertEtlRun сохраняет запись marts.etl_runs.
func (r *Repository) InsertEtlRun(ctx context.Context, run *models.EtlRun) error {
	if run == nil {
		return fmt.Errorf("repository: InsertEtlRun: run is nil")
	}
	exec := r.chooseExec(nil)
	_, err := exec.Exec(ctx, queries.MustGet("etl_runs_insert"),
		run.ID,
		run.StartedAt,
		run.Status,
		run.Kind,
		run.TargetMart,
		run.SourceLoadID,
		run.ParentRunID,
		run.Trigger,
		run.Requester,
		run.MartsSummary,
		run.FailureReason,
		run.LinesTotal,
		run.LinesFailed,
	)
	if err != nil {
		return fmt.Errorf("repository: InsertEtlRun: %w", mapPgError(err))
	}
	return nil
}

// GetEtlRunByID возвращает строку marts.etl_runs.
// pgx.ErrNoRows → errorspkg.ErrEtlRunNotFound.
func (r *Repository) GetEtlRunByID(ctx context.Context, id uuid.UUID) (*models.EtlRun, error) {
	exec := r.chooseExec(nil)
	row := exec.QueryRow(ctx, queries.MustGet("etl_runs_get_by_id"), id)
	out, err := scanEtlRun(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errorspkg.ErrEtlRunNotFound.Wrap(err)
		}
		return nil, fmt.Errorf("repository: GetEtlRunByID: %w", err)
	}
	return out, nil
}

// EtlRunListFilter — фильтр для cursor-pagination.
type EtlRunListFilter struct {
	Status     string
	Kind       string
	BeforeTime *time.Time
	Limit      int
}

// ListEtlRuns возвращает страницу runs (cursor pagination по started_at DESC).
func (r *Repository) ListEtlRuns(ctx context.Context, f EtlRunListFilter) ([]models.EtlRun, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	var statusArg, kindArg any
	if f.Status != "" {
		statusArg = f.Status
	}
	if f.Kind != "" {
		kindArg = f.Kind
	}
	var beforeArg any
	if f.BeforeTime != nil {
		beforeArg = *f.BeforeTime
	}
	exec := r.chooseExec(nil)
	rows, err := exec.Query(ctx, queries.MustGet("etl_runs_list"), statusArg, kindArg, beforeArg, limit)
	if err != nil {
		return nil, fmt.Errorf("repository: ListEtlRuns: %w", err)
	}
	defer rows.Close()

	out := make([]models.EtlRun, 0, limit)
	for rows.Next() {
		run, scanErr := scanEtlRun(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("repository: ListEtlRuns scan: %w", scanErr)
		}
		out = append(out, *run)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, fmt.Errorf("repository: ListEtlRuns rows.Err: %w", rerr)
	}
	return out, nil
}

// UpdateEtlRunStatus — частичный UPDATE marts.etl_runs (вне транзакции).
func (r *Repository) UpdateEtlRunStatus(ctx context.Context, id uuid.UUID, p EtlRunStatusPatch) error {
	return r.updateEtlRunStatus(ctx, r.chooseExec(nil), id, p)
}

// UpdateEtlRunStatusTx — то же, что UpdateEtlRunStatus, но в рамках tx.
// Используется loader-ом для атомарного flip-а.
func (r *Repository) UpdateEtlRunStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, p EtlRunStatusPatch) error {
	if tx == nil {
		return fmt.Errorf("repository: UpdateEtlRunStatusTx: tx is required")
	}
	return r.updateEtlRunStatus(ctx, tx, id, p)
}

func (r *Repository) updateEtlRunStatus(ctx context.Context, exec queryExecutor, id uuid.UUID, p EtlRunStatusPatch) error {
	_, err := exec.Exec(ctx, queries.MustGet("etl_runs_update_status"),
		id,
		p.Status,
		p.FinishedAt,
		p.CommittedAt,
		p.SourceLoadID,
		p.MartsSummary,
		p.FailureReason,
		p.LinesTotal,
		p.LinesFailed,
	)
	if err != nil {
		return fmt.Errorf("repository: UpdateEtlRunStatus: %w", mapPgError(err))
	}
	return nil
}

// GetCurrentRunningEtlRun возвращает последний run со status='running'.
// Если ни одного — errorspkg.ErrEtlRunNotFound.
func (r *Repository) GetCurrentRunningEtlRun(ctx context.Context) (*models.EtlRun, error) {
	exec := r.chooseExec(nil)
	row := exec.QueryRow(ctx, queries.MustGet("etl_runs_get_running"))
	out, err := scanEtlRun(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errorspkg.ErrEtlRunNotFound.Wrap(err)
		}
		return nil, fmt.Errorf("repository: GetCurrentRunningEtlRun: %w", err)
	}
	return out, nil
}

// rowScanner — общий интерфейс для pgx.Row и pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanEtlRun — приватный helper, чтобы не дублировать список колонок.
func scanEtlRun(row rowScanner) (*models.EtlRun, error) {
	var r models.EtlRun
	err := row.Scan(
		&r.ID,
		&r.StartedAt,
		&r.FinishedAt,
		&r.CommittedAt,
		&r.Status,
		&r.Kind,
		&r.TargetMart,
		&r.SourceLoadID,
		&r.ParentRunID,
		&r.Trigger,
		&r.Requester,
		&r.MartsSummary,
		&r.FailureReason,
		&r.LinesTotal,
		&r.LinesFailed,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return nil, err //nolint:wrapcheck // wrapping is done by callers
	}
	return &r, nil
}
