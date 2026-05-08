// Package app собирает компоненты сервиса source-adapter воедино.
//
// Только M1 (data_export). Остальные модули (M2..M7) — отдельные binary.
//
// DI порядок:
//  1. slog logger (получаем извне)
//  2. pgxpool.New
//  3. Repository
//  4. Snapshot.Seed (idempotent)
//  5. validation.Engine.Load
//  6. Loader (с stub-reader)
//  7. Scheduler (registered, started in Run)
//  8. ExportsStorage + Exports.Service
//  9. Audit.Writer
// 10. Handlers
// 11. Router (через internal/routers)
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/config"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/audit"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/exports"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/exports_storage"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/loader"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	dataExportRouter "github.com/Kitavrus/e_zoo/internal/features/data_export/router"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/scheduler"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/snapshot"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/validation"
	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/internal/observability"
	"github.com/Kitavrus/e_zoo/internal/routers"
)

const shutdownTimeout = 30 * time.Second

// App — корневая структура.
type App struct {
	cfg       *config.Config
	log       *slog.Logger
	fiber     *fiber.App
	pool      *pgxpool.Pool
	scheduler *scheduler.Scheduler
}

// New собирает граф зависимостей.
//
//nolint:funlen,cyclop // DI compose-функция — длинная по природе
func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if log == nil {
		return nil, errors.New("logger is nil")
	}

	pool, err := pgxpool.New(ctx, cfg.DBDsn)
	if err != nil {
		return nil, fmt.Errorf("app: pgxpool: %w", err)
	}

	repo := repository.New(pool)
	if err := repo.Seed(ctx); err != nil {
		log.Warn("app: seed snapshot_pointer failed (continue)", "error", err)
	}

	var engine *validation.Engine
	if cfg.ValidationRulesPath != "" {
		eng, lerr := validation.Load(cfg.ValidationRulesPath)
		if lerr != nil {
			log.Warn("app: validation rules load failed", "path", cfg.ValidationRulesPath, "error", lerr)
			engine = validation.New(nil, nil)
		} else {
			engine = eng
		}
	} else {
		engine = validation.New(nil, nil)
	}

	var ldr *loader.Loader
	if r := buildSourceReader(cfg, log); r != nil {
		// Окно facts: [today - LoadFactsWindowDays, today]. Default 365 дней
		// (см. config.go) — нужно KPI/Forecast и закрывает свежий seed mock-erp.
		now := time.Now().UTC()
		dateTo := now
		dateFrom := dateTo.AddDate(0, 0, -cfg.LoadFactsWindowDays)
		ldr = loader.NewLoader(r, repo, engine, loader.Options{
			Logger:   log,
			DateFrom: dateFrom,
			DateTo:   dateTo,
		})
	}

	var sch *scheduler.Scheduler
	if ldr != nil {
		s, serr := scheduler.New(scheduler.Config{
			CronExpr:    cfg.SourceAdapterCron,
			TZ:          cfg.SourceAdapterTZ,
			StaleAfter:  cfg.StaleLoadTimeout,
			MonthsAhead: 2,
			Source:      "erp_e_zoo",
		}, ldr, repo, pool, log)
		if serr != nil {
			return nil, fmt.Errorf("app: scheduler: %w", serr)
		}
		sch = s
	}

	storage, err := exports_storage.NewLocalFS(cfg.ExportsBaseDir)
	if err != nil {
		log.Warn("app: exports storage init failed", "dir", cfg.ExportsBaseDir, "error", err)
	}
	var exportsSvc *exports.Service
	if storage != nil {
		exportsSvc = exports.New(storage, log)
	}

	auditWriter := audit.New(repo, log)
	snapSvc := snapshot.New(repo, log)

	healthzH := handler.NewHealthzHandler(pool)
	snapshotsH := handler.NewSnapshotsHandler(snapSvc)
	productsH := handler.NewProductsHandler(repo, snapSvc)
	receiptH := handler.NewReceiptLineHandler(repo, snapSvc)
	categoryH := handler.NewCategoryHandler(repo, snapSvc)
	locationH := handler.NewLocationHandler(repo, snapSvc)
	supplierH := handler.NewSupplierHandler(repo, snapSvc)
	orderRuleH := handler.NewOrderRuleHandler(repo, snapSvc)
	supplySpecH := handler.NewSupplySpecHandler(repo, snapSvc)
	locationStockH := handler.NewLocationStockSnapshotHandler(repo, snapSvc)
	// Phase 13 — 8 missing entity handlers.
	productBarcodesH := handler.NewProductBarcodesHandler(repo, snapSvc)
	promoH := handler.NewPromoHandler(repo, snapSvc)
	supplyPlanH := handler.NewSupplyPlanHandler(repo, snapSvc)
	storeAssortmentH := handler.NewStoreAssortmentHandler(repo, snapSvc)
	lifecycleEventsH := handler.NewLifecycleEventsHandler(repo, snapSvc)
	masterChangeLogH := handler.NewMasterChangeLogHandler(repo, snapSvc)
	stockMovementH := handler.NewStockMovementHandler(repo, snapSvc)
	supplierStockH := handler.NewSupplierStockSnapshotHandler(repo, snapSvc)
	var exportsH *handler.ExportsHandler
	if exportsSvc != nil {
		exportsH = handler.NewExportsHandler(exportsSvc)
	}

	var trigger handler.AdminLoadsTrigger = noopTrigger{}
	if sch != nil {
		trigger = sch
	}
	adminH := handler.NewAdminLoadsHandler(repo, trigger, repo)

	metricsReg := observability.Init()

	f := fiber.New(fiber.Config{
		AppName:      "source-adapter",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		BodyLimit:    10 * 1024 * 1024,
	})
	f.Use(middleware.RequestID())
	f.Use(observability.HTTPMetricsMiddleware())
	f.Use(observability.AccessLogMiddleware(log))

	jwtCfg := middleware.JWTConfig{
		Alg:           cfg.JWTAlg,
		Secret:        cfg.JWTSecret,
		PublicKeyPath: cfg.JWTPublicKeyPath,
	}

	deps := dataExportRouter.Deps{
		JWTConfig:                    jwtCfg,
		HealthzHandler:               healthzH,
		SnapshotsHandler:             snapshotsH,
		ProductsHandler:              productsH,
		ReceiptLineHandler:           receiptH,
		CategoryHandler:              categoryH,
		LocationHandler:              locationH,
		SupplierHandler:              supplierH,
		OrderRuleHandler:             orderRuleH,
		SupplySpecHandler:            supplySpecH,
		LocationStockSnapshotHandler: locationStockH,
		ProductBarcodesHandler:       productBarcodesH,
		PromoHandler:                 promoH,
		SupplyPlanHandler:            supplyPlanH,
		StoreAssortmentHandler:       storeAssortmentH,
		LifecycleEventsHandler:       lifecycleEventsH,
		MasterChangeLogHandler:       masterChangeLogH,
		StockMovementHandler:         stockMovementH,
		SupplierStockSnapshotHandler: supplierStockH,
		ExportsHandler:               exportsH,
		AdminLoadsHandler:            adminH,
		AuditMiddleware:              auditWriter.Middleware(),
		MetricsHandler:               observability.Handler(metricsReg),
	}

	routers.Register(f, deps)

	return &App{
		cfg:       cfg,
		log:       log,
		fiber:     f,
		pool:      pool,
		scheduler: sch,
	}, nil
}

