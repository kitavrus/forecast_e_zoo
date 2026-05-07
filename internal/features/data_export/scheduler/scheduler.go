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
	"github.com/Kitavrus/e_zoo/internal/observability"
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
	// background-задача после ответа: фоновый ctx намеренный
	// (gocron.NewTask не принимает ctx; вынужденная мера, см. также §Серьёзные замечания #1).
	ctx := context.Background()
	// Метрика SchedulerTickTotal инкрементируется внутри Tick (ok/skipped/error),
	// чтобы корректно различить путь lock-busy (Tick возвращает nil, но это не "ok").
	if err := s.Tick(ctx); err != nil {
		s.logger.Error("scheduler.tick_error", slog.Any("error", err))
	}
}

// Tick — публичный метод (для tests / TriggerOnce).
//
// Метрика SchedulerTickTotal:
//   - "skipped" — advisory lock занят (другой tick уже исполняется);
//   - "error"   — любая ошибка ниже advisory-lock checkpoint;
//   - "ok"      — load завершён без ошибок (включая lock-busy: tick формально завершился).
//
// Принцип: метрику считает Tick, а не runTick, чтобы lock-busy путь
// корректно различался от "ok" (Tick возвращает nil в обоих кейсах).
func (s *Scheduler) Tick(ctx context.Context) (retErr error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		observability.SchedulerTickTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("scheduler: acquire conn: %w", err)
	}
	defer conn.Release()

	key := LockKey(LockTagDailyLoad)
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
		observability.SchedulerTickTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("scheduler: try_advisory_lock: %w", err)
	}
	if !locked {
		observability.AdvisoryLockBusyTotal.Inc()
		observability.SchedulerTickTotal.WithLabelValues("skipped").Inc()
		s.logger.InfoContext(ctx, "scheduler.tick_skipped_lock_busy")
		return nil
	}
	defer func() {
		// Всегда отпускаем session lock на выходе.
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
		// И только теперь — финальная классификация результата tick-а.
		if retErr == nil {
			observability.SchedulerTickTotal.WithLabelValues("ok").Inc()
		} else {
			observability.SchedulerTickTotal.WithLabelValues("error").Inc()
		}
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

// TriggerOnce — синхронный запуск Tick (для retry / тестов).
// Блокируется на время load-а. Если advisory lock занят — Tick вернёт nil,
// фактически пропустив запуск (см. метрику scheduler.tick_skipped_lock_busy).
func (s *Scheduler) TriggerOnce(ctx context.Context) error {
	return s.Tick(ctx)
}

// TryTrigger — для admin-handler-а POST /admin/loads (Issue #6 validation 2026-05-07).
//
// Контракт:
//   - Синхронно пытается захватить pg_try_advisory_lock(LockTagDailyLoad).
//   - Если lock занят → возвращает (false, nil); load НЕ запущен.
//     Хендлер обязан отдать 409 ErrLoadAlreadyRunning.
//   - Если lock получен → стартует tick (партиции, reaper, load) В ФОНЕ
//     на отдельной goroutine с background-ctx, лок держится до конца load-а,
//     возвращает (true, nil) сразу.
//
// Это критично для детерминированного 409: handler не ждёт завершения load-а
// (тот может занять минуты), но и параллельный POST увидит lock busy и тоже
// получит 409, а не «ложный» 202.
//
// Реализация держит pgxpool-connection до конца load-а — это допустимо,
// потому что одновременно работает максимум один tick (по контракту lock-а).
func (s *Scheduler) TryTrigger(ctx context.Context) (bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("scheduler: try_trigger acquire: %w", err)
	}

	key := LockKey(LockTagDailyLoad)
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
		conn.Release()
		return false, fmt.Errorf("scheduler: try_advisory_lock: %w", err)
	}
	if !locked {
		// Lock занят — возвращаем conn в пул, не запускаем tick.
		observability.AdvisoryLockBusyTotal.Inc()
		s.logger.InfoContext(ctx, "scheduler.try_trigger_lock_busy")
		conn.Release()
		return false, nil
	}

	// Lock получен. Запускаем оставшуюся часть Tick в фоновой goroutine
	// и сразу возвращаем acquired=true. Лок держится до конца load-а.
	go func() { //nolint:gosec // G118: bgCtx намеренный — request ctx будет cancelled сразу после ответа handler-а и убил бы load
		// background-задача после ответа: ctx запроса cancellится сразу
		// после возврата хендлера и убил бы load.
		bgCtx := context.Background()
		defer func() {
			_, _ = conn.Exec(bgCtx, "SELECT pg_advisory_unlock($1)", key)
			conn.Release()
		}()

		if err := s.tickLocked(bgCtx); err != nil {
			s.logger.Error("scheduler.try_trigger_tick_failed", slog.Any("error", err))
			observability.SchedulerTickTotal.WithLabelValues("error").Inc()
			return
		}
		observability.SchedulerTickTotal.WithLabelValues("ok").Inc()
	}()
	return true, nil
}

// tickLocked — выделенная часть Tick БЕЗ повторного захвата advisory lock.
// Вызывается из TryTrigger (lock уже захвачен). Параллелит реализацию Tick.
func (s *Scheduler) tickLocked(ctx context.Context) error {
	if err := EnsureNextPartitions(ctx, s.pool, time.Now(), s.cfg.MonthsAhead); err != nil {
		return fmt.Errorf("scheduler: ensure partitions: %w", err)
	}
	if n, err := s.repo.MarkAborted(ctx, s.cfg.StaleAfter); err != nil {
		s.logger.WarnContext(ctx, "scheduler.mark_aborted_failed", slog.Any("error", err))
	} else if n > 0 {
		s.logger.InfoContext(ctx, "scheduler.aborted_stale_loads", slog.Int64("count", n))
	}
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

// _ keep imports stable.
var _ pgx.Tx = nil
