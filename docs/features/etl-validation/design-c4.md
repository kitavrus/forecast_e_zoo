# Design C4 — etl-validation

C4 levels 1-4 для feature `etl_validation`. Все диаграммы Mermaid.

---

## L1 — System Context

```mermaid
C4Context
title System Context — X-Flow ETL (Модуль 2)

Person(devops, "DevOps / on-call X-Flow", "Запускает retry, ondemand refresh")
Person(itread, "IT E-Zoo", "Read-only audit просмотр")

System(etl, "X-Flow ETL", "Cron-driven ETL: source-adapter API → 5 mart-таблиц + reject_log + provenance")

System_Ext(sa, "source-adapter (Модуль 1)", "REST API: master/facts NDJSON + snapshot pointer")
System_Ext(repl, "Replenishment (Модуль 5)", "Читает mart_calculation_input, mart_master_current, mart_demand_history (read-only SQL)")
System_Ext(kpi, "KPI module (Модуль 4)", "Читает mart_kpi_daily, mart_supplier_scorecard")
System_Ext(prom, "Prometheus + Grafana", "etl_* метрики, дашборд X-Flow")

Rel(devops, etl, "POST /admin/etl-runs, /retry, /marts/{name}/refresh", "HTTPS + JWT admin-cli")
Rel(itread, etl, "GET /admin/etl-runs", "HTTPS + JWT it-read")
Rel(etl, sa, "GET /v1/snapshots/current, /v1/{entity}", "HTTPS + JWT x-flow-etl")
Rel(repl, etl, "SELECT FROM marts.mart_*", "PG read-only role mart_reader")
Rel(kpi, etl, "SELECT FROM marts.mart_*", "PG read-only role mart_reader")
Rel(etl, prom, "scrape /metrics", "HTTP")
```

---

## L2 — Container

```mermaid
C4Container
title Containers — etl-validation

Person(devops, "DevOps")
System_Ext(sa, "source-adapter API")

Container_Boundary(b1, "X-Flow ETL deploy unit") {
  Container(bin, "cmd/etl", "Go 1.26 binary", "main, graceful shutdown")
  Container(httpapi, "HTTP API (Fiber v3)", "Go", "/admin/etl-*, /admin/marts/*, /admin/reject-log, /healthz, /metrics")
  Container(scheduler, "Scheduler", "go-co-op/gocron/v2", "Cron 02:30 Kyiv (configurable)")
  Container(pipeline, "ETL pipeline", "Go service", "Extract → Stage → Validate → Transform → Load → Flip")
  Container(extractor, "extractor (HTTP client)", "Go net/http + JWT", "GET к source-adapter с ETag, retry, backoff")
}

ContainerDb(pg, "PostgreSQL 18", "PG 18", "schema marts: 5 mart-таблиц + etl_runs + reject_log + audit_access")
Container_Ext(prom, "Prometheus")

Rel(devops, httpapi, "HTTPS + JWT admin-cli")
Rel(scheduler, pipeline, "tick → run pipeline")
Rel(pipeline, extractor, "fetch entities")
Rel(extractor, sa, "HTTPS + JWT x-flow-etl")
Rel(pipeline, pg, "INSERT/UPSERT marts.* + etl_runs + reject_log", "pgxpool")
Rel(httpapi, pg, "READ etl_runs, reject_log, audit_access INSERT")
Rel(prom, httpapi, "scrape /metrics")
```

---

## L3 — Components внутри `internal/features/etl_validation`

