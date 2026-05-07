# Design DI (Dependency Injection) — source-adapter

Сборка модуля без DI-фреймворка (без `wire`, без `fx`). Простая «ручная» сборка в `internal/app/app.go`.

---

## 1. cmd/source-adapter/main.go

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/Kitavrus/e_zoo/internal/app"
    "github.com/Kitavrus/e_zoo/internal/config"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        slog.Error("config load failed", "err", err)
        os.Exit(1)
    }

    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: cfg.LogLevel,
    }))
    slog.SetDefault(logger)

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    application, err := app.New(ctx, cfg, logger)
    if err != nil {
        slog.Error("app init failed", "err", err)
        os.Exit(1)
    }

    if err := application.Run(ctx); err != nil {
        slog.Error("app run failed", "err", err)
        os.Exit(1)
    }
}
```

## 2. internal/config/config.go

```go
package config

import (
    "log/slog"
    "time"

    "github.com/kelseyhightower/envconfig"
)

type Config struct {
    HTTPAddr             string        `envconfig:"HTTP_ADDR"             default:":8080"`
    DBDSN                string        `envconfig:"DB_DSN"                required:"true"`
    DBMaxConns           int32         `envconfig:"DB_MAX_CONNS"          default:"20"`
    DBMinConns           int32         `envconfig:"DB_MIN_CONNS"          default:"2"`
    LogLevel             slog.Level    `envconfig:"LOG_LEVEL"             default:"INFO"`

    JWTAlgorithm         string        `envconfig:"JWT_ALG"               default:"HS256"`
    JWTSecret            string        `envconfig:"JWT_SECRET"            required:"true"`
    JWTPublicKeyPath     string        `envconfig:"JWT_PUBLIC_KEY_PATH"   default:""`
    JWTAdminRole         string        `envconfig:"JWT_ADMIN_ROLE"        default:"admin-cli"`
    JWTReadRoles         []string      `envconfig:"JWT_READ_ROLES"        default:"x-flow-etl,it-read"`

    SourceAdapterCron    string        `envconfig:"SOURCE_ADAPTER_CRON_SCHEDULE" default:"0 2 * * *"`
    SourceAdapterTZ      string        `envconfig:"SOURCE_ADAPTER_TZ"            default:"Europe/Kyiv"`
    QualityThresholdPct  float64       `envconfig:"QUALITY_THRESHOLD_PCT"        default:"1.0"`

    ERPBaseURL           string        `envconfig:"ERP_BASE_URL"          default:""`
    ERPAuthMode          string        `envconfig:"ERP_AUTH_MODE"         default:"none"` // none|bearer|mtls|apikey
    ERPHTTPTimeout       time.Duration `envconfig:"ERP_HTTP_TIMEOUT"      default:"30s"`
    ERPRetryMax          int           `envconfig:"ERP_RETRY_MAX"         default:"3"`
    ERPRetryBackoffCap   time.Duration `envconfig:"ERP_RETRY_BACKOFF_CAP" default:"30s"`

    ExportsBaseDir       string        `envconfig:"EXPORTS_BASE_DIR"      default:"/var/exports"`
    ExportsRetention     time.Duration `envconfig:"EXPORTS_RETENTION"     default:"24h"`
    ExportsInlineMaxMB   int           `envconfig:"EXPORTS_INLINE_MAX_MB" default:"50"`

    AuditRetention       time.Duration `envconfig:"AUDIT_RETENTION"       default:"2160h"` // 90d
    RejectLogRetention   time.Duration `envconfig:"REJECT_LOG_RETENTION"  default:"2160h"` // 90d

    PrometheusPath       string        `envconfig:"PROMETHEUS_PATH"       default:"/metrics"`

    ValidationRulesPath        string  `envconfig:"VALIDATION_RULES_PATH"        default:"/etc/source-adapter/validation_rules.yaml"`
    MasterTrackedFieldsPath    string  `envconfig:"MASTER_TRACKED_FIELDS_PATH"   default:"/etc/source-adapter/master_tracked_fields.yaml"`

    AppEnv               string        `envconfig:"APP_ENV"               default:"dev"` // dev | prod
    StaleLoadTimeout     time.Duration `envconfig:"STALE_LOAD_TIMEOUT"    default:"1h"`  // Q-015
}

func Load() (*Config, error) {
    var c Config
    if err := envconfig.Process("", &c); err != nil { return nil, err }
    return &c, nil
}
```

## 3. internal/app/app.go (главная сборка)

```go
package app

