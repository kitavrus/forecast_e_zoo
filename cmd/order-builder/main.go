// Command order-builder — entrypoint сервиса order-builder (Модуль 6).
//
// Грузит конфиг ORDER_BUILDER_* из ENV, поднимает корневой slog-логгер,
// создаёт *ordersapp.App и блокируется в Run до SIGINT/SIGTERM.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kitavrus/e_zoo/internal/logger"
	ordersapp "github.com/Kitavrus/e_zoo/internal/ordersapp"
	ordersappconfig "github.com/Kitavrus/e_zoo/internal/ordersapp/config"
)

// version — заполняется через -ldflags при сборке.
var version = "dev"

func main() {
	cfg, err := ordersappconfig.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("order-builder config load failed", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg.SlogLevel())
	slog.SetDefault(log)
	log.Info("order-builder starting",
		"version", version,
		"env", cfg.Env,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := ordersapp.New(ctx, cfg, log)
	if err != nil {
		log.Error("order-builder app init failed", "err", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		log.Error("order-builder app run failed", "err", err)
		os.Exit(1)
	}
}
