// Command source-adapter — entrypoint сервиса.
//
// Грузит конфиг из ENV, поднимает корневой slog-логгер,
// создаёт *app.App и блокируется в Run до SIGINT/SIGTERM.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kitavrus/e_zoo/internal/app"
	"github.com/Kitavrus/e_zoo/internal/config"
	"github.com/Kitavrus/e_zoo/internal/logger"
)

// version — заполняется через -ldflags при сборке.
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Минимальный логгер, чтобы не молчать при крахе конфигурации.
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("config load failed", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.SlogLevel())
	slog.SetDefault(log)
	log.Info("source-adapter starting",
		"version", version,
		"env", cfg.AppEnv,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("app init failed", "err", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		log.Error("app run failed", "err", err)
		os.Exit(1)
	}
}