import (
    "context"
    "log/slog"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/recover"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/Kitavrus/e_zoo/internal/config"
    "github.com/Kitavrus/e_zoo/internal/middleware"
    "github.com/Kitavrus/e_zoo/internal/routers"

    de "github.com/Kitavrus/e_zoo/internal/features/data_export"
    de_handler "github.com/Kitavrus/e_zoo/internal/features/data_export/handler"
    de_repo "github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
    de_service "github.com/Kitavrus/e_zoo/internal/features/data_export/service"
    de_validators "github.com/Kitavrus/e_zoo/internal/features/data_export/validators"
    de_storage "github.com/Kitavrus/e_zoo/internal/features/data_export/exports_storage"
    de_scheduler "github.com/Kitavrus/e_zoo/internal/features/data_export/scheduler"
)

type App struct {
    cfg    *config.Config
    log    *slog.Logger
    fiber  *fiber.App
    pool   *pgxpool.Pool
    sched  de_scheduler.Scheduler
}

func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
    // 1. PG pool
    poolCfg, err := pgxpool.ParseConfig(cfg.DBDSN)
    if err != nil { return nil, err }
    poolCfg.MaxConns = cfg.DBMaxConns
    poolCfg.MinConns = cfg.DBMinConns
    poolCfg.MaxConnLifetime = 30 * time.Minute
    pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
    if err != nil { return nil, err }
    if err := pool.Ping(ctx); err != nil { return nil, err }

    // 2. Repository (единый для фичи)
    repo := de_repo.New(pool, log)

    // 3. Validator engine + tracked fields
    rules, err := de_validators.LoadRulesYAML(cfg.ValidationRulesPath)
    if err != nil { return nil, err }
    valEngine := de_validators.NewEngine(rules, log)

    tracked, err := de_validators.LoadTrackedFieldsYAML(cfg.MasterTrackedFieldsPath)
    if err != nil { return nil, err }

    // 4. Source reader (impl зависит от Q-001/Q-002)
    sourceAuth := de_service.NewSourceAuth(cfg) // строит по ERPAuthMode
    httpClient := de_service.NewERPHTTPClient(cfg, sourceAuth, log)
    var reader de_service.SourceReader
    if cfg.ERPBaseURL == "" {
        reader = de_service.NewInMemoryReader() // dev placeholder
    } else {
        reader = de_service.NewERPHTTPReader(cfg.ERPBaseURL, httpClient, log)
    }

    // 5. Exports storage
    storage, err := de_storage.NewLocalFS(cfg.ExportsBaseDir)
    if err != nil { return nil, err }

    // 6. Snapshot service + Loader
    snapSvc := de_service.NewSnapshotService(repo, log)
    auditWriter := de_service.NewAuditWriter(repo, log)
    loaderSvc := de_service.NewLoader(de_service.LoaderDeps{
        Repo: repo, Reader: reader, Validator: valEngine,
        Snapshot: snapSvc, Tracked: tracked,
        QualityThresholdPct: cfg.QualityThresholdPct,
        Logger: log,
    })
    exportsSvc := de_service.NewExportsService(repo, storage, log)

    // 7. Scheduler
    sched, err := de_scheduler.New(cfg.SourceAdapterCron, cfg.SourceAdapterTZ, loaderSvc, snapSvc, log)
    if err != nil { return nil, err }

    // 8. Handlers (по одному на сущность)
    handlers := de_handler.Build(de_handler.Deps{
        Repo:        repo,
        Snapshot:    snapSvc,
        Loader:      loaderSvc,
        Exports:     exportsSvc,
        Audit:       auditWriter,
        MasterEnts:  de_service.NewMasterEntities(repo, snapSvc, log),
        Facts:       de_service.NewFactsService(repo, snapSvc, log),
        ChangeLog:   de_service.NewChangeLogService(repo, log),
        Logger:      log,
    })

    // 9. Fiber app + middleware
    fapp := fiber.New(fiber.Config{
        AppName:               "source-adapter",
        ReadTimeout:           60 * time.Second,
        WriteTimeout:          5 * time.Minute, // длинные NDJSON-стримы
        DisableStartupMessage: cfg.AppEnv == "prod",
        ErrorHandler:          middleware.FiberErrorHandler(log),
    })
    fapp.Use(recover.New())
    fapp.Use(middleware.RequestID())
    fapp.Use(middleware.Logger(log))

    jwtMW := middleware.JWT(middleware.JWTConfig{
        Alg: cfg.JWTAlgorithm, Secret: cfg.JWTSecret, PublicKeyPath: cfg.JWTPublicKeyPath,
    })
    roleMW := middleware.Role(middleware.RoleConfig{
        AdminRole: cfg.JWTAdminRole, ReadRoles: cfg.JWTReadRoles,
    })
    auditMW := middleware.Audit(auditWriter)

    routers.RegisterAll(fapp, routers.Deps{
        DataExport: handlers,
        JWT: jwtMW, Role: roleMW, Audit: auditMW,
        PrometheusPath: cfg.PrometheusPath,
    })

    return &App{cfg: cfg, log: log, fiber: fapp, pool: pool, sched: sched}, nil
}