// Run — стартует scheduler (если есть) и Fiber listener; блокируется до ctx.Done.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	if a.scheduler != nil {
		if err := a.scheduler.Start(ctx); err != nil {
			a.log.Warn("app: scheduler start failed", "error", err)
		}
	}

	go func() {
		a.log.Info("http server starting", "addr", a.cfg.HTTPAddr)
		listenCfg := fiber.ListenConfig{DisableStartupMessage: a.cfg.AppEnv != "dev"}
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
			return fmt.Errorf("app: %w", err)
		}
		return nil
	}
}

// Shutdown — graceful: scheduler.Stop → fiber.Shutdown → pool.Close.
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if a.scheduler != nil {
		if err := a.scheduler.Stop(ctx); err != nil {
			a.log.Warn("scheduler stop error", "error", err)
		}
	}
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.log.Error("fiber shutdown error", "error", err)
		return fmt.Errorf("app: %w", err)
	}
	if a.pool != nil {
		a.pool.Close()
	}
	a.log.Info("shutdown complete")
	return nil
}

// noopTrigger — placeholder, если scheduler не сконфигурирован.
type noopTrigger struct{}

func (noopTrigger) TriggerOnce(_ context.Context) error { return nil }

func (noopTrigger) TryTrigger(_ context.Context) (bool, error) {
	return false, nil
}

// buildSourceReader выбирает backend SourceReader по содержимому ERP_BASE_URL:
//
//   - "" → nil (in-memory stub, no real loads).
//   - "http://" / "https://" → HTTPSourceReader (реальный REST-клиент к mock-erp/E-Zoo).
//   - всё остальное (относительный/абсолютный путь) → file-backed ErpEZooReader из fixtures.
func buildSourceReader(cfg *config.Config, log *slog.Logger) loader.SourceReader {
	if cfg.ERPBaseURL == "" {
		log.Warn("app: ERP_BASE_URL is empty — using in-memory stub backend (no real loads will run; "+
			"Q-001..Q-003 blocked, POST /admin/loads will reply 202 with no DB writes); "+
			"set ERP_BASE_URL=./testdata/fixtures or http://mock-erp:8090 to enable loads",
			"erp_auth_mode", cfg.ERPAuthMode)
		return nil
	}
	if strings.HasPrefix(cfg.ERPBaseURL, "http://") || strings.HasPrefix(cfg.ERPBaseURL, "https://") {
		log.Info("app: using HTTPSourceReader",
			"base_url", cfg.ERPBaseURL,
			"timeout", cfg.ERPHTTPTimeout,
			"retry_max", cfg.ERPRetryMax,
			"backoff_cap", cfg.ERPRetryBackoffCap,
			"api_key_set", cfg.ERPAPIKey != "")
		return loader.NewHTTPSourceReader(loader.HTTPSourceReaderConfig{
			BaseURL:     cfg.ERPBaseURL,
			APIKey:      cfg.ERPAPIKey,
			HTTPTimeout: cfg.ERPHTTPTimeout,
			RetryMax:    cfg.ERPRetryMax,
			BackoffCap:  cfg.ERPRetryBackoffCap,
			Logger:      log,
		})
	}
	r, err := loader.New(cfg.ERPBaseURL)
	if err != nil {
		log.Warn("app: file-backed reader load failed", "dir", cfg.ERPBaseURL, "error", err)
		return nil
	}
	log.Info("app: using file-backed ErpEZooReader from fixtures", "dir", cfg.ERPBaseURL)
	return r
}
