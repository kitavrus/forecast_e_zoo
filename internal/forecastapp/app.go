// Package forecastapp собирает компоненты сервиса forecast (Модуль 5) воедино.
package forecastapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	forecastappconfig "github.com/Kitavrus/e_zoo/internal/forecastapp/config"
	forecastEngine "github.com/Kitavrus/e_zoo/internal/features/forecast/engine"
	forecastHandler "github.com/Kitavrus/e_zoo/internal/features/forecast/handler"
	forecastRepo "github.com/Kitavrus/e_zoo/internal/features/forecast/repository"
	forecastRouter "github.com/Kitavrus/e_zoo/internal/features/forecast/router"
	forecastScheduler "github.com/Kitavrus/e_zoo/internal/features/forecast/scheduler"
	forecastService "github.com/Kitavrus/e_zoo/internal/features/forecast/service"
	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/internal/observability"
)

const shutdownTimeout = 30 * time.Second

// App — корневая структура forecast сервиса.
type App struct {
	cfg       *forecastappconfig.Config
	log       *slog.Logger
	fiber     *fiber.App
	pool      *pgxpool.Pool
	scheduler *forecastScheduler.Scheduler
}

// New собирает граф зависимостей.
func New(ctx context.Context, cfg *forecastappconfig.Config, log *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, errors.New("forecastapp: config is nil")
	}
	if log == nil {
		return nil, errors.New("forecastapp: logger is nil")
	}

	pool, err := pgxpool.New(ctx, cfg.DBDsn)
	if err != nil {
		return nil, fmt.Errorf("forecastapp: pgxpool: %w", err)
	}

	repo := forecastRepo.New(pool)
	eng := forecastEngine.New(repo, nil, nil, nil, log,
		forecastEngine.NewPrometheusMetrics())
	sch, schErr := forecastScheduler.New(forecastScheduler.Config{
		CronExpr:    cfg.CronSchedule,
		TZ:          cfg.CronTZ,
		HorizonDays: cfg.HorizonDays,
	}, eng, repo, pool, log)
	if schErr != nil {
		log.Warn("forecastapp: scheduler init failed (continue without scheduler)", "error", schErr)
		sch = nil
	}
	var trigger forecastService.Trigger
	if sch != nil {
		trigger = sch
	}
	svc := forecastService.New(repo, trigger)
	h := forecastHandler.NewHandler(svc)

	metricsReg := observability.Init()

	f := fiber.New(fiber.Config{
		AppName:      "forecast",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		BodyLimit:    10 * 1024 * 1024,
	})
	f.Use(middleware.RequestID())
	f.Use(observability.HTTPMetricsMiddleware())
	f.Use(observability.AccessLogMiddleware(log))

	f.Get("/healthz", func(c fiber.Ctx) error {
		if err := pool.Ping(c.Context()); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "down",
				"err":    err.Error(),
			})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})
	f.Get("/metrics", observability.Handler(metricsReg))

	jwtCfg := middleware.JWTConfig{
		Alg:           cfg.JWTAlg,
		Secret:        cfg.JWTSecret,
		PublicKeyPath: cfg.JWTPublicKeyPath,
	}
	forecastRouter.Register(f, forecastRouter.Deps{
		JWTConfig: jwtCfg,
		Handler:   h,
	})

	return &App{
		cfg:       cfg,
		log:       log,
		fiber:     f,
		pool:      pool,
		scheduler: sch,
	}, nil
}

// Run — стартует scheduler и Fiber listener; блокируется до ctx.Done.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	if a.scheduler != nil {
		if err := a.scheduler.Start(ctx); err != nil {
			a.log.Warn("forecastapp: scheduler start failed", "error", err)
		}
	}

	go func() {
		a.log.Info("http server starting", "addr", a.cfg.HTTPAddr)
		listenCfg := fiber.ListenConfig{DisableStartupMessage: a.cfg.Env != "dev"}
		if err := a.fiber.Listen(a.cfg.HTTPAddr, listenCfg); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		a.log.Info("shutdown signal received")
		return a.Shutdown()
	case err := <-errCh:
		if err != nil {
			a.log.Error("http server failed", "error", err)
			return fmt.Errorf("forecastapp: %w", err)
		}
		return nil
	}
}

// Shutdown — graceful.
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if a.scheduler != nil {
		if err := a.scheduler.Stop(); err != nil {
			a.log.Warn("forecast scheduler stop error", "error", err)
		}
	}
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.log.Error("fiber shutdown error", "error", err)
		return fmt.Errorf("forecastapp: %w", err)
	}
	if a.pool != nil {
		a.pool.Close()
	}
	a.log.Info("shutdown complete")
	return nil
}
