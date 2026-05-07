package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/loader"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// EtlPipelineConfig — конфигурация EtlPipeline.
type EtlPipelineConfig struct {
	QualityThreshold float64       // ADR-015: 0.01 (1%)
	LockKey          int64         // ключ pg_try_advisory_xact_lock
	RunTimeout       time.Duration // максимум на runAsync (default 1h)
}

// AdvisoryLockKey — детерминированный ключ для pg_try_advisory_xact_lock.
//
// Используется глобально: все запуски ETL run конкурируют за один ключ.
func AdvisoryLockKey() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("etl-run"))
	// pg accepts signed bigint; FNV возвращает uint64 — приводим в int64.
	return int64(h.Sum64()) //nolint:gosec // intentional reinterpret
}

// EtlPipeline — orchestrator-сервис.
type EtlPipeline struct {
	pool      Pool
	repo      Repo
	extr      Extractor
	engine    ValidationEngine
	registry  Registry
	loader    LoaderIface
	metrics   Metrics
	logger    *slog.Logger
	cfg       EtlPipelineConfig
}

// NewEtlPipeline — DI-конструктор.
func NewEtlPipeline(
	pool Pool, repo Repo,
	extr Extractor, engine ValidationEngine,
	registry Registry, l LoaderIface,
	metrics Metrics, log *slog.Logger,
	cfg EtlPipelineConfig,
) *EtlPipeline {
	if log == nil {
		log = slog.Default()
	}
	if metrics == nil {
		metrics = NoopMetrics{}
	}
	if cfg.LockKey == 0 {
		cfg.LockKey = AdvisoryLockKey()
	}
	if cfg.RunTimeout <= 0 {
		cfg.RunTimeout = time.Hour
	}
	return &EtlPipeline{
		pool: pool, repo: repo, extr: extr, engine: engine,
		registry: registry, loader: l, metrics: metrics, logger: log,
		cfg: cfg,
	}
}

// TryStart запускает full-run ETL, асинхронно. Возвращает свежесозданный run.
//
// Контракт:
//   - захватывает advisory-lock в короткой tx и проверяет занятость;
//   - если другой run выполняется → errorspkg.ErrEtlRunAlreadyRunning + текущий running;
//   - INSERT run со status='running';
//   - запускает goroutine с detached context на runAsync.
//
// HTTP-handler возвращает 202 Accepted с body=run.
func (p *EtlPipeline) TryStart(ctx context.Context, trigger string, requester *string, parentRunID *uuid.UUID) (*models.EtlRun, error) {
	if !isValidTrigger(trigger) {
		return nil, errorspkg.NewBadRequest("Некорректный trigger")
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("service: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ok, err := p.repo.TryAdvisoryXactLock(ctx, tx, p.cfg.LockKey)
	if err != nil {
		return nil, fmt.Errorf("service: try lock: %w", err)
	}
	if !ok {
		return nil, errorspkg.ErrEtlRunAlreadyRunning
	}

	run := &models.EtlRun{
		ID:          uuid.New(),
		StartedAt:   time.Now().UTC(),
		Status:      constants.StatusRunning,
		Kind:        constants.KindFull,
		Trigger:     trigger,
		Requester:   requester,
		ParentRunID: parentRunID,
	}
	if err := p.repo.InsertEtlRun(ctx, run); err != nil {
		return nil, fmt.Errorf("service: insert etl_run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("service: commit init tx: %w", err)
	}

	// Detached context — handler не должен отменять long-running pipeline
	// при возврате 202; runAsync создаёт собственный context.WithTimeout.
	go p.runAsync(run.ID) //nolint:gosec // see comment
	return run, nil
}

// runAsync — основная цепочка обработки. На любой ошибке вызывает markFailed.
//
//nolint:cyclop,funlen // линейный путь через стадии Extract→Validate→Load.
func (p *EtlPipeline) runAsync(runID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.RunTimeout)
	defer cancel()
	startedAt := time.Now()

	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("pipeline: panic recovered", "run_id", runID.String(), "panic", r)
			p.markFailed(ctx, runID, fmt.Sprintf("panic: %v", r))
			p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "panic")
		}
	}()

	// 1. Extract: snapshot.
	snap, err := p.extr.GetCurrentSnapshot(ctx)
	if err != nil {
		p.markFailed(ctx, runID, fmt.Sprintf("snapshot: %v", err))
		p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "snapshot")
		return
	}
	sourceLoadID, parseErr := uuid.Parse(snap.LoadID)
	if parseErr != nil {
		p.markFailed(ctx, runID, fmt.Sprintf("bad source load id: %v", parseErr))
		p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "bad_source_load_id")
		return
	}

	// 2/3/4: реальный extract+stage+validate был бы тут (Phase 13/14 интеграция).
	// Для MVP: пустой Dataset → quality gate проходит автоматически.
	dataset := validation.NewDataset()
	report := p.engine.Run(dataset)

	// 5. Quality threshold.
	if report.LinesTotal > 0 {
		failureRate := float64(report.LinesFailed) / float64(report.LinesTotal)
		if failureRate > p.cfg.QualityThreshold {
			p.markFailed(ctx, runID,
				fmt.Sprintf("quality threshold %.4f exceeded (failed=%d/total=%d)",
					p.cfg.QualityThreshold, report.LinesFailed, report.LinesTotal))
			p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "quality_threshold")
			return
		}
	}

	// 6. Load: вызываем loader.Apply со всеми full-run builder-ами.
	builders := p.registry.BuildersForFullRun()
	summary, err := p.loader.Apply(ctx, loader.ApplyParams{
		RunID:        runID,
		SourceLoadID: sourceLoadID,
		Builders:     builders,
		LinesTotal:   int64(report.LinesTotal),
		LinesFailed:  int64(report.LinesFailed),
	})
	if err != nil {
		p.markFailed(ctx, runID, fmt.Sprintf("loader: %v", err))
		p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "loader")
		return
	}
	for k, v := range summary {
		p.metrics.RecordRowsProcessed(k, v)
	}
	p.metrics.RecordRunSuccess(time.Since(startedAt).Seconds())
	p.logger.InfoContext(ctx, "pipeline: run committed",
		"run_id", runID.String(), "rows", summary.Total())
}

// markFailed — UPDATE etl_runs.status='failed', failure_reason.
//
// Отдельная транзакция — независимо от состояния основной tx loader-а.
func (p *EtlPipeline) markFailed(ctx context.Context, runID uuid.UUID, reason string) {
	now := time.Now().UTC()
	patch := repository.EtlRunStatusPatch{
		Status:        constants.StatusFailed,
		FinishedAt:    &now,
		FailureReason: &reason,
	}
	if err := p.repo.UpdateEtlRunStatus(ctx, runID, patch); err != nil {
		p.logger.Error("pipeline: markFailed: update etl_run failed",
			"run_id", runID.String(), "err", err)
	}
}

// CurrentRunning — публичный helper для admin handler (если нужен).
func (p *EtlPipeline) CurrentRunning(ctx context.Context) (*models.EtlRun, error) {
	return p.repo.GetCurrentRunningEtlRun(ctx)
}

func isValidTrigger(t string) bool {
	for _, v := range constants.EtlRunTriggers {
		if t == v {
			return true
		}
	}
	return false
}

