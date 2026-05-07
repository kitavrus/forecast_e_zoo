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
// Стадии (Q-005, Q-008, Q-017, ADR-005):
//  1. Snapshot: GET /v1/snapshots/current → source_load_id зафиксирован.
//  2. Source load id → UPDATE etl_runs (вне основной tx, для трассировки).
//  3. Extract+stage: full read NDJSON для всех 16 entity, накопление в Dataset.
//  4. Validate: engine.Run(dataset) → reject_log + counters lines_total/failed.
//  5. Quality gate: lines_failed/lines_total > 1% → markFailed(quality_threshold).
//  6. Load (atomic flip): loader.Apply открывает tx, populateStaging COPY-загружает
//     stg_* в той же tx, mart-builder-ы строят marts, UpdateEtlRunStatusTx
//     помечает run committed; всё в одной tx.
//
//nolint:cyclop,funlen // линейный путь через стадии; разбивка ухудшит читаемость.
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

	// 1. Snapshot.
	snap, err := p.extr.GetCurrentSnapshot(ctx)
	if err != nil {
		p.markFailed(ctx, runID, fmt.Sprintf("snapshot: %v", err))
		p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "snapshot")
		return
	}
	sourceLoadID, parseErr := uuid.Parse(snap.CurrentLoadID)
	if parseErr != nil {
		p.markFailed(ctx, runID, fmt.Sprintf("bad source load id: %v", parseErr))
		p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "bad_source_load_id")
		return
	}

	// 2. Зафиксировать source_load_id в etl_runs (best-effort: ошибка лишь
	//    логируется, не валит run — атомарная фиксация всё равно произойдёт
	//    в loader.Apply через UpdateEtlRunStatusTx).
	if err := p.repo.UpdateEtlRunStatus(ctx, runID, repository.EtlRunStatusPatch{
		Status:       constants.StatusRunning,
		SourceLoadID: &sourceLoadID,
	}); err != nil {
		p.logger.WarnContext(ctx, "pipeline: persist source_load_id failed",
			"run_id", runID.String(), "err", err)
	}

	// 3. Extract + stage (in-memory).
	staged, err := extractAllEntities(ctx, p.extr, snap)
	if err != nil {
		p.markFailed(ctx, runID, fmt.Sprintf("extract: %v", err))
		p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "extract")
		return
	}

	// 4. Validate.
	report := p.engine.Run(staged.dataset)

	// LinesTotal в report равен сумме строк по entities (CountAll), либо может
	// быть скорректирован engine — берём максимум, чтобы quality gate имел
	// корректный знаменатель даже если engine не считал.
	linesTotal := int64(report.LinesTotal)
	if linesTotal < staged.linesTotal {
		linesTotal = staged.linesTotal
	}
	linesFailed := int64(report.LinesFailed)

	// 4a. Persist violations → reject_log (best-effort: ошибка лишь логируется,
	//     но не блокирует pipeline — quality threshold уже учитывает linesFailed).
	if len(report.Violations) > 0 {
		entries := violationsToRejectEntries(runID, report.Violations)
		if _, perr := p.repo.InsertRejectEntries(ctx, entries); perr != nil {
			p.logger.WarnContext(ctx, "pipeline: insert reject_log failed",
				"run_id", runID.String(), "violations", len(entries), "err", perr)
		}
	}

	// 5. Quality threshold.
	if linesTotal > 0 {
		failureRate := float64(linesFailed) / float64(linesTotal)
		if failureRate > p.cfg.QualityThreshold {
			p.markFailed(ctx, runID,
				fmt.Sprintf("quality threshold %.4f exceeded (failed=%d/total=%d)",
					p.cfg.QualityThreshold, linesFailed, linesTotal))
			p.metrics.RecordRunFailure(time.Since(startedAt).Seconds(), "quality_threshold")
			return
		}
	}

	// 6. Load (atomic flip): staging populate + mart-builders + etl_runs commit.
	builders := p.registry.BuildersForFullRun()
	summary, err := p.loader.Apply(ctx, loader.ApplyParams{
		RunID:           runID,
		SourceLoadID:    sourceLoadID,
		Builders:        builders,
		LinesTotal:      linesTotal,
		LinesFailed:     linesFailed,
		PopulateStaging: populateStaging(staged.rowsByEnt),
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
		"run_id", runID.String(),
		"rows", summary.Total(),
		"lines_total", linesTotal,
		"lines_failed", linesFailed)
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


