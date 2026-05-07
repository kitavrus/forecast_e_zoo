// Package scheduler — gocron-based периодический запуск ChannelRouter с
// pg_advisory_lock для идемпотентности (нельзя запустить два tick одновременно).
//
// Зеркалит internal/features/orders/scheduler. Cron default: 06:30 (после order-builder 06:00).
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

// Router — узкий интерфейс ChannelRouter (DI seam).
type Router interface {
	SendAll(ctx context.Context, maxPOs int) (models.SendRunResult, error)
}

// Config — параметры scheduler.
type Config struct {
	CronExpr string
	TZ       string
	Timeout  time.Duration
	MaxPOs   int
}

// Scheduler — обёртка над gocron + Router + advisory lock.
type Scheduler struct {
	cron   gocron.Scheduler
	pool   *pgxpool.Pool
	router Router
	cfg    Config
	logger *slog.Logger
	job    gocron.Job

	LockBusyMetric func()
	TickMetric     func(result string)
}

// New создаёт scheduler.
func New(cfg Config, r Router, pool *pgxpool.Pool, logger *slog.Logger) (*Scheduler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute //nolint:mnd
	}
	if cfg.MaxPOs == 0 {
		cfg.MaxPOs = constants.MaxPosPerRunDefault
	}
	loc := time.UTC
	if cfg.TZ != "" {
		l, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return nil, fmt.Errorf("channels scheduler: invalid TZ %q: %w", cfg.TZ, err)
		}
		loc = l
	}
	cron, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("channels scheduler: gocron init: %w", err)
	}
	return &Scheduler{
		cron:   cron,
		pool:   pool,
		router: r,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Start регистрирует cron job и запускает.
func (s *Scheduler) Start(_ context.Context) error {
	if s.cfg.CronExpr == "" {
		return errors.New("channels scheduler: empty cron expression")
	}
	job, err := s.cron.NewJob(
		gocron.CronJob(s.cfg.CronExpr, false),
		gocron.NewTask(func() { _ = s.tick(context.Background()) }),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("channels scheduler: register job: %w", err)
	}
	s.job = job
	s.cron.Start()
	s.logger.Info("channels scheduler started",
		slog.String("cron", s.cfg.CronExpr),
		slog.String("tz", s.cfg.TZ),
	)
	return nil
}

// Stop graceful-ный останов.
func (s *Scheduler) Stop() error {
	if err := s.cron.Shutdown(); err != nil {
		return fmt.Errorf("channels scheduler: shutdown: %w", err)
	}
	return nil
}

// tick — один проход router с advisory-lock.
func (s *Scheduler) tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("channels scheduler: acquire conn: %w", err)
	}
	defer conn.Release()

	var acquired bool
	if scanErr := conn.QueryRow(ctx,
		"SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired); scanErr != nil {
		return fmt.Errorf("channels scheduler: try_advisory_lock: %w", scanErr)
	}
	if !acquired {
		s.logger.InfoContext(ctx, "channels scheduler: lock busy, skip tick")
		if s.LockBusyMetric != nil {
			s.LockBusyMetric()
		}
		if s.TickMetric != nil {
			s.TickMetric("skipped")
		}
		return nil
	}
	defer func() {
		if _, unlockErr := conn.Exec(context.Background(),
			"SELECT pg_advisory_unlock($1)", constants.AdvisoryLockKey); unlockErr != nil {
			s.logger.Error("channels scheduler: advisory_unlock failed", slog.Any("error", unlockErr))
		}
	}()

	res, runErr := s.router.SendAll(ctx, s.cfg.MaxPOs)
	if runErr != nil {
		s.logger.ErrorContext(ctx, "channels scheduler: run failed",
			slog.Any("error", runErr),
		)
		if s.TickMetric != nil {
			s.TickMetric("error")
		}
		return fmt.Errorf("channels scheduler: run: %w", runErr)
	}
	if s.TickMetric != nil {
		s.TickMetric("ok")
	}
	s.logger.InfoContext(ctx, "channels scheduler: tick committed",
		slog.String("run_id", res.RunID.String()),
		slog.Int("processed", res.POsProcessed),
		slog.Int("sent", res.POsSent),
		slog.Int("failed", res.POsFailed),
		slog.Int("skipped", res.POsSkipped),
	)
	return nil
}

// TryTrigger — public API для admin handler-а POST /v1/channels/send.
//
// Контракт зеркалит orders.scheduler.TryTrigger:
//   - Lock busy → (uuid.Nil, false, nil), router не запускается.
//   - Lock получен → router.SendAll В ФОНЕ, возвращает (runID, true, nil) сразу.
func (s *Scheduler) TryTrigger(ctx context.Context, maxPOs int) (uuid.UUID, bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("channels scheduler: acquire: %w", err)
	}
	var acquired bool
	if scanErr := conn.QueryRow(ctx,
		"SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired); scanErr != nil {
		conn.Release()
		return uuid.Nil, false, fmt.Errorf("channels scheduler: try_advisory_lock: %w", scanErr)
	}
	if !acquired {
		conn.Release()
		if s.LockBusyMetric != nil {
			s.LockBusyMetric()
		}
		return uuid.Nil, false, nil
	}

	if maxPOs <= 0 {
		maxPOs = s.cfg.MaxPOs
	}
	runID := uuid.New()
	go func() { //nolint:gosec // bgCtx намеренный (не наследуем cancel от HTTP-запроса)
		bgCtx := context.Background()
		defer func() {
			if _, unlockErr := conn.Exec(bgCtx,
				"SELECT pg_advisory_unlock($1)", constants.AdvisoryLockKey); unlockErr != nil {
				s.logger.Error("channels scheduler: trigger unlock failed",
					slog.Any("error", unlockErr))
			}
			conn.Release()
		}()
		if _, runErr := s.router.SendAll(bgCtx, maxPOs); runErr != nil {
			s.logger.Error("channels scheduler: triggered run failed",
				slog.Any("error", runErr),
			)
		}
	}()
	return runID, true, nil
}
