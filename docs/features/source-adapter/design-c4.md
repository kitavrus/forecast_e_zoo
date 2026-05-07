# Design C4 — source-adapter

C4-модель в 4 уровнях детализации (Mermaid). Уровни 3 и 4 ограничены фичей `data_export`.

---

## L1 — System Context

```mermaid
C4Context
title System Context — source-adapter (Модуль 1 MVP e_zoo)

Person(it_ezoo, "IT E-Zoo", "Оператор. Дёргает /admin/* для запуска/диагностики load-а.")
Person(cat_mgr, "Категорийный менеджер", "Не работает напрямую с адаптером, но видит данные через Replenishment / X-Flow UI.")

System_Boundary(b1, "e_zoo MVP-пайплайн") {
  System(adapter, "source-adapter", "Pull из ERP, валидация, snapshot, REST с JWT.")
  System_Ext(xflow, "X-Flow ETL", "Строит витрины из сырых данных адаптера.")
  System_Ext(replen, "Replenishment", "Прогноз спроса + калькулятор заказа. Читает витрины X-Flow.")
}

System_Ext(erp, "ERP клиента (E-Zoo)", "1С / SAP / custom — стек не выбран. Источник правды для master + facts.")

Rel(adapter, erp, "Pull (REST/SOAP/SFTP — Q-002/Q-003)", "HTTP/HTTPS, cron")
Rel(xflow, adapter, "GET /v1/{entity}", "HTTPS + JWT")
Rel(it_ezoo, adapter, "POST /admin/loads, GET /admin/reject-log", "HTTPS + JWT")
Rel(replen, xflow, "GET /v1/marts/*", "HTTPS")
```

## L2 — Containers

```mermaid
C4Container
title Containers — source-adapter

Person(it_ezoo, "IT E-Zoo")
System_Ext(erp, "ERP клиента")
System_Ext(xflow, "X-Flow ETL")

System_Boundary(b1, "source-adapter") {
  Container(api, "HTTP API", "Go 1.26 + Fiber v3", "REST с JWT. /v1/* (read), /admin/* (operate). Inline NDJSON, async exports.")
  Container(scheduler, "Scheduler", "Go + go-co-op/gocron/v2", "Внутрипроцессный cron. Триггер ежедневной выгрузки.")
  Container(loader, "Loader", "Go", "Координирует чтение из ERP, валидацию, UPSERT, atomic flip snapshot.")
  Container(validator, "Validator", "Go + YAML rules", "Severity-движок (critical|soft) на основе validation_rules.yaml.")
  Container(reader, "ERP reader", "Go", "Реализация SourceReader. HTTP-клиент с retry/backoff.")
  Container(exports, "Exports worker", "Go + local FS", "Async parquet/NDJSON >50 MB. Хранит на /var/exports/{id}.")
  ContainerDb(pg, "PostgreSQL 18", "pgx/v5 + pgxpool", "Master + facts (partitioned by event_date), loads, snapshot_pointer, reject_log, audit_access.")
  ContainerDb(fs, "Local FS", "/var/exports/", "Async export файлы, retention 24ч.")
}

Rel(it_ezoo, api, "POST /admin/loads", "HTTPS + JWT")
Rel(xflow, api, "GET /v1/products?since=...", "HTTPS + JWT, NDJSON")
Rel(scheduler, loader, "trigger daily-load", "in-process")
Rel(loader, reader, "Read{Entity}(ctx, since)", "in-process")
Rel(reader, erp, "Pull batches", "HTTPS")
Rel(loader, validator, "Validate(rows)", "in-process")
Rel(loader, pg, "UPSERT, INSERT reject_log, advisory lock, snapshot flip", "TCP")
Rel(api, pg, "SELECT с фильтром load_id = current_load_id", "TCP")
Rel(api, fs, "ServeStatic(/exports/{id})", "Fiber static")
Rel(exports, pg, "SELECT snapshot rows for export_id", "TCP")
Rel(exports, fs, "Write parquet/NDJSON", "FS")
```

## L3 — Components внутри `internal/features/data_export`

