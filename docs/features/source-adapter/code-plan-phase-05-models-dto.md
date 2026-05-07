# Phase 05: Models / DTO

**Цель:** создать доменные модели (внутренние) и DTO (API request/response) для всех 16 сущностей контракта + admin-эндпоинтов + общих типов (cursor pagination, etag, error details). Без логики — только структуры с json/db тегами и валидаторами `validate:"..."`. Базовый pattern — две папки: `models/` (домен) и `models/dto/` (наружу/внутрь по HTTP).

**Commit:** `feat(data_export/models): доменные модели и DTO для всех 16 сущностей и admin API`

---

## Files to CREATE

### Доменные модели (`internal/features/data_export/models/`)

- `models/load.go` — `Load{LoadID, StartedAt, CommittedAt, FailedAt, Status, FailedReason, Source, LinesTotal, LinesFailed, EntityStats}`. Тип `LoadStatus` (string-enum: running, committed, failed, aborted).
- `models/snapshot.go` — `SnapshotPointer{CurrentLoadID, PreviousLoadID, CommittedAt}`.
- `models/reject_log.go` — `RejectEntry{ID, LoadID, Entity, Payload, Errors, Severity, CreatedAt}`. Тип `Severity` (critical, soft).
- `models/audit.go` — `AuditAccessEntry{ID, At, ActorRole, ActorSub, Method, Path, Status, TraceID}`.
- `models/master.go` — `Product`, `ProductBarcode`, `Category`, `Location`, `Supplier`, `SupplyPrec`/`SupplySpec`, `Promo`, `OrderRule`, `SupplyPlan`, `StoreAssortment`, `StoreAssortmentLifecycleEvent`, `MasterChangeLogEntry`. По одному структу на сущность с `LoadID` полем.
- `models/facts.go` — `ReceiptLine`, `LocationStockSnapshot`, `StockMovement`, `SupplierStockSnapshot`. Поля `EventDate`/`EventTime` обязательны.
- `models/cursor.go` — `Cursor{LoadID uuid.UUID; AfterPK string}`. Методы `Encode()` (base64 JSON) / `Decode(s string) error`.

### DTO (`internal/features/data_export/models/dto/`)

- `dto/common.go` — `PageRequest{Cursor string; Limit int `validate:"min=1,max=10000"`}`, `PageResponse[T any]{Items []T; NextCursor string}`, `IfNoneMatch string` хелпер.
- `dto/admin.go`:
  - `PostLoadRequest{Source string `validate:"required"`}`, `PostLoadResponse{LoadID, Status, StartedAt}`.
  - `PostLoadRetryRequest{LoadID uuid.UUID}`, `PostLoadRetryResponse{NewLoadID, OriginalLoadID}`.
  - `GetLoadResponse{Load, EntityProgress []EntityStat}`.
  - `GetRejectLogRequest{LoadID, Entity, Severity, Limit, Cursor}`, `GetRejectLogResponse{Items []RejectEntry, NextCursor}`.
- `dto/snapshots.go` — `GetSnapshotsCurrentResponse{SnapshotID uuid.UUID, CommittedAt, Entities []EntitySummary}`.
- `dto/master.go` — `GetProductsResponse`, `GetProductBarcodesResponse`, `GetCategoryResponse`, `GetLocationResponse`, `GetSupplierResponse`, `GetSupplySpecResponse`, `GetPromoResponse`, `GetOrderRuleResponse`, `GetSupplyPlanResponse`, `GetStoreAssortmentResponse`, `GetStoreAssortmentLifecycleEventsResponse`, `GetMasterChangeLogResponse`. Все — обёртка `PageResponse[T]`.
- `dto/facts.go` — `GetReceiptLineResponse`, `GetLocationStockSnapshotResponse`, `GetStockMovementResponse`, `GetSupplierStockSnapshotResponse`. Все — `PageResponse[T]` + обязательный фильтр `EventDate`.
- `dto/exports.go` — `PostExportRequest{Entity string `validate:"oneof=receipt_line stock_movement ..."` Format string `validate:"oneof=ndjson parquet"`; Filters map[string]string; SnapshotID uuid.UUID}`. `PostExportResponse{ExportID uuid.UUID, Status string, Location string}`. `GetExportResponse` (через `GET /v1/exports/{id}` — кроме случая, когда нужно вернуть бинарь).
- `dto/healthz.go` — `HealthzResponse{Status, DB string, Version, BuildTime}`.

### Маппинг models ↔ DTO

- `models/mappers.go` — функции `Product.ToDTO()`, `DTO.ToProduct()` и т.д. Только перекладывание полей, без логики.

### Тесты

- `models/cursor_test.go` — `TestCursor_EncodeDecode_Roundtrip`, `TestCursor_DecodeBadBase64_Error`, `TestCursor_DecodeBadJSON_Error`.
- `models/dto/dto_validate_test.go` — валидация: `TestPostExportRequest_BadFormat`, `TestPageRequest_LimitTooLarge`, `TestPostExportRequest_HappyPath`.

## Files to MODIFY

- `go.mod` — добавить `github.com/go-playground/validator/v10` (используется в фазе 07 формальных валидаторов и здесь — для аннотаций).

## SQL/Migrations

— нет.

## Run after

```bash
go mod tidy
make build
make test-unit
make lint
```

## Tests in this phase

- `TestCursor_EncodeDecode_Roundtrip`
- `TestCursor_DecodeBadBase64_Error`
- `TestCursor_DecodeBadJSON_Error`
- `TestPostExportRequest_BadFormat`
- `TestPostExportRequest_HappyPath`
- `TestPageRequest_LimitTooLarge`

## Definition of Done

- [ ] Все 16 доменных моделей созданы с `db:"..."` тегами для pgx.
- [ ] Все DTO созданы с `json:"..."` и `validate:"..."`.
- [ ] `Cursor.Encode/Decode` симметричны.
- [ ] Маппинг model ↔ DTO покрывает все поля без потерь.
- [ ] `make build` зелёный.
- [ ] `make test-unit` зелёный.
- [ ] `make lint` без ошибок.
- [ ] Коммит атомарный, сообщение `feat(data_export/models): ...`.
