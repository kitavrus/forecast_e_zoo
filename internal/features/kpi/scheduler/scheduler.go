// Package scheduler — gocron-based периодический запуск KPI engine с
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

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/engine"
)

// Config — параметры scheduler-а.
type Config struct {
	CronExpr string        // e.g. "0 4 * * *"
	TZ       string        // e.g. "Europe/Kyiv"
	JobName  string        // optional, для логов
	Timeout  time.Duration // hard timeout одного run-а
}

// Scheduler — обёртка над gocron + Engine + advisory lock.
type Scheduler struct {
	cron   gocron.Scheduler
	pool   *pgxpool.Pool
	engine *engine.Engine
	cfg    Config
	logger *slog.Logger
	job    gocron.Job

	// LockBusyMetric — увеличивается при skip из-за busy lock.
	LockBusyMetric func()
	// TickMetric — увеличивается на каждый tick (label result=ok|error|skipped).
	TickMetric func(result string)
}

// New создаёт scheduler.
func New(cfg Config, eng *engine.Engine, pool *pgxpool.Pool, logger *slog.Logger) (*Scheduler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute //nolint:mnd // дефолт hard-cap
	}
	loc := time.UTC
	if cfg.TZ != "" {
		l, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return nil, fmt.Errorf("kpi scheduler: invalid TZ %q: %w", cfg.TZ, err)
		}
		loc = l
	}
	cron, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("kpi scheduler: gocron init: %w", err)
	}
	return &Scheduler{
		cron:   cron,
		pool:   pool,
		engine: eng,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Start регистрирует job и запускает.
func (s *Scheduler) Start(_ context.Context) error {
	if s.cfg.CronExpr == "" {
		return errors.New("kpi scheduler: empty cron expression")
	}
	job, err := s.cron.NewJob(
		gocron.CronJob(s.cfg.CronExpr, false),
		gocron.NewTask(func() { _ = s.tick(context.Background()) }),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("kpi scheduler: register job: %w", err)
	}
	s.job = job
	s.cron.Start()
	s.logger.Info("kpi scheduler started",
		slog.String("cron", s.cfg.CronExpr),
		slog.String("tz", s.cfg.TZ),
	)
	return nil
}

// Stop graceful-ный останов scheduler-а.
func (s *Scheduler) Stop() error {
	if err := s.cron.Shutdown(); err != nil {
		return fmt.Errorf("kpi scheduler: shutdown: %w", err)
	}
	return nil
}

// tick — один проход engine с advisory-lock. Безопасно вызывать ad-hoc.
//
// Контракт:
//   - Если lock busy → возвращает nil (не ошибка), метрики IncRun("skipped").
//   - Если lock получен → запускает engine.Run; ошибка пробрасывается.
func (s *Scheduler) tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("kpi scheduler: acquire conn: %w", err)
	}
	defer conn.Release()

	var acquired bool
	err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired)
	if err != nil {
		return fmt.Errorf("kpi scheduler: try_advisory_lock: %w", err)
	}
	if !acquired {
		s.logger.InfoContext(ctx, "kpi scheduler: lock busy, skip tick")
		if s.LockBusyMetric != nil {
			s.LockBusyMetric()
		}
		if s.TickMetric != nil {
			s.TickMetric("skipped")
		}
		return nil
	}
	defer func() {
		if _, unlockErr := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", constants.AdvisoryLockKey); unlockErr != nil {
			s.logger.Error("kpi scheduler: advisory_unlock failed", slog.Any("error", unlockErr))
		}
	}()

	asOf := time.Now().In(time.UTC).Truncate(24 * time.Hour) //nolint:mnd // 1 day truncation
	runID := uuid.New()
	s.logger.InfoContext(ctx, "kpi scheduler: tick start",
		slog.String("run_id", runID.String()),
		slog.Time("as_of_date", asOf),
	)

	_, runErr := s.engine.Run(ctx, engine.RunInput{
		RunID:    runID,
		AsOfDate: asOf,
	})
	if runErr != nil {
		s.logger.ErrorContext(ctx, "kpi scheduler: run failed",
			slog.String("run_id", runID.String()),
			slog.Any("error", runErr),
		)
		if s.TickMetric != nil {
			s.TickMetric("error")
		}
		return fmt.Errorf("kpi scheduler: engine run: %w", runErr)
	}
	if s.TickMetric != nil {
		s.TickMetric("ok")
	}
	return nil
}

// TryTrigger — public API для admin handler-а POST /v1/kpi/snapshots/refresh.
//
// Контракт:
//   - Синхронно пытается захватить advisory lock.
//   - Если lock busy → возвращает (false, nil), engine не запущен.
//   - Если lock получен → запускает engine В ФОНЕ на отдельной goroutine,
//     возвращает (true, runID) сразу. Лок держится до конца engine.Run.
func (s *Scheduler) TryTrigger(
	ctx context.Context, asOfDate time.Time, kpiNames []string,
) (uuid.UUID, bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("kpi scheduler: acquire: %w", err)
	}
	var acquired bool
	if scanErr := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired); scanErr != nil {
		conn.Release()
		return uuid.Nil, false, fmt.Errorf("kpi scheduler: try_advisory_lock: %w", scanErr)
	}
	if !acquired {
		conn.Release()
		if s.LockBusyMetric != nil {
			s.LockBusyMetric()
		}
		return uuid.Nil, false, nil
	}

	runID := uuid.New()
	go func() { //nolint:gosec // bgCtx намеренный — request ctx закроется сразу
		bgCtx := context.Background()
		defer func() {
			if _, unlockErr := conn.Exec(bgCtx, "SELECT pg_advisory_unlock($1)", constants.AdvisoryLockKey); unlockErr != nil {
				s.logger.Error("kpi scheduler: trigger unlock failed", slog.Any("error", unlockErr))
			}
			conn.Release()
		}()
		if _, runErr := s.engine.Run(bgCtx, engine.RunInput{
			RunID:    runID,
			AsOfDate: asOfDate,
			KpiNames: kpiNames,
		}); runErr != nil {
			s.logger.Error("kpi scheduler: triggered run failed",
				slog.String("run_id", runID.String()),
				slog.Any("error", runErr),
			)
		}
	}()

	return runID, true, nil
}
