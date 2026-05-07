// Command forecast — entrypoint сервиса forecast (Модуль 5).
//
// Грузит конфиг FORECAST_* из ENV, поднимает корневой slog-логгер,
// создаёт *forecastapp.App и блокируется в Run до SIGINT/SIGTERM.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	forecastapp "github.com/Kitavrus/e_zoo/internal/forecastapp"
	forecastappconfig "github.com/Kitavrus/e_zoo/internal/forecastapp/config"
	"github.com/Kitavrus/e_zoo/internal/logger"
)

// version — заполняется через -ldflags при сборке.
var version = "dev"

func main() {
	cfg, err := forecastappconfig.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("forecast config load failed", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.SlogLevel())
	slog.SetDefault(log)
	log.Info("forecast starting",
		"version", version,
		"env", cfg.Env,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := forecastapp.New(ctx, cfg, log)
	if err != nil {
		log.Error("forecast app init failed", "err", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		log.Error("forecast app run failed", "err", err)
		os.Exit(1)
	}
}
