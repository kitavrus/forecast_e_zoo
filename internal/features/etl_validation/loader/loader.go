package loader

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/transformer"
)

// PoolBeginner — минимальный интерфейс к pgxpool, чтобы упростить тесты.
type PoolBeginner interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// EtlRunsUpdater — узкий контракт для обновления etl_runs (тестируемая абстракция).
//
// UpdateEtlRunStatusTx обязан выполняться в переданной tx, чтобы flip
// был атомарным с mart-builder-ами.
type EtlRunsUpdater interface {
	UpdateEtlRunStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, p repository.EtlRunStatusPatch) error
	CreateStagingTables(ctx context.Context, tx pgx.Tx) error
}

// ApplyParams — параметры одного запуска Apply.
//
// PopulateStaging (опциональный) — вызывается после CreateStagingTables, до
// Builders[].Build. Используется pipeline-ом для COPY-загрузки в pg_temp.stg_*
// в той же tx, чтобы mart-builder-ы видели данные. nil → staging остаётся пустым
// (legacy-поведение для тестов и mart_refresh).
type ApplyParams struct {
	RunID           uuid.UUID
	SourceLoadID    uuid.UUID
	Builders        []transformer.Builder
	LinesTotal      int64
	LinesFailed     int64
	PopulateStaging func(ctx context.Context, tx pgx.Tx) error
}

// Loader — публичный интерфейс.
type Loader interface {
	Apply(ctx context.Context, p ApplyParams) (BuildSummary, error)
}

// Impl — конкретная реализация.
type Impl struct {
	pool   PoolBeginner
	repo   EtlRunsUpdater
	logger *slog.Logger
}

// New собирает Impl.
func New(pool PoolBeginner, repo EtlRunsUpdater, log *slog.Logger) *Impl {
	if log == nil {
		log = slog.Default()
	}
	return &Impl{pool: pool, repo: repo, logger: log}
}

// AssertPoolOK — guard, что pool — *pgxpool.Pool. Помогает поймать DI-ошибки.
func AssertPoolOK(p *pgxpool.Pool) error {
	if p == nil {
		return errors.New("loader: pgx pool is nil")
	}
	return nil
}

// Apply выполняет атомарный snapshot-flip: внутри одной транзакции
// 1) создаёт staging-таблицы (но не наполняет — это делает pipeline через CopyFrom),
// 2) последовательно вызывает Builder.Build,
// 3) обновляет etl_runs.status='committed' с marts_summary/timestamps.
//
// Контракт: caller (pipeline) уже наполнил staging-таблицы через
// repository.BulkInsertStaging до вызова Apply (в той же tx? нет —
// staging создаётся здесь, и весь pipeline выполняется в одной tx).
//
// Семантика:
//   - При ошибке любого Builder.Build — tx.Rollback(), возвращаем ошибку.
//   - При успехе — tx.Commit(), возвращаем BuildSummary.
//   - Caller обязан, в случае ошибки, отдельной транзакцией пометить
//     etl_run как 'failed' (loader.Apply этого не делает — это уже логика pipeline).
//
//nolint:cyclop,funlen // путь линейный (begin → builders → update → commit), сложность невысокая.
func (l *Impl) Apply(ctx context.Context, p ApplyParams) (BuildSummary, error) {
	if p.RunID == uuid.Nil {
		return nil, errors.New("loader: RunID is nil")
	}
	if p.SourceLoadID == uuid.Nil {
		return nil, errors.New("loader: SourceLoadID is nil")
	}
	if len(p.Builders) == 0 {
		return nil, errors.New("loader: builders empty")
	}

	tx, err := l.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("loader: begin tx: %w", err)
	}
	// Гарантируем rollback при любом раннем выходе (после Commit это no-op).
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Staging — создаём в начале tx; caller (pipeline) при необходимости
	// потом наполняет через BulkInsertStaging в той же tx (фаза 13).
	if err := l.repo.CreateStagingTables(ctx, tx); err != nil {
		return nil, fmt.Errorf("loader: staging: %w", err)
	}

	// Опциональная стадия наполнения staging — pipeline (full-run) передаёт сюда
	// CopyFrom-загрузку из in-memory Dataset; mart_refresh — нет.
	if p.PopulateStaging != nil {
		if err := p.PopulateStaging(ctx, tx); err != nil {
			return nil, fmt.Errorf("loader: populate staging: %w", err)
		}
	}

	summary := NewBuildSummary()
	for _, b := range p.Builders {
		l.logger.InfoContext(ctx, "loader: building mart",
			"mart", b.Name(), "run_id", p.RunID.String())
		rows, buildErr := b.Build(ctx, tx, p.RunID, p.SourceLoadID)
		if buildErr != nil {
			return nil, fmt.Errorf("loader: build %s: %w", b.Name(), buildErr)
		}
		summary.Add(b.Name(), rows)
	}

	jsonb, err := summary.MarshalJSONB()
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped in MarshalJSONB
	}
	now := time.Now().UTC()
	patch := repository.EtlRunStatusPatch{
		Status:       constants.StatusCommitted,
		FinishedAt:   &now,
		CommittedAt:  &now,
		SourceLoadID: &p.SourceLoadID,
		MartsSummary: jsonb,
		LinesTotal:   &p.LinesTotal,
		LinesFailed:  &p.LinesFailed,
	}
	if err := l.repo.UpdateEtlRunStatusTx(ctx, tx, p.RunID, patch); err != nil {
		return nil, fmt.Errorf("loader: update etl_run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("loader: commit: %w", err)
	}
	return summary, nil
}