```mermaid
C4Component
title Components — feature etl_validation

Container_Ext(pg, "PostgreSQL 18 (schema marts)")
Container_Ext(saApi, "source-adapter REST API")

Container_Boundary(c1, "internal/features/etl_validation") {
  Component(router, "router", "Fiber v3 router", "Маршруты + JWT middleware + audit middleware")
  Component(handler, "handler", "Fiber handlers", "admin_etl_runs, admin_marts, admin_reject_log, healthz")
  Component(service, "service", "Go business logic", "etl_run.go (CRUD-API), etl_pipeline.go (orchestration), mart_refresh.go (ondemand)")
  Component(repo, "repository", "pgxpool", "etl_runs, reject_log, audit_access, marts, staging — все SQL go:embed")
  Component(extractor, "extractor", "net/http + JWT", "SnapshotsClient, EntitiesClient — клиент к source-adapter API")
  Component(transformer, "transformer", "Go + SQL", "build mart_demand_history / calculation_input / kpi_daily / master_current / supplier_scorecard")
  Component(loader, "loader", "Go", "UPSERT в mart_* + atomic flip (один tx: INSERT + UPDATE etl_runs)")
  Component(validator, "validation", "Go (engine reuse)", "fk_exists, unique_business_key, aggregate_sum_matches, referential_integrity, null_required_field")
  Component(scheduler, "scheduler", "go-co-op/gocron/v2", "Cron 02:30 Kyiv + advisory lock")
  Component(mappers, "mappers", "Go", "source DTO → domain → mart row")
  Component(models, "models/dto", "Go structs", "etl_run, reject, mart_*, admin DTOs")
  Component(sqls, "sqls", "go:embed *.sql", "queries + migrations 1001/1002")
  Component(audit, "audit middleware", "Go", "INSERT audit_access на каждый /admin/* запрос")
  Component(validators, "validators", "Go", "формат-валидация request DTO")
}

Rel(router, handler, "")
Rel(router, audit, "all /admin/*")
Rel(handler, service, "")
Rel(handler, validators, "validate request DTO")
Rel(service, repo, "")
Rel(service, extractor, "fetch")
Rel(service, validator, "check rules")
Rel(service, transformer, "build mart")
Rel(service, loader, "upsert + flip")
Rel(scheduler, service, "tick → etl_pipeline.Run()")
Rel(transformer, mappers, "")
Rel(repo, sqls, "")
Rel(repo, pg, "pgxpool")
Rel(extractor, saApi, "HTTPS + JWT")
```

---

## L4 — Code (выборочные code-level блоки)

### 4.1. Pipeline orchestration (service/etl_pipeline.go)

```mermaid
classDiagram
    class EtlPipeline {
        -repo Repository
        -extractor Extractor
        -validator ValidationEngine
        -transformer Transformer
        -loader Loader
        -lock AdvisoryLock
        +Run(ctx) (RunID, error)
    }
    class Repository {
        <<interface>>
        +CreateRun(ctx, run) error
        +UpdateRunStatus(ctx, id, status, reason) error
        +InsertRejects(ctx, rows) error
        +UpsertMart(ctx, name, rows) error
    }
    class Extractor {
        <<interface>>
        +GetCurrentSnapshot(ctx) (LoadID, error)
        +StreamEntity(ctx, entity, snapshotID) (iter, error)
    }
    class ValidationEngine {
        <<interface>>
        +CheckBatch(entity, rows) []Violation
    }
    class Transformer {
        <<interface>>
        +BuildDemandHistory(ctx, runID, snapshotID) error
        +BuildCalculationInput(ctx, runID, snapshotID) error
        +BuildKpiDaily(ctx, runID, snapshotID) error
        +BuildMasterCurrent(ctx, runID, snapshotID) error
        +BuildSupplierScorecard(ctx, runID, snapshotID) error
    }
    class Loader {
        <<interface>>
        +UpsertAndFlip(ctx, runID, mart MartName) error
    }
    EtlPipeline --> Repository
    EtlPipeline --> Extractor
    EtlPipeline --> ValidationEngine
    EtlPipeline --> Transformer
    EtlPipeline --> Loader
```

### 4.2. extractor

```mermaid
classDiagram
    class Client {
        -httpClient http.Client
        -baseURL string
        -tokenSrc TokenSource
        -retryPolicy RetryPolicy
        +Do(req) (resp, error)
    }
    class SnapshotsClient {
        -client *Client
        +GetCurrent(ctx) (Snapshot, error)
    }
    class EntitiesClient {
        -client *Client
        +Stream(ctx, entity, snapshotID, etag) (NDJSONReader, error)
    }
    class TokenSource {
        <<interface>>
        +Token(ctx) string
    }
    Client --> TokenSource
    SnapshotsClient --> Client
    EntitiesClient --> Client
```

### 4.3. validation (reuse Модуля 1)

```mermaid
classDiagram
    class EngineAdapter {
        -base validation.Engine
        +CheckBatch(entity, rows) []Violation
    }
    class BuiltinFKExists
    class BuiltinUniqueBKey
    class BuiltinAggregateSum
    class BuiltinRefIntegrity
    class BuiltinNullRequired
    EngineAdapter ..> BuiltinFKExists
    EngineAdapter ..> BuiltinUniqueBKey
    EngineAdapter ..> BuiltinAggregateSum
    EngineAdapter ..> BuiltinRefIntegrity
    EngineAdapter ..> BuiltinNullRequired
```

> Engine движка из Модуля 1 (`internal/features/data_export/validation`) импортируется как библиотека. Builtin-чеки регистрируются адаптером.
