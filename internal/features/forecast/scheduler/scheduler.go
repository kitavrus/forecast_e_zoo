// Package scheduler — gocron-based периодический запуск forecast engine с
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

	"github.com/Kitavrus/e_zoo/internal/features/forecast/constants"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/engine"
	forecastModels "github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// EngineRepo — узкий интерфейс repository для синхронной вставки forecast_run
// в TryTrigger. Объявлен здесь, чтобы избежать циклического импорта engine.
type EngineRepo interface {
	InsertRun(ctx context.Context, in forecastModels.InsertRunInput) (forecastModels.ForecastRun, error)
}

// Config — параметры scheduler-а.
type Config struct {
	CronExpr    string
	TZ          string
	HorizonDays int
	Timeout     time.Duration
}

// Scheduler — обёртка над gocron + Engine + advisory lock.
type Scheduler struct {
	cron       gocron.Scheduler
	pool       *pgxpool.Pool
	engine     *engine.Engine
	engineRepo EngineRepo
	cfg        Config
	logger     *slog.Logger
	job        gocron.Job

	LockBusyMetric func()
	TickMetric     func(result string)
}

// New создаёт scheduler.
//
// engineRepo нужен для синхронной вставки forecast_run в TryTrigger; если nil,
// TryTrigger откатится на старое поведение (uuid.New() — небезопасно для
// API-poll'еров, см. ADR-027).
func New(cfg Config, eng *engine.Engine, engineRepo EngineRepo, pool *pgxpool.Pool, logger *slog.Logger) (*Scheduler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute //nolint:mnd // hard-cap по умолчанию
	}
	if cfg.HorizonDays <= 0 {
		cfg.HorizonDays = constants.HorizonDefault
	}
	loc := time.UTC
	if cfg.TZ != "" {
		l, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return nil, fmt.Errorf("forecast scheduler: invalid TZ %q: %w", cfg.TZ, err)
		}
		loc = l
	}
	cron, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("forecast scheduler: gocron init: %w", err)
	}
	return &Scheduler{
		cron:       cron,
		pool:       pool,
		engine:     eng,
		engineRepo: engineRepo,
		cfg:        cfg,
		logger:     logger,
	}, nil
}

// Start регистрирует job и запускает.
func (s *Scheduler) Start(_ context.Context) error {
	if s.cfg.CronExpr == "" {
		return errors.New("forecast scheduler: empty cron expression")
	}
	job, err := s.cron.NewJob(
		gocron.CronJob(s.cfg.CronExpr, false),
		gocron.NewTask(func() { _ = s.tick(context.Background()) }),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("forecast scheduler: register job: %w", err)
	}
	s.job = job
	s.cron.Start()
	s.logger.Info("forecast scheduler started",
		slog.String("cron", s.cfg.CronExpr),
		slog.String("tz", s.cfg.TZ),
	)
	return nil
}

// Stop graceful-ный останов.
func (s *Scheduler) Stop() error {
	if err := s.cron.Shutdown(); err != nil {
		return fmt.Errorf("forecast scheduler: shutdown: %w", err)
	}
	return nil
}

