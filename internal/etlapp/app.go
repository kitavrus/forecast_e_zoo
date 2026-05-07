// Package etlapp собирает компоненты сервиса etl (Модуль 2) воедино.
//
// DI порядок (заполняется последовательно фазами 06/10/13/14/15/16):
//  1. slog logger (получаем извне)
//  2. pgxpool.New
//  3. Repository (фаза 06)
//  4. Validation engine (фаза 09)
//  5. Extractor / Transformer / Loader (фазы 10/11/12)
//  6. EtlPipeline service (фаза 13)
//  7. Scheduler (фаза 14)
//  8. Handlers + Router (фаза 15)
package etlapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	etlconfig "github.com/Kitavrus/e_zoo/internal/etlapp/config"
	etldeps "github.com/Kitavrus/e_zoo/internal/etlapp/deps"
)

const shutdownTimeout = 30 * time.Second

// App — корневая структура etl-сервиса.
type App struct {
	cfg   *etlconfig.Config
	log   *slog.Logger
	deps  *etldeps.Deps
	fiber *fiber.App
}

// New собирает граф зависимостей.
func New(ctx context.Context, cfg *etlconfig.Config, log *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, errors.New("etlapp: config is nil")
	}
	if log == nil {
		return nil, errors.New("etlapp: logger is nil")
	}

	deps, err := etldeps.BuildDeps(ctx, cfg, log)
	if err != nil {
		return nil, fmt.Errorf("etlapp: build deps: %w", err)
	}

	// HTTP layer (фаза 15 добавит реальные роутеры).
	fiberApp := fiber.New(fiber.Config{
		AppName:      "etl",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	return &App{
		cfg:   cfg,
		log:   log,
		deps:  deps,
		fiber: fiberApp,
	}, nil
}

// Run запускает HTTP-сервер и блокируется до отмены ctx или фатальной ошибки.
//
// На текущей фазе 01 — заглушка: поднимает HTTP-сервер на cfg.HTTPPort
// и ждёт сигнала остановки. Реальная логика scheduler/pipeline появится в фазах 13/14/15.
func (a *App) Run(ctx context.Context) error {
	if a == nil {
		return errors.New("etlapp: app is nil")
	}
	addr := fmt.Sprintf(":%d", a.cfg.HTTPPort)
	a.log.Info("etl app started, waiting", "addr", addr)

	errCh := make(chan error, 1)
	go func() {
		// nolint:contextcheck // Fiber не принимает ctx; lifecycle управляется через Shutdown.
		if err := a.fiber.Listen(addr); err != nil {
			errCh <- fmt.Errorf("etlapp: fiber listen: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		a.log.Info("etl app: shutdown signal received")
		return a.shutdown()
	case err := <-errCh:
		return err
	}
}

func (a *App) shutdown() error {
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := a.fiber.ShutdownWithContext(shutCtx); err != nil {
		a.log.Error("etl app: fiber shutdown failed", "err", err)
	}

	a.deps.Close()
	a.log.Info("etl app: shutdown complete")
	return nil
}
