// Package etlapp собирает компоненты сервиса etl (Модуль 2) воедино.
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
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/extractor"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/handler"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/loader"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/router"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/scheduler"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/service"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/transformer"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validators"
)

const shutdownTimeout = 30 * time.Second

// App — корневая структура etl-сервиса.
type App struct {
	cfg       *etlconfig.Config
	log       *slog.Logger
	deps      *etldeps.Deps
	fiber     *fiber.App
	scheduler *scheduler.Scheduler
}

// New собирает граф зависимостей.
//
//nolint:funlen,cyclop // линейный wire DI; разбиение на helper-ы лишь усложнило бы.
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

	repo := repository.New(deps.Pool)

	// Validation engine.
	engine, err := validation.Load(cfg.ValidationRulesPath)
	if err != nil {
		log.Warn("etlapp: validation rules unavailable, using empty engine",
			"path", cfg.ValidationRulesPath, "err", err)
		engine = validation.New(validation.DefaultRegistry(), nil)
	}

	// Extractor (HS256 token source if signing key set; иначе static empty token).
	tokenSrc, tokErr := buildTokenSource(cfg)
	if tokErr != nil {
		return nil, tokErr
	}
	httpClient := deps.HTTPClient
	extrClient, err := extractor.NewClient(httpClient, tokenSrc, extractor.ClientConfig{
		BaseURL:     cfg.SourceAdapterURL,
		HTTPTimeout: cfg.HTTPTimeout,
		RetryMax:    cfg.HTTPRetryMax,
		BackoffCap:  cfg.RetryBackoffCap,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("etlapp: extractor client: %w", err)
	}
	extr := service.ExtractorAdapter{
		Snapshots: extractor.NewSnapshotsClient(extrClient),
		Entities:  extractor.NewEntitiesClient(extrClient),
	}

	// Transformer registry + Loader + Pipeline.
	registry := transformer.NewRegistry(repo)
	ld := loader.New(deps.Pool, repo, log)

	pipelineCfg := service.EtlPipelineConfig{
		QualityThreshold: cfg.QualityThreshold,
	}
	pipeline := service.NewEtlPipeline(deps.Pool, repo, extr, engine, registry, ld, service.NoopMetrics{}, log, pipelineCfg)

	runSvc := service.NewEtlRunService(repo, pipeline)
	refreshSvc := service.NewMartRefreshService(deps.Pool, repo, registry, ld, extr)

	v := validators.New()
	h := handler.NewHandler(runSvc, refreshSvc, repo, v)

	// HTTP layer.
	fiberApp := fiber.New(fiber.Config{
		AppName:      "etl",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	})
	apiV1 := fiberApp.Group("/api/v1")
	router.Register(apiV1, h, router.Middlewares{
		Admin: router.AdminSecretMiddleware(cfg.AdminJWTSecret),
	})

	// Scheduler.
	maint := scheduler.NewPartitionMaintainer(deps.Pool)
	sch, err := scheduler.New(pipeline, maint, scheduler.NoopSkipMetrics{}, log, scheduler.Config{
		CronExpr: cfg.CronSchedule,
		Timezone: cfg.CronTimezone,
	})
	if err != nil {
		return nil, fmt.Errorf("etlapp: scheduler new: %w", err)
	}

	return &App{
		cfg:       cfg,
		log:       log,
		deps:      deps,
		fiber:     fiberApp,
		scheduler: sch,
	}, nil
}

func buildTokenSource(cfg *etlconfig.Config) (extractor.TokenSource, error) {
	if cfg.JWTSigningKey == "" {
		return extractor.StaticTokenSource{Value: ""}, nil
	}
	ts, err := extractor.NewHS256TokenSource(extractor.HS256Config{
		SigningKey: []byte(cfg.JWTSigningKey),
		Role:       cfg.JWTRole,
	})
	if err != nil {
		return nil, fmt.Errorf("etlapp: token source: %w", err)
	}
	return ts, nil
}

// Run запускает HTTP-сервер + scheduler и блокируется до отмены ctx.
func (a *App) Run(ctx context.Context) error {
	if a == nil {
		return errors.New("etlapp: app is nil")
	}
	if err := a.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("etlapp: scheduler start: %w", err)
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

	if a.scheduler != nil {
		if err := a.scheduler.Stop(shutCtx); err != nil {
			a.log.Error("etl app: scheduler stop failed", "err", err)
		}
	}
	if err := a.fiber.ShutdownWithContext(shutCtx); err != nil {
		a.log.Error("etl app: fiber shutdown failed", "err", err)
	}

	a.deps.Close()
	a.log.Info("etl app: shutdown complete")
	return nil
}
