// Package app собирает компоненты сервиса source-adapter воедино.
//
// На фазе 01 здесь только Fiber + минимальный /healthz.
// Вся доменная сборка (DI, scheduler, handlers и т.д.) — фаза 15.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/config"
)

// shutdownTimeout — макс. время на graceful drain in-flight запросов.
const shutdownTimeout = 30 * time.Second

// App — корневая структура приложения.
type App struct {
	cfg   *config.Config
	log   *slog.Logger
	fiber *fiber.App
}

// New собирает Fiber и регистрирует базовые маршруты.
// На последующих фазах сюда добавится pgxpool, scheduler, handlers.
func New(_ context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if log == nil {
		return nil, errors.New("logger is nil")
	}

	f := fiber.New(fiber.Config{
		AppName:      "source-adapter",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		BodyLimit:    10 * 1024 * 1024, // 10 MB
	})

	// Recover от паник в Fiber v3 включён в ядро через ErrorHandler.
	// Кастомный recover-middleware добавим, когда ErrorHandler станет недостаточно.

	// Минимальный healthz: жив ли процесс. Расширим в фазе 16.
	f.Get("/healthz", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"status": "ok",
		})
	})

	return &App{
		cfg:   cfg,
		log:   log,
		fiber: f,
	}, nil
}

// Run запускает HTTP-сервер и блокирует до получения сигнала отмены ctx.
// При завершении — выполняет graceful shutdown.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.log.Info("http server starting", "addr", a.cfg.HTTPAddr)
		// Скрываем стартовый banner Fiber вне dev-окружения.
		listenCfg := fiber.ListenConfig{
			DisableStartupMessage: a.cfg.AppEnv != "dev",
		}
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
			a.log.Error("http server failed", "err", err)
			return err
		}
		return nil
	}
}

// Shutdown корректно останавливает Fiber c таймаутом.
// На последующих фазах сюда добавится pool.Close() и sched.Stop().
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.log.Error("fiber shutdown error", "err", err)
		return err
	}
	a.log.Info("shutdown complete")
	return nil
}
