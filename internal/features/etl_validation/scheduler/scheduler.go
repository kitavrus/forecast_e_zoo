package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Pipeline — узкий контракт к service.EtlPipeline (избегаем cyclic import,
// scheduler не должен знать о DI service).
type Pipeline interface {
	TryStart(ctx context.Context, trigger string, requester *string, parentRunID *uuid.UUID) (*models.EtlRun, error)
}

// SkipMetrics — узкий интерфейс для prometheus skip-counter (Phase 16).
type SkipMetrics interface {
	IncSkipped(reason string)
}

// NoopSkipMetrics — no-op реализация.
type NoopSkipMetrics struct{}

// IncSkipped — no-op.
func (NoopSkipMetrics) IncSkipped(_ string) {}

// Config — параметры Scheduler.
type Config struct {
	CronExpr string // например, "30 2 * * *"
	Timezone string // например, "Europe/Kyiv"
}

// Scheduler — обёртка над gocron.Scheduler для ETL.
type Scheduler struct {
	s        gocron.Scheduler
	pipeline Pipeline
	maint    PartitionMaintainer
	metrics  SkipMetrics
	logger   *slog.Logger
	cfg      Config
}

// New собирает Scheduler.
func New(pipeline Pipeline, maint PartitionMaintainer, metrics SkipMetrics, log *slog.Logger, cfg Config) (*Scheduler, error) {
	if pipeline == nil {
		return nil, errors.New("scheduler: pipeline is nil")
	}
	if maint == nil {
		maint = NoopPartitionMaintainer{}
	}
	if metrics == nil {
		metrics = NoopSkipMetrics{}
	}
	if log == nil {
		log = slog.Default()
	}
	loc := time.UTC
	if cfg.Timezone != "" {
		l, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, fmt.Errorf("scheduler: bad timezone %q: %w", cfg.Timezone, err)
		}
		loc = l
	}
	gs, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("scheduler: gocron new: %w", err)
	}
	return &Scheduler{
		s: gs, pipeline: pipeline, maint: maint, metrics: metrics, logger: log, cfg: cfg,
	}, nil
}

// Start регистрирует cron-job и стартует scheduler.
//
// ctx используется для job-callback-ов (но gocron создаёт собственный per-tick
// context — здесь мы передаём только initial context).
func (sc *Scheduler) Start(_ context.Context) error {
	if sc.cfg.CronExpr == "" {
		return errors.New("scheduler: cron expr is empty")
	}
	_, err := sc.s.NewJob(
		gocron.CronJob(sc.cfg.CronExpr, false),
		gocron.NewTask(sc.tick),
		gocron.WithName("etl-cron-tick"),
	)
	if err != nil {
		return fmt.Errorf("scheduler: register job: %w", err)
	}
	sc.s.Start()
	sc.logger.Info("scheduler: started", "cron", sc.cfg.CronExpr, "tz", sc.cfg.Timezone)
	return nil
}

// Stop корректно завершает scheduler.
func (sc *Scheduler) Stop(_ context.Context) error {
	if err := sc.s.Shutdown(); err != nil {
		return fmt.Errorf("scheduler: shutdown: %w", err)
	}
	sc.logger.Info("scheduler: stopped")
	return nil
}

// tick — основной callback, вызываемый gocron.
func (sc *Scheduler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	if err := sc.maint.EnsureNextMonth(ctx); err != nil {
		sc.logger.Error("scheduler: partition maintenance failed", "err", err)
		// не блокируем pipeline — partition errors fail-soft.
	}

	_, err := sc.pipeline.TryStart(ctx, constants.TriggerCron, nil, nil)
	if err == nil {
		sc.logger.Info("scheduler: tick started run")
		return
	}
	switch {
	case errors.Is(err, errorspkg.ErrEtlRunAlreadyRunning):
		sc.logger.Warn("scheduler: tick skipped — already running")
		sc.metrics.IncSkipped("already_running")
	case errors.Is(err, errorspkg.ErrSnapshotNotReady):
		sc.logger.Warn("scheduler: tick skipped — snapshot not ready")
		sc.metrics.IncSkipped("snapshot_not_ready")
	default:
		sc.logger.Error("scheduler: tick failed", "err", err)
		sc.metrics.IncSkipped("error")
	}
}