```mermaid
C4Component
title Components — feature `data_export`

Container_Ext(pg, "PostgreSQL 18")
Container_Ext(erp, "ERP клиента")
Container_Ext(fs, "Local FS")

Container_Boundary(c1, "internal/features/data_export") {
  Component(router, "router", "Fiber v3 router", "Маршруты + JWT middleware + audit middleware (для /admin/*).")
  Component(handler, "handler", "Fiber handlers", "Один handler-файл на сущность. Парсит query, дёргает service.")
  Component(service, "service", "Go business logic", "Координирует repository, validator, source reader, snapshot.")
  Component(repo, "repository", "pgxpool", "Все SQL через go:embed. Без ORM.")
  Component(loader_svc, "loader (service)", "Go", "Pipeline: Read → Map → Validate → UPSERT → Flip.")
  Component(validator_svc, "validators", "Go + YAML", "Engine + rules. Возвращает severity per row.")
  Component(scheduler_svc, "scheduler", "go-co-op/gocron/v2", "Cron entry, регистрирует daily-load job.")
  Component(snapshot_svc, "snapshot", "Go + SQL", "Atomic flip snapshot_pointer + advisory lock helper.")
  Component(exports_svc, "exports", "Go", "Async writer parquet/NDJSON через интерфейс ExportsStorage.")
  Component(mappers, "mappers", "Go", "ERP DTO → internal domain model.")
  Component(models, "models/dto", "Go structs", "Все entity + request/response DTO.")
  Component(sqls, "sqls", "go:embed *.sql", "Все запросы и миграции.")
  Component(audit, "audit", "Go", "Writer для audit_access (только /admin/*).")
}

Rel(router, handler, "calls")
Rel(handler, service, "calls")
Rel(service, repo, "calls")
Rel(service, loader_svc, "trigger")
Rel(loader_svc, validator_svc, "validate")
Rel(loader_svc, snapshot_svc, "flip")
Rel(loader_svc, mappers, "map ERP→domain")
Rel(loader_svc, repo, "UPSERT + reject_log")
Rel(scheduler_svc, loader_svc, "tick → start load")
Rel(repo, sqls, "embed.FS")
Rel(repo, pg, "pgxpool.Query/Exec")
Rel(loader_svc, erp, "via SourceReader interface")
Rel(exports_svc, fs, "WriteFile")
Rel(audit, repo, "INSERT audit_access")
```

## L4 — Code (key types и связи)

```mermaid
classDiagram
  class SourceReader {
    <<interface>>
    +ReadProducts(ctx, since) Iterator~ProductDTO~
    +ReadCategories(ctx, since) Iterator~CategoryDTO~
    +ReadLocations(ctx, since) Iterator~LocationDTO~
    +ReadStoreAssortment(ctx, since) Iterator~StoreAssortmentDTO~
    +ReadSuppliers(ctx, since) Iterator~SupplierDTO~
    +ReadSupplySpecs(ctx, since) Iterator~SupplySpecDTO~
    +ReadPromos(ctx, since) Iterator~PromoDTO~
    +ReadOrderRules(ctx, since) Iterator~OrderRuleDTO~
    +ReadSupplyPlans(ctx, since) Iterator~SupplyPlanDTO~
    +ReadReceiptLines(ctx, since) Iterator~ReceiptLineDTO~
    +ReadStockSnapshots(ctx, since) Iterator~StockSnapshotDTO~
    +ReadStockMovements(ctx, since) Iterator~StockMovementDTO~
    +ReadSupplierStockSnapshot(ctx, since) Iterator~SupplierStockDTO~
  }

  class SourceAuth {
    <<interface>>
    +Apply(req *http.Request) error
  }

  class ExportsStorage {
    <<interface>>
    +Write(ctx, exportID, reader) (path string, size int64, err)
    +Reader(ctx, exportID) (io.ReadCloser, err)
    +Delete(ctx, exportID) error
  }

  class Loader {
    -reader SourceReader
    -repo Repository
    -validator ValidatorEngine
    -snapshot SnapshotService
    +Run(ctx, loadID) error
  }

  class SnapshotService {
    -repo Repository
    +AcquireLock(ctx) (release func(), error)
    +Flip(ctx, loadID) error
    +Current(ctx) (loadID UUID, err)
  }

  class ValidatorEngine {
    -rules []Rule
    +Validate(entity, row) Severity
  }

  class Repository {
    -pool *pgxpool.Pool
    +UpsertProducts(ctx, loadID, batch) error
    +InsertRejectLog(ctx, loadID, entry) error
    +InsertAuditAccess(ctx, entry) error
    +CurrentSnapshot(ctx) (loadID UUID, err)
    +InsertMasterChangeLog(ctx, loadID, events) error
  }

  class ScheduledTrigger {
    -loader Loader
    -snapshot SnapshotService
    +RegisterDailyLoad(scheduler) error
  }

  Loader --> SourceReader
  Loader --> Repository
  Loader --> ValidatorEngine
  Loader --> SnapshotService
  SnapshotService --> Repository
  ScheduledTrigger --> Loader
  ScheduledTrigger --> SnapshotService
```

## Граничные принципы

- **Один handler — один файл.** `handler/products.go`, `handler/admin_loads.go` и т.д.
- **Repository — единственный owner SQL.** Все строки SQL — через `go:embed`. Никаких inline-строк
  в service-слое.
- **Service ничего не знает о Fiber.** Принимает `context.Context` + типизированные args, возвращает
  domain-объекты или sentinel-ошибки.
- **Loader работает только через интерфейсы** — `SourceReader`, `Repository`, `ValidatorEngine`,
  `SnapshotService`. Это упрощает unit-тесты с моками.