func (a *App) Run(ctx context.Context) error {
    a.sched.Start()
    defer a.sched.Stop()

    errCh := make(chan error, 1)
    go func() {
        if err := a.fiber.Listen(a.cfg.HTTPAddr); err != nil { errCh <- err }
    }()

    select {
    case <-ctx.Done():
        a.log.Info("shutdown signal received")
    case err := <-errCh:
        return err
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    _ = a.fiber.ShutdownWithContext(shutdownCtx)
    a.pool.Close()
    return nil
}
```

## 4. internal/routers/routers.go

```go
package routers

import (
    "github.com/gofiber/fiber/v3"
    de_handler "github.com/Kitavrus/e_zoo/internal/features/data_export/handler"
    "github.com/Kitavrus/e_zoo/internal/middleware"
)

type Deps struct {
    DataExport     de_handler.Handlers
    JWT            fiber.Handler
    Role           middleware.RoleFactory
    Audit          fiber.Handler
    PrometheusPath string
}

func RegisterAll(app *fiber.App, d Deps) {
    // Public
    app.Get("/v1/healthz", d.DataExport.Healthz.Handle)
    app.Get(d.PrometheusPath, middleware.PrometheusHandler())

    // /v1/* (read, JWT + read-role)
    v1 := app.Group("/v1", d.JWT, d.Role.Read())

    v1.Get("/snapshots",          d.DataExport.Snapshots.List)
    v1.Get("/snapshots/current",  d.DataExport.Snapshots.Current)

    v1.Get("/products",                              d.DataExport.Products.List)
    v1.Get("/product_barcodes",                      d.DataExport.ProductBarcodes.List)
    v1.Get("/category",                              d.DataExport.Categories.List)
    v1.Get("/location",                              d.DataExport.Locations.List)
    v1.Get("/supplier",                              d.DataExport.Suppliers.List)
    v1.Get("/store_assortment",                      d.DataExport.StoreAssortment.List)
    v1.Get("/store_assortment/lifecycle_events",     d.DataExport.StoreAssortmentLifecycle.List)
    v1.Get("/master_change_log",                     d.DataExport.MasterChangeLog.List)
    v1.Get("/supplier_stock_snapshot",               d.DataExport.SupplierStock.List)
    v1.Get("/supply_spec",                           d.DataExport.SupplySpec.List)
    v1.Get("/promo",                                 d.DataExport.Promo.List)
    v1.Get("/supply_plan",                           d.DataExport.SupplyPlan.List)
    v1.Get("/order_rule",                            d.DataExport.OrderRule.List)

    // facts
    v1.Get("/receipt_line",                          d.DataExport.ReceiptLine.List)
    v1.Get("/location_stock_snapshot",               d.DataExport.LocationStock.List)
    v1.Get("/stock_movement",                        d.DataExport.StockMovement.List)

    // exports
    v1.Post("/exports",                              d.DataExport.Exports.Create)
    v1.Get("/exports/:id",                           d.DataExport.Exports.Get)
    v1.Get("/exports/:id/download",                  d.DataExport.Exports.Download)

    // /admin/* (JWT + admin role + audit)
    admin := app.Group("/admin", d.JWT, d.Role.Admin(), d.Audit)
    admin.Post("/loads",            d.DataExport.AdminLoads.Start)
    admin.Post("/loads/:id/retry",  d.DataExport.AdminLoads.Retry)
    admin.Get("/loads/:id",         d.DataExport.AdminLoads.Get)
    admin.Get("/reject-log",        d.DataExport.AdminRejectLog.List)
}
```

## 5. Жизненный цикл shutdown

1. SIGINT/SIGTERM → `ctx.Done()`.
2. `sched.Stop()` — ждёт завершения текущего job (`gocron.Shutdown()`).
3. `fiber.ShutdownWithContext(30s)` — даём в-flight запросам дописать.
4. `pool.Close()` — закрываем pgxpool.
5. Process exit с 0.

Если в момент shutdown идёт `daily-load`, он:
- Замечает `ctx.Done()` в loader-pipeline (передаём `ctx` глубоко вниз).
- Записывает `loads.status='failed'`, `failure_reason='shutdown_signal'`.
- Освобождает advisory lock.
- Корректно закрывается.

## 6. Stale load detection (Q-015)

При старте процесса (после миграций):

```go
// internal/app/startup_recovery.go
func RecoverStaleLoads(ctx context.Context, repo Repository, timeout time.Duration, log *slog.Logger) {
    rows, _ := repo.SelectRunningLoadsOlderThan(ctx, timeout)
    for _, l := range rows {
        log.Warn("aborting stale load", "load_id", l.ID, "started_at", l.StartedAt)
        _ = repo.UpdateLoadStatus(ctx, l.ID, "failed", ptr(time.Now()), nil)
    }
}
```

`STALE_LOAD_TIMEOUT` default = `1h`.
