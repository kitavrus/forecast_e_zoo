// Command etl — entrypoint сервиса x-flow-etl (Модуль 2).
//
// Грузит конфиг ETL_* из ENV, поднимает корневой slog-логгер,
// создаёт *etlapp.App и блокируется в Run до SIGINT/SIGTERM.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	etlapp "github.com/Kitavrus/e_zoo/internal/etlapp"
	etlconfig "github.com/Kitavrus/e_zoo/internal/etlapp/config"
	"github.com/Kitavrus/e_zoo/internal/logger"
)

// version — заполняется через -ldflags при сборке.
var version = "dev"

func main() {
	cfg, err := etlconfig.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("etl config load failed", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.SlogLevel())
	slog.SetDefault(log)
	log.Info("etl starting",
		"version", version,
		"env", cfg.Env,
		"port", cfg.HTTPPort,
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := etlapp.New(ctx, cfg, log)
	if err != nil {
		log.Error("etl app init failed", "err", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		log.Error("etl app run failed", "err", err)
		os.Exit(1)
	}
}
