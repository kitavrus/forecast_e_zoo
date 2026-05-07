package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/loader"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
)

// Config — параметры scheduler-а.
type Config struct {
	CronExpr      string        // e.g. "0 3 * * *"  (3am daily)
	TZ            string        // e.g. "Europe/Kyiv"
	StaleAfter    time.Duration // running > этого → aborted
	MonthsAhead   int           // сколько месяцев партиций создавать вперёд
	Source        string        // "erp_e_zoo" | "manual" | "retry"
}

// Scheduler — обёртка над gocron + Loader.
type Scheduler struct {
	cron   gocron.Scheduler
	pool   *pgxpool.Pool
	loader *loader.Loader
	repo   *repository.Repository
	cfg    Config
	logger *slog.Logger

	job gocron.Job
}

// New создаёт scheduler.
func New(cfg Config, ldr *loader.Loader, repo *repository.Repository, pool *pgxpool.Pool, logger *slog.Logger) (*Scheduler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MonthsAhead == 0 {
		cfg.MonthsAhead = 2
	}
	if cfg.StaleAfter == 0 {
		cfg.StaleAfter = time.Hour
	}
	if cfg.Source == "" {
		cfg.Source = "erp_e_zoo"
	}
	loc := time.UTC
	if cfg.TZ != "" {
		l, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return nil, fmt.Errorf("scheduler: invalid TZ %q: %w", cfg.TZ, err)
		}
		loc = l
	}
	cron, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("scheduler: gocron init: %w", err)
	}
	return &Scheduler{
		cron:   cron,
		pool:   pool,
		loader: ldr,
		repo:   repo,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Start регистрирует job и запускает scheduler.
func (s *Scheduler) Start(_ context.Context) error {
	if s.cfg.CronExpr == "" {
		return errors.New("scheduler: empty CronExpr")
	}
	job, err := s.cron.NewJob(
		gocron.CronJob(s.cfg.CronExpr, false),
		gocron.NewTask(s.runTick),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("scheduler: register job: %w", err)
	}
	s.job = job
	s.cron.Start()
	s.logger.Info("scheduler.started", slog.String("cron", s.cfg.CronExpr), slog.String("tz", s.cfg.TZ))
	return nil
}

// Stop останавливает scheduler.
func (s *Scheduler) Stop(_ context.Context) error {
	if s.cron == nil {
		return nil
	}
	if err := s.cron.Shutdown(); err != nil {
		return fmt.Errorf("scheduler: shutdown: %w", err)
	}
	return nil
}

// runTick — обёртка для gocron, без context из-за сигнатуры NewTask.
func (s *Scheduler) runTick() {
	ctx := context.Background()
	if err := s.Tick(ctx); err != nil {
		s.logger.Error("scheduler.tick_error", slog.Any("error", err))
	}
}

// Tick — публичный метод (для tests / TriggerOnce).
func (s *Scheduler) Tick(ctx context.Context) error {
	// 1. Захватываем session-scoped advisory lock на отдельном connection,
	//    держим его до конца load-а.
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("scheduler: acquire conn: %w", err)
	}
	defer conn.Release()

	key := LockKey(LockTagDailyLoad)
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
		return fmt.Errorf("scheduler: try_advisory_lock: %w", err)
	}
	if !locked {
		s.logger.InfoContext(ctx, "scheduler.tick_skipped_lock_busy")
		return nil
	}
	defer func() {
		// Всегда отпускаем session lock на выходе.
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	}()

	// 2. Pre-step: партиции.
	if err := EnsureNextPartitions(ctx, s.pool, time.Now(), s.cfg.MonthsAhead); err != nil {
		return fmt.Errorf("scheduler: ensure partitions: %w", err)
	}

	// 3. Reaper стейл-загрузок.
	if n, err := s.repo.MarkAborted(ctx, s.cfg.StaleAfter); err != nil {
		s.logger.WarnContext(ctx, "scheduler.mark_aborted_failed", slog.Any("error", err))
	} else if n > 0 {
		s.logger.InfoContext(ctx, "scheduler.aborted_stale_loads", slog.Int64("count", n))
	}

	// 4. Сам load.
	loadID, err := s.loader.Run(ctx, s.cfg.Source)
	if err != nil {
		s.logger.ErrorContext(ctx, "scheduler.load_failed",
			slog.String("load_id", loadID.String()),
			slog.Any("error", err))
		return err
	}
	s.logger.InfoContext(ctx, "scheduler.load_completed", slog.String("load_id", loadID.String()))
	return nil
}

// TriggerOnce — для admin-handler-а POST /admin/loads. Запускает tick немедленно.
func (s *Scheduler) TriggerOnce(ctx context.Context) error {
	return s.Tick(ctx)
}

// _ keep imports stable.
var _ pgx.Tx = nil
