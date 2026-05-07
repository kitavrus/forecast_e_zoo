// Package channelsapp собирает компоненты сервиса channel-router (Модуль 7) воедино.
package channelsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	channelsappconfig "github.com/Kitavrus/e_zoo/internal/channelsapp/config"
	channelsAuth "github.com/Kitavrus/e_zoo/internal/features/channels/auth"
	channelsFormatter "github.com/Kitavrus/e_zoo/internal/features/channels/formatter"
	channelsHandler "github.com/Kitavrus/e_zoo/internal/features/channels/handler"
	channelsRepo "github.com/Kitavrus/e_zoo/internal/features/channels/repository"
	channelsRouter "github.com/Kitavrus/e_zoo/internal/features/channels/router"
	channelsRouterSvc "github.com/Kitavrus/e_zoo/internal/features/channels/router_svc"
	channelsScheduler "github.com/Kitavrus/e_zoo/internal/features/channels/scheduler"
	channelsSender "github.com/Kitavrus/e_zoo/internal/features/channels/sender"
	channelsService "github.com/Kitavrus/e_zoo/internal/features/channels/service"
	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/internal/observability"
)

const shutdownTimeout = 30 * time.Second

// App — корневая структура channel-router сервиса.
type App struct {
	cfg       *channelsappconfig.Config
	log       *slog.Logger
	fiber     *fiber.App
	pool      *pgxpool.Pool
	scheduler *channelsScheduler.Scheduler
}

// New собирает граф зависимостей.
//
//nolint:funlen // DI compose-функция — длинная по природе
func New(ctx context.Context, cfg *channelsappconfig.Config, log *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, errors.New("channelsapp: config is nil")
	}
	if log == nil {
		return nil, errors.New("channelsapp: logger is nil")
	}

	pool, err := pgxpool.New(ctx, cfg.DBDsn)
	if err != nil {
		return nil, fmt.Errorf("channelsapp: pgxpool: %w", err)
	}

	repo := channelsRepo.New(pool)
	senderRegistry := buildChannelSenderRegistry(log)
	metricsReg := observability.Init()
	rtr := channelsRouterSvc.New(repo, pool, senderRegistry, log,
		buildChannelMetrics())

	var sch *channelsScheduler.Scheduler
	schInst, schErr := channelsScheduler.New(channelsScheduler.Config{
		CronExpr: cfg.CronSchedule,
		TZ:       cfg.CronTZ,
		MaxPOs:   cfg.MaxPOs,
	}, rtr, pool, log)
	if schErr != nil {
		log.Warn("channelsapp: scheduler init failed (continue without scheduler)", "error", schErr)
	} else {
		sch = schInst
	}
	var trigger channelsService.Trigger
	if sch != nil {
		trigger = sch
	}
	svc := channelsService.New(repo, rtr, trigger)
	h := channelsHandler.NewHandler(svc)

	f := fiber.New(fiber.Config{
		AppName:      "channel-router",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		BodyLimit:    10 * 1024 * 1024,
	})
	f.Use(middleware.RequestID())
	f.Use(observability.HTTPMetricsMiddleware())
	f.Use(observability.AccessLogMiddleware(log))

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
	channelsRouter.Register(f, channelsRouter.Deps{
		JWTConfig: jwtCfg,
		Handler:   h,
	})

	return &App{
		cfg:       cfg,
		log:       log,
		fiber:     f,
		pool:      pool,
		scheduler: sch,
	}, nil
}

// buildChannelSenderRegistry собирает sender.Registry с ErpAPISender (MVP)
// + NotImplementedSender для остальных channel_type.
func buildChannelSenderRegistry(log *slog.Logger) *channelsSender.Registry {
	authProvider := channelsAuth.NewAPIKeyProvider()
	jsonFmt := channelsFormatter.NewJSONFormatter()
	erp, err := channelsSender.NewErpAPISender(channelsSender.Config{
		AuthProvider: authProvider,
		Formatter:    jsonFmt,
		Logger:       log,
	})
	if err != nil {
		log.Warn("channelsapp: erp_api sender init failed", "error", err)
		return channelsSender.NewRegistry()
	}
	return channelsSender.NewRegistry(
		erp,
		&channelsSender.NotImplementedSender{Channel: "edi_x12"},
		&channelsSender.NotImplementedSender{Channel: "edi_edifact"},
		&channelsSender.NotImplementedSender{Channel: "1c_xml"},
		&channelsSender.NotImplementedSender{Channel: "crm"},
	)
}

// buildChannelMetrics возвращает Metrics callbacks.
func buildChannelMetrics() channelsRouterSvc.Metrics {
	return channelsRouterSvc.Metrics{
		SendTotal: func(channel, status string) {
			observability.ChannelSendTotal.WithLabelValues(channel, status).Inc()
		},
		SendDuration: func(channel string, seconds float64) {
			observability.ChannelSendDuration.WithLabelValues(channel).Observe(seconds)
		},
		RetryCount: func(channel string, retries int) {
			observability.ChannelRetryCountTotal.WithLabelValues(channel).Add(float64(retries))
		},
		IdempotentHit: func(channel string) {
			observability.ChannelIdempotentHitTotal.WithLabelValues(channel).Inc()
		},
	}
}

// Run — стартует scheduler и Fiber listener; блокируется до ctx.Done.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	if a.scheduler != nil {
		if err := a.scheduler.Start(ctx); err != nil {
			a.log.Warn("channelsapp: scheduler start failed", "error", err)
		}
	}

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
			return fmt.Errorf("channelsapp: %w", err)
		}
		return nil
	}
}

// Shutdown — graceful.
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if a.scheduler != nil {
		if err := a.scheduler.Stop(); err != nil {
			a.log.Warn("channels scheduler stop error", "error", err)
		}
	}
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.log.Error("fiber shutdown error", "error", err)
		return fmt.Errorf("channelsapp: %w", err)
	}
	if a.pool != nil {
		a.pool.Close()
	}
	a.log.Info("shutdown complete")
	return nil
}
