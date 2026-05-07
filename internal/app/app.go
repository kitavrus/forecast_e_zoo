// Package app собирает компоненты сервиса source-adapter воедино.
//
// DI порядок (фаза 15):
//  1. slog logger (получаем извне)
//  2. pgxpool.New
//  3. Repository
//  4. Snapshot.Seed (idempotent)
//  5. validation.Engine.Load
//  6. Loader (с stub-reader; реальный ERP-reader — пост-MVP)
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
	dataMartsHandler "github.com/Kitavrus/e_zoo/internal/features/data_marts/handler"
	dataMartsRepo "github.com/Kitavrus/e_zoo/internal/features/data_marts/repository"
	dataMartsRouter "github.com/Kitavrus/e_zoo/internal/features/data_marts/router"
	dataMartsService "github.com/Kitavrus/e_zoo/internal/features/data_marts/service"
	kpiEngine "github.com/Kitavrus/e_zoo/internal/features/kpi/engine"
	kpiHandler "github.com/Kitavrus/e_zoo/internal/features/kpi/handler"
	kpiRepo "github.com/Kitavrus/e_zoo/internal/features/kpi/repository"
	kpiRouter "github.com/Kitavrus/e_zoo/internal/features/kpi/router"
	kpiScheduler "github.com/Kitavrus/e_zoo/internal/features/kpi/scheduler"
	kpiService "github.com/Kitavrus/e_zoo/internal/features/kpi/service"
	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/internal/observability"
	"github.com/Kitavrus/e_zoo/internal/routers"
)

const shutdownTimeout = 30 * time.Second

// App — корневая структура.
type App struct {
	cfg          *config.Config
	log          *slog.Logger
	fiber        *fiber.App
	pool         *pgxpool.Pool
	scheduler    *scheduler.Scheduler
	kpiScheduler *kpiScheduler.Scheduler
}

// New собирает граф зависимостей.
//
//nolint:funlen,gocyclo // DI compose-функция — длинная по природе
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
	if r := tryStubReader(cfg, log); r != nil {
		ldr = loader.NewLoader(r, repo, engine, loader.Options{Logger: log})
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
		JWTConfig:          jwtCfg,
		HealthzHandler:     healthzH,
		SnapshotsHandler:   snapshotsH,
		ProductsHandler:    productsH,
		ReceiptLineHandler: receiptH,
		ExportsHandler:     exportsH,
		AdminLoadsHandler:  adminH,
		AuditMiddleware:    auditWriter.Middleware(),
		MetricsHandler:     observability.Handler(metricsReg),
	}

	// data_marts: read-only API над marts.* (slim feature, тот же binary).
	martsRepo := dataMartsRepo.New(pool)
	martsReader := dataMartsService.NewPGReader(martsRepo)
	martsSvc := dataMartsService.New(martsReader)
	martsH := dataMartsHandler.NewHandler(martsSvc)
	martsDeps := dataMartsRouter.Deps{
		JWTConfig: jwtCfg,
		Handler:   martsH,
	}

	// kpi: KPI engine + admin API (Module 4 kpi-calibration).
	kRepo := kpiRepo.New(pool)
	kEng := kpiEngine.New(kRepo, log, kpiEngine.NewPrometheusMetrics())
	kSch, kSchErr := kpiScheduler.New(kpiScheduler.Config{
		CronExpr: cfg.KPICronSchedule,
		TZ:       cfg.KPICronTZ,
	}, kEng, pool, log)
	if kSchErr != nil {
		log.Warn("app: kpi scheduler init failed (continue without scheduler)", "error", kSchErr)
		kSch = nil
	}
	var kSvcTrigger kpiService.Trigger
	if kSch != nil {
		kSvcTrigger = kSch
	}
	kSvc := kpiService.New(kRepo, kSvcTrigger)
	kH := kpiHandler.NewHandler(kSvc)
	kpiDeps := kpiRouter.Deps{
		JWTConfig: jwtCfg,
		Handler:   kH,
	}

	routers.Register(f, deps, martsDeps, kpiDeps)

	return &App{
		cfg:          cfg,
		log:          log,
		fiber:        f,
		pool:         pool,
		scheduler:    sch,
		kpiScheduler: kSch,
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
	if a.kpiScheduler != nil {
		if err := a.kpiScheduler.Start(ctx); err != nil {
			a.log.Warn("app: kpi scheduler start failed", "error", err)
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
			return err
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
	if a.kpiScheduler != nil {
		if err := a.kpiScheduler.Stop(); err != nil {
			a.log.Warn("kpi scheduler stop error", "error", err)
		}
	}
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		a.log.Error("fiber shutdown error", "error", err)
		return err
	}
	if a.pool != nil {
		a.pool.Close()
	}
	a.log.Info("shutdown complete")
	return nil
}

// noopTrigger — placeholder, если scheduler не сконфигурирован.
//
// TryTrigger возвращает (false, nil): без scheduler-а load запустить нельзя,
// и параллельный POST /admin/loads должен получить 409 (а не «псевдо-202»).
type noopTrigger struct{}

func (noopTrigger) TriggerOnce(_ context.Context) error { return nil }

func (noopTrigger) TryTrigger(_ context.Context) (bool, error) {
	return false, nil
}

// tryStubReader пытается загрузить in-memory ErpEZooReader из ERPBaseURL,
// если это путь к директории с fixtures (MVP fallback).
//
// Если ERP_BASE_URL пуст и ERP_AUTH_MODE=none — это ожидаемый dev-сценарий,
// но молчание сильно ухудшает DX (Issue #4 из validation): сервис стартует с
// noopTrigger, /admin/loads возвращает 202, но в БД ничего не появляется.
// Поэтому логируем явный WARN со ссылкой, как починить.
func tryStubReader(cfg *config.Config, log *slog.Logger) loader.SourceReader {
	if cfg.ERPBaseURL == "" {
		log.Warn("app: ERP_BASE_URL is empty — using in-memory stub backend (no real loads will run; "+
			"Q-001..Q-003 blocked, POST /admin/loads will reply 202 with no DB writes); "+
			"set ERP_BASE_URL=./testdata/fixtures (or path to ERP fixtures) to enable loads",
			"erp_auth_mode", cfg.ERPAuthMode)
		return nil
	}
	r, err := loader.New(cfg.ERPBaseURL)
	if err != nil {
		log.Warn("app: stub reader load failed", "dir", cfg.ERPBaseURL, "error", err)
		return nil
	}
	log.Info("app: stub reader loaded from fixtures", "dir", cfg.ERPBaseURL)
	return r
}
