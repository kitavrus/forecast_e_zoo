// Package martsapp собирает компоненты сервиса data-marts (Модуль 3) воедино.
//
// data-marts — read-only API над marts.* (отдельный binary).
package martsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	martsconfig "github.com/Kitavrus/e_zoo/internal/martsapp/config"
	dataMartsHandler "github.com/Kitavrus/e_zoo/internal/features/data_marts/handler"
	dataMartsRepo "github.com/Kitavrus/e_zoo/internal/features/data_marts/repository"
	dataMartsRouter "github.com/Kitavrus/e_zoo/internal/features/data_marts/router"
	dataMartsService "github.com/Kitavrus/e_zoo/internal/features/data_marts/service"
	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/internal/observability"
)

const shutdownTimeout = 30 * time.Second

// App — корневая структура data-marts сервиса.
type App struct {
	cfg   *martsconfig.Config
	log   *slog.Logger
	fiber *fiber.App
	pool  *pgxpool.Pool
}

// New собирает граф зависимостей.
func New(ctx context.Context, cfg *martsconfig.Config, log *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, errors.New("martsapp: config is nil")
	}
	if log == nil {
		return nil, errors.New("martsapp: logger is nil")
	}

	pool, err := pgxpool.New(ctx, cfg.DBDsn)
	if err != nil {
		return nil, fmt.Errorf("martsapp: pgxpool: %w", err)
	}

	repo := dataMartsRepo.New(pool)
	reader := dataMartsService.NewPGReader(repo)
	svc := dataMartsService.New(reader)
	h := dataMartsHandler.NewHandler(svc)

	metricsReg := observability.Init()

	f := fiber.New(fiber.Config{
		AppName:      "data-marts",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		BodyLimit:    10 * 1024 * 1024,
	})
	f.Use(middleware.RequestID())
	f.Use(observability.HTTPMetricsMiddleware())
	f.Use(observability.AccessLogMiddleware(log))

	// /healthz и /metrics — без JWT.
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
	dataMartsRouter.Register(f, dataMartsRouter.Deps{
		JWTConfig: jwtCfg,
		Handler:   h,
	})

	return &App{
		cfg:   cfg,
		log:   log,
		fiber: f,
		pool:  pool,
	}, nil
}

// Run — запускает Fiber listener; блокируется до ctx.Done.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
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
			return fmt.Errorf("martsapp: %w", err)
		}
		return nil
	}
}

// Shutdown — graceful: fiber.Shutdown → pool.Close.
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.log.Error("fiber shutdown error", "error", err)
		return fmt.Errorf("martsapp: %w", err)
	}
	if a.pool != nil {
		a.pool.Close()
	}
	a.log.Info("shutdown complete")
	return nil
}
