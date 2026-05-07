// Command channel-router — entrypoint сервиса channel-router (Модуль 7).
//
// Грузит конфиг CHANNEL_ROUTING_* из ENV, поднимает корневой slog-логгер,
// создаёт *channelsapp.App и блокируется в Run до SIGINT/SIGTERM.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	channelsapp "github.com/Kitavrus/e_zoo/internal/channelsapp"
	channelsappconfig "github.com/Kitavrus/e_zoo/internal/channelsapp/config"
	"github.com/Kitavrus/e_zoo/internal/logger"
)

// version — заполняется через -ldflags при сборке.
var version = "dev"

func main() {
	cfg, err := channelsappconfig.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("channel-router config load failed", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.SlogLevel())
	slog.SetDefault(log)
	log.Info("channel-router starting",
		"version", version,
		"env", cfg.Env,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := channelsapp.New(ctx, cfg, log)
	if err != nil {
		log.Error("channel-router app init failed", "err", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		log.Error("channel-router app run failed", "err", err)
		os.Exit(1)
	}
}