// tick — один проход engine с advisory-lock.
func (s *Scheduler) tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("forecast scheduler: acquire conn: %w", err)
	}
	defer conn.Release()

	var acquired bool
	if scanErr := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired); scanErr != nil {
		return fmt.Errorf("forecast scheduler: try_advisory_lock: %w", scanErr)
	}
	if !acquired {
		s.logger.InfoContext(ctx, "forecast scheduler: lock busy, skip tick")
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
			s.logger.Error("forecast scheduler: advisory_unlock failed", slog.Any("error", unlockErr))
		}
	}()

	asOf := time.Now().In(time.UTC).Truncate(24 * time.Hour) //nolint:mnd
	s.logger.InfoContext(ctx, "forecast scheduler: tick start",
		slog.Time("as_of", asOf),
		slog.Int("horizon_days", s.cfg.HorizonDays),
	)
	res, runErr := s.engine.Run(ctx, engine.RunInput{
		AsOf:        asOf,
		HorizonDays: s.cfg.HorizonDays,
	})
	if runErr != nil {
		s.logger.ErrorContext(ctx, "forecast scheduler: run failed",
			slog.Any("error", runErr),
		)
		if s.TickMetric != nil {
			s.TickMetric("error")
		}
		return fmt.Errorf("forecast scheduler: engine run: %w", runErr)
	}
	if s.TickMetric != nil {
		s.TickMetric("ok")
	}
	s.logger.InfoContext(ctx, "forecast scheduler: tick committed",
		slog.String("run_id", res.RunID.String()),
		slog.Int("forecasts", res.Forecasts),
		slog.Int("lines", res.Lines),
		slog.Int("plans", res.Plans),
	)
	return nil
}

// TryTrigger — public API для admin handler-а POST /v1/forecast/runs/refresh.
//
// Контракт:
//   - Lock busy → (uuid.Nil, false, nil), engine не запускается.
//   - Lock получен → engine.Run В ФОНЕ, возвращает (runID, true, nil) сразу.
func (s *Scheduler) TryTrigger(
	ctx context.Context, horizonDays int,
) (uuid.UUID, bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("forecast scheduler: acquire: %w", err)
	}
	var acquired bool
	if scanErr := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", constants.AdvisoryLockKey).Scan(&acquired); scanErr != nil {
		conn.Release()
		return uuid.Nil, false, fmt.Errorf("forecast scheduler: try_advisory_lock: %w", scanErr)
	}
	if !acquired {
		conn.Release()
		if s.LockBusyMetric != nil {
			s.LockBusyMetric()
		}
		return uuid.Nil, false, nil
	}

	if horizonDays <= 0 {
		horizonDays = s.cfg.HorizonDays
	}
	// Вставляем forecast_run row синхронно, чтобы вернуть клиенту тот же
	// run_id, что персистится в БД (раньше TryTrigger возвращал uuid.New(),
	// а engine генерил свой ID — приводило к 404 на poll API).
	var runID uuid.UUID
	if s.engineRepo != nil {
		run, insertErr := s.engineRepo.InsertRun(ctx, forecastModels.InsertRunInput{
			HorizonDays: horizonDays,
		})
		if insertErr != nil {
			if _, unlockErr := conn.Exec(ctx,
				"SELECT pg_advisory_unlock($1)", constants.AdvisoryLockKey); unlockErr != nil {
				s.logger.Error("forecast scheduler: insert-run unlock failed", slog.Any("error", unlockErr))
			}
			conn.Release()
			return uuid.Nil, false, fmt.Errorf("forecast scheduler: insert run: %w", insertErr)
		}
		runID = run.ID
	} else {
		// Fallback (tests без репо) — uuid.New() как раньше; API-poll'ер
		// получит 404, что и было до фикса.
		runID = uuid.New()
	}
	go func() { //nolint:gosec // bgCtx намеренный
		bgCtx := context.Background()
		defer func() {
			if _, unlockErr := conn.Exec(bgCtx,
				"SELECT pg_advisory_unlock($1)", constants.AdvisoryLockKey); unlockErr != nil {
				s.logger.Error("forecast scheduler: trigger unlock failed",
					slog.Any("error", unlockErr))
			}
			conn.Release()
		}()
		asOf := time.Now().In(time.UTC).Truncate(24 * time.Hour) //nolint:mnd
		if _, runErr := s.engine.Run(bgCtx, engine.RunInput{
			AsOf:        asOf,
			HorizonDays: horizonDays,
			PresetRunID: runID,
		}); runErr != nil {
			s.logger.Error("forecast scheduler: triggered run failed",
				slog.Any("error", runErr),
			)
		}
	}()
	return runID, true, nil
}
