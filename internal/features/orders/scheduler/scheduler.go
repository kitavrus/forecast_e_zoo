// Package scheduler — gocron-based периодический запуск order builder с
// pg_advisory_lock для идемпотентности (нельзя запустить два tick одновременно).
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

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
)

// Builder — узкий интерфейс, который реализует service.Service (метод BuildAll).
type Builder interface {
	BuildAll(ctx context.Context, maxPlans int) (BuildResult, error)
}

// BuildResult — мини-DTO, чтобы scheduler не зависел от полного models.BuildResult.
//
// Заполняется адаптером (см. NewServiceAdapter).
type BuildResult struct {
	RunID          uuid.UUID
	PlansProcessed int
	POsCreated     int
}

// Config — параметры scheduler-а.
type Config struct {
	CronExpr     string
	TZ           string
	Timeout      time.Duration
	MaxPlans     int
}

// Scheduler — обёртка над gocron + Builder + advisory lock.
type Scheduler struct {
	cron    gocron.Scheduler
	pool    *pgxpool.Pool
	builder Builder
	cfg     Config
	logger  *slog.Logger
	job     gocron.Job

	LockBusyMetric func()
	TickMetric     func(result string)
}

// New создаёт scheduler.
func New(cfg Config, b Builder, pool *pgxpool.Pool, logger *slog.Logger) (*Scheduler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute //nolint:mnd
	}
	if cfg.MaxPlans == 0 {
		cfg.MaxPlans = constants.MaxPlansPerBuildBatch
	}
	loc := time.UTC
	if cfg.TZ != "" {
		l, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return nil, fmt.Errorf("orders scheduler: invalid TZ %q: %w", cfg.TZ, err)
		}
		loc = l
	}
	cron, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("orders scheduler: gocron init: %w", err)
	}
	return &Scheduler{
		cron:    cron,
		pool:    pool,
		builder: b,
		cfg:     cfg,
		logger:  logger,
	}, nil
}

// Start регистрирует job и запускает.
func (s *Scheduler) Start(_ context.Context) error {
	if s.cfg.CronExpr == "" {
		return errors.New("orders scheduler: empty cron expression")
	}
	job, err := s.cron.NewJob(
		gocron.CronJob(s.cfg.CronExpr, false),
		gocron.NewTask(func() { _ = s.tick(context.Background()) }),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("orders scheduler: register job: %w", err)
	}
	s.job = job
	s.cron.Start()
	s.logger.Info("orders scheduler started",
		slog.String("cron", s.cfg.CronExpr),
		slog.String("tz", s.cfg.TZ),
	)
	return nil
}

// Stop graceful-ный останов.
func (s *Scheduler) Stop() error {
	if err := s.cron.Shutdown(); err != nil {
		return fmt.Errorf("orders scheduler: shutdown: %w", err)
	}
	return nil
}

// tick — один проход builder с advisory-lock.
func (s *Scheduler) tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("orders scheduler: acquire conn: %w", err)
	}
	defer conn.Release()

	var acquired bool
	if scanErr := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired); scanErr != nil {
		return fmt.Errorf("orders scheduler: try_advisory_lock: %w", scanErr)
	}
	if !acquired {
		s.logger.InfoContext(ctx, "orders scheduler: lock busy, skip tick")
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
			s.logger.Error("orders scheduler: advisory_unlock failed", slog.Any("error", unlockErr))
		}
	}()

	res, runErr := s.builder.BuildAll(ctx, s.cfg.MaxPlans)
	if runErr != nil {
		s.logger.ErrorContext(ctx, "orders scheduler: build failed",
			slog.Any("error", runErr),
		)
		if s.TickMetric != nil {
			s.TickMetric("error")
		}
		return fmt.Errorf("orders scheduler: build: %w", runErr)
	}
	if s.TickMetric != nil {
		s.TickMetric("ok")
	}
	s.logger.InfoContext(ctx, "orders scheduler: tick committed",
		slog.String("run_id", res.RunID.String()),
		slog.Int("plans", res.PlansProcessed),
		slog.Int("pos_created", res.POsCreated),
	)
	return nil
}

// TryTrigger — public API для admin handler-а POST /v1/orders/purchase-orders/build.
//
// Контракт:
//   - Lock busy → (uuid.Nil, false, nil), builder не запускается.
//   - Lock получен → builder.BuildAll В ФОНЕ, возвращает (runID, true, nil) сразу.
func (s *Scheduler) TryTrigger(
	ctx context.Context, maxPlans int,
) (uuid.UUID, bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("orders scheduler: acquire: %w", err)
	}
	var acquired bool
	if scanErr := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired); scanErr != nil {
		conn.Release()
		return uuid.Nil, false, fmt.Errorf("orders scheduler: try_advisory_lock: %w", scanErr)
	}
	if !acquired {
		conn.Release()
		if s.LockBusyMetric != nil {
			s.LockBusyMetric()
		}
		return uuid.Nil, false, nil
	}

	if maxPlans <= 0 {
		maxPlans = s.cfg.MaxPlans
	}
	runID := uuid.New()
	go func() { //nolint:gosec // bgCtx намеренный (не наследуем cancel от HTTP-запроса)
		bgCtx := context.Background()
		defer func() {
			if _, unlockErr := conn.Exec(bgCtx,
				"SELECT pg_advisory_unlock($1)", constants.AdvisoryLockKey); unlockErr != nil {
				s.logger.Error("orders scheduler: trigger unlock failed",
					slog.Any("error", unlockErr))
			}
			conn.Release()
		}()
		if _, runErr := s.builder.BuildAll(bgCtx, maxPlans); runErr != nil {
			s.logger.Error("orders scheduler: triggered run failed",
				slog.Any("error", runErr),
			)
		}
	}()
	return runID, true, nil
}

// ServiceAdapter — адаптер service.Service → Builder (без циклических зависимостей).
type ServiceAdapter struct {
	BuildAllFn func(ctx context.Context, maxPlans int) (uuid.UUID, int, int, error)
}

// BuildAll реализует Builder.
func (a ServiceAdapter) BuildAll(ctx context.Context, maxPlans int) (BuildResult, error) {
	if a.BuildAllFn == nil {
		return BuildResult{}, errors.New("orders scheduler: ServiceAdapter not configured")
	}
	id, plans, pos, err := a.BuildAllFn(ctx, maxPlans)
	if err != nil {
		return BuildResult{RunID: id, PlansProcessed: plans, POsCreated: pos}, err
	}
	return BuildResult{RunID: id, PlansProcessed: plans, POsCreated: pos}, nil
}
