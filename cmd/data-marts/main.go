// Command data-marts — entrypoint сервиса data-marts (Модуль 3).
//
// Грузит конфиг MARTS_* из ENV, поднимает корневой slog-логгер,
// создаёт *martsapp.App и блокируется в Run до SIGINT/SIGTERM.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	martsapp "github.com/Kitavrus/e_zoo/internal/martsapp"
	martsconfig "github.com/Kitavrus/e_zoo/internal/martsapp/config"
	"github.com/Kitavrus/e_zoo/internal/logger"
)

// version — заполняется через -ldflags при сборке.
var version = "dev"

func main() {
	cfg, err := martsconfig.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("data-marts config load failed", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.SlogLevel())
	slog.SetDefault(log)
	log.Info("data-marts starting",
		"version", version,
		"env", cfg.Env,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := martsapp.New(ctx, cfg, log)
	if err != nil {
		log.Error("data-marts app init failed", "err", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		log.Error("data-marts app run failed", "err", err)
		os.Exit(1)
	}
}
