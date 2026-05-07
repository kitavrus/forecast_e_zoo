// Command kpi — entrypoint сервиса kpi (Модуль 4).
//
// Грузит конфиг KPI_* из ENV, поднимает корневой slog-логгер,
// создаёт *kpiapp.App и блокируется в Run до SIGINT/SIGTERM.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	kpiapp "github.com/Kitavrus/e_zoo/internal/kpiapp"
	kpiappconfig "github.com/Kitavrus/e_zoo/internal/kpiapp/config"
	"github.com/Kitavrus/e_zoo/internal/logger"
)

// version — заполняется через -ldflags при сборке.
var version = "dev"

func main() {
	cfg, err := kpiappconfig.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("kpi config load failed", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.SlogLevel())
	slog.SetDefault(log)
	log.Info("kpi starting",
		"version", version,
		"env", cfg.Env,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := kpiapp.New(ctx, cfg, log)
	if err != nil {
		log.Error("kpi app init failed", "err", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		log.Error("kpi app run failed", "err", err)
		os.Exit(1)
	}
}
