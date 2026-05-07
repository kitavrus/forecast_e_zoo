# Design Go Layers — source-adapter

Структура Go-кода фичи `data_export` (snake_case). Module path: `github.com/Kitavrus/e_zoo`.

---

## 1. Дерево репозитория (целевое)

```
e_zoo/
├── cmd/
│   └── source-adapter/
│       └── main.go                       # entrypoint: env → app.New → app.Run
├── internal/
│   ├── app/
│   │   └── app.go                        # сборка: pgxpool, scheduler, fiber, mw, handlers
│   ├── routers/
│   │   └── routers.go                    # регистрация всех маршрутов фичей
│   ├── middleware/
│   │   ├── jwt.go                        # JWT HS256/RS256 verify, claims в Locals
│   │   ├── role.go                       # Role-check (x-flow-etl, admin-cli, it-read)
│   │   ├── audit.go                      # INSERT audit_access (только /admin/*)
│   │   ├── recover.go                    # panic → 500 + ErrorResponseJSON
│   │   └── request_id.go                 # X-Request-Id, traceId
│   ├── config/
│   │   └── config.go                     # env-config через kelseyhightower/envconfig
│   └── features/
│       └── data_export/
│           ├── constants/
│           │   └── constants.go          # entity names, limits, advisory lock keys
│           ├── handler/
│           │   ├── healthz.go
│           │   ├── snapshots.go
│           │   ├── products.go
│           │   ├── product_barcodes.go
│           │   ├── category.go
│           │   ├── location.go
│           │   ├── supplier.go
│           │   ├── store_assortment.go
│           │   ├── store_assortment_lifecycle.go
│           │   ├── master_change_log.go
│           │   ├── supplier_stock.go
│           │   ├── supply_spec.go
│           │   ├── promo.go
│           │   ├── supply_plan.go
│           │   ├── order_rule.go
│           │   ├── exports.go
│           │   ├── admin_loads.go
│           │   └── admin_reject_log.go
│           ├── service/
│           │   ├── snapshot.go           # Current(), Flip(loadID), AcquireLock()
│           │   ├── loader.go             # Loader pipeline: Read→Validate→UPSERT→Flip
│           │   ├── reader.go             # SourceReader интерфейс
│           │   ├── reader_erp_http.go    # HTTP-impl (placeholder под Q-001/Q-002)
│           │   ├── reader_inmem.go       # in-memory импл для тестов / dev
│           │   ├── exports.go            # CreateExport, GetExport, worker loop
│           │   ├── audit.go              # AuditWriter
│           │   ├── master_entities.go    # ListProducts, ListCategory, ListLocation, ListSupplier
│           │   ├── facts.go              # ListReceiptLines, ListStockMovement, ...
│           │   ├── store_assortment.go
│           │   ├── master_change_log.go
│           │   └── admin_loads.go
│           ├── repository/
│           │   ├── repository.go         # Repository struct + ctor
│           │   ├── snapshot.go
│           │   ├── loads.go
│           │   ├── reject_log.go
│           │   ├── audit.go
│           │   ├── exports.go
│           │   ├── master.go             # все master-сущности
│           │   ├── facts.go              # все факт-сущности
│           │   ├── store_assortment.go
│           │   ├── master_change_log.go
│           │   └── healthz.go
│           ├── models/
│           │   └── dto/
│           │       ├── product.go
│           │       ├── product_barcode.go
│           │       ├── category.go
│           │       ├── location.go
│           │       ├── supplier.go
│           │       ├── store_assortment.go
│           │       ├── store_assortment_lifecycle.go
│           │       ├── supply_spec.go
│           │       ├── promo.go
│           │       ├── order_rule.go
│           │       ├── supply_plan.go
│           │       ├── receipt_line.go
│           │       ├── location_stock_snapshot.go
│           │       ├── stock_movement.go
│           │       ├── supplier_stock_snapshot.go
│           │       ├── master_change_log.go
│           │       ├── load.go
│           │       ├── snapshot.go
│           │       ├── reject_log.go
│           │       ├── export.go
│           │       └── audit.go
│           ├── mappers/
│           │   ├── erp_to_domain.go      # ERP DTO → domain
│           │   └── domain_to_response.go # domain → API response (JSON tags)
│           ├── validators/
│           │   ├── engine.go             # ValidatorEngine
│           │   ├── rules.go              # типы Rule, Severity
│           │   ├── rules_loader.go       # YAML loader
│           │   └── builtin.go            # built-in checks (negative_qty, future_event_time, ...)
│           ├── exports_storage/
│           │   ├── storage.go            # ExportsStorage интерфейс
│           │   └── local_fs.go           # LocalFSStorage (impl)
│           ├── scheduler/
│           │   └── scheduler.go          # gocron registration
│           ├── router/
│           │   └── router.go             # RegisterRoutes(app *fiber.App, deps)
│           └── sqls/
│               ├── sqls.go               # go:embed FS, helpers
│               ├── migrations/
│               │   ├── 0001_init_loads_snapshot.up.sql
│               │   ├── 0002_init_master.up.sql
│               │   ├── 0003_init_store_assortment.up.sql
│               │   ├── 0004_init_facts_partitioned.up.sql
│               │   ├── 0005_init_master_change_log.up.sql
│               │   ├── 0006_init_reject_log.up.sql
│               │   ├── 0007_init_audit_access.up.sql
│               │   ├── 0008_init_exports.up.sql
│               │   └── *.down.sql
│               └── queries/
│                   ├── snapshot_select_current.sql
│                   ├── snapshot_flip.sql
│                   ├── loads_insert.sql
│                   ├── loads_update_status.sql
│                   ├── loads_select_by_id.sql
│                   ├── products_upsert.sql
│                   ├── products_select.sql
│                   ├── reject_log_insert.sql
│                   ├── audit_access_insert.sql
│                   ├── exports_insert.sql
│                   ├── exports_claim.sql
│                   └── ... (по таблице)
├── pkg/
│   ├── errorspkg/
│   │   ├── errors.go                     # sentinel-ошибки + ErrorResponseJSON
│   │   └── support_codes.go              # supportMessage коды
│   └── pgxutil/
│       └── pool.go                       # pgxpool helpers
├── config/
│   ├── validation_rules.yaml             # severity rules
│   └── master_tracked_fields.yaml        # tracked fields для master_change_log
├── migrations/                           # symlink на internal/.../sqls/migrations (или дубликат для CLI)
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── Makefile                              # build, test, lint, migrate
└── README.md
```

## 2. Ключевые интерфейсы

### 2.1. `SourceReader`

```go
package data_export

type SourceReader interface {
    ReadProducts(ctx context.Context, since time.Time) ProductIterator
    ReadProductBarcodes(ctx context.Context, since time.Time) ProductBarcodeIterator
    ReadCategories(ctx context.Context, since time.Time) CategoryIterator
    ReadLocations(ctx context.Context, since time.Time) LocationIterator
    ReadSuppliers(ctx context.Context, since time.Time) SupplierIterator
    ReadStoreAssortment(ctx context.Context, since time.Time) StoreAssortmentIterator
    ReadStoreAssortmentLifecycle(ctx context.Context, since time.Time) StoreAssortmentLifecycleIterator
    ReadSupplySpecs(ctx context.Context, since time.Time) SupplySpecIterator
    ReadPromos(ctx context.Context, since time.Time) PromoIterator
    ReadOrderRules(ctx context.Context, since time.Time) OrderRuleIterator
    ReadSupplyPlans(ctx context.Context, since time.Time) SupplyPlanIterator
    ReadReceiptLines(ctx context.Context, since time.Time) ReceiptLineIterator
    ReadLocationStockSnapshots(ctx context.Context, since time.Time) LocationStockIterator
    ReadStockMovements(ctx context.Context, since time.Time) StockMovementIterator
    ReadSupplierStockSnapshot(ctx context.Context, since time.Time) (SupplierStockIterator, bool)
    // bool = present. Если ERP не поддерживает — false, load не fail-ается (Q-009).
}

type ProductIterator interface {
    Next(ctx context.Context) bool
    Value() dto.Product
    Err() error
    Close() error
}
// аналогично для остальных сущностей
```

### 2.2. `SourceAuth`

```go
type SourceAuth interface {
    Apply(req *http.Request) error  // proставляет Bearer / mTLS cert / API-key
}

// Реализации зависят от Q-001:
//  - bearerAuth      (OAuth2 client_credentials, refresh внутри)
//  - mtlsAuth        (cert + key в config)
//  - apiKeyAuth      (X-API-Key header + IP allowlist enforcement на стороне ERP)
//  - noAuth          (для in-memory dev backend)
```

### 2.3. `ExportsStorage`

```go
type ExportsStorage interface {
    Write(ctx context.Context, exportID uuid.UUID, format string, src io.Reader) (path string, sizeBytes int64, err error)
    Open(ctx context.Context, exportID uuid.UUID) (io.ReadCloser, error)
    Path(exportID uuid.UUID, format string) string
    Delete(ctx context.Context, exportID uuid.UUID) error
}
// MVP impl: localFSStorage{baseDir: "/var/exports"}
// Будущая impl: s3Storage{bucket, prefix} — без изменений в loader/handler.
```

### 2.4. `Repository` (интерфейс уровня service)

```go
type Repository interface {
    // snapshot/loads
    InsertLoad(ctx context.Context, l dto.Load) error
    UpdateLoadStatus(ctx context.Context, id uuid.UUID, status string, finishedAt *time.Time, summary []byte) error
    SelectLoadByID(ctx context.Context, id uuid.UUID) (dto.Load, error)
    SelectCommittedLoads(ctx context.Context, limit int) ([]dto.Load, error)
    SelectSnapshotPointer(ctx context.Context) (dto.SnapshotPointer, error)
    FlipSnapshotPointer(ctx context.Context, newLoadID uuid.UUID) error
    TryAdvisoryLock(ctx context.Context, key string) (acquired bool, release func() error, err error)

    // master entities
    UpsertProductsBatch(ctx context.Context, loadID uuid.UUID, batch []dto.Product) error
    SelectProducts(ctx context.Context, loadID uuid.UUID, since time.Time, cursor string, limit int) ([]dto.Product, error)
    // ... аналогично для product_barcodes, category, location, supplier, supply_spec, promo, order_rule, supply_plan

    // store_assortment
    UpsertStoreAssortmentBatch(ctx context.Context, loadID uuid.UUID, batch []dto.StoreAssortment) error
    SelectStoreAssortment(ctx context.Context, loadID uuid.UUID, filters AssortmentFilters) ([]dto.StoreAssortment, error)
    SelectStoreAssortmentLifecycle(ctx context.Context, loadID uuid.UUID, since time.Time) ([]dto.StoreAssortmentLifecycle, error)

    // facts (partitioned)
    InsertReceiptLinesBatch(ctx context.Context, loadID uuid.UUID, batch []dto.ReceiptLine) error
    InsertLocationStockSnapshotBatch(ctx context.Context, loadID uuid.UUID, batch []dto.LocationStockSnapshot) error
    InsertStockMovementsBatch(ctx context.Context, loadID uuid.UUID, batch []dto.StockMovement) error
    InsertSupplierStockSnapshotBatch(ctx context.Context, loadID uuid.UUID, batch []dto.SupplierStockSnapshot) error
    SelectReceiptLines(ctx context.Context, loadID uuid.UUID, filters FactFilters) ([]dto.ReceiptLine, error)
    // ... аналогично

    // master_change_log
    InsertMasterChangeLog(ctx context.Context, loadID uuid.UUID, events []dto.MasterChangeLogEvent) error
    SelectMasterChangeLog(ctx context.Context, filters ChangeLogFilters) ([]dto.MasterChangeLogEvent, error)

    // reject_log
    InsertRejectLog(ctx context.Context, loadID uuid.UUID, entry dto.RejectLogEntry) error
    SelectRejectLog(ctx context.Context, filters RejectLogFilters) ([]dto.RejectLogEntry, error)
    CountRejectByEntity(ctx context.Context, loadID uuid.UUID) ([]dto.RejectSummary, error)

    // audit
    InsertAuditAccess(ctx context.Context, e dto.AuditAccessEntry) error

    // exports
    InsertExport(ctx context.Context, e dto.Export) error
    SelectExportByID(ctx context.Context, id uuid.UUID) (dto.Export, error)
    ClaimNextExport(ctx context.Context) (dto.Export, bool, error)
    UpdateExportStatus(ctx context.Context, id uuid.UUID, status string, sizeBytes *int64, errStr *string) error

    // healthz
    Ping(ctx context.Context) error
}
```

## 3. DTO (полный список полей по сущностям)

### 3.1. Master entities

```go
// dto/product.go
type Product struct {
    ProductID            string     `db:"product_id" json:"product_id"`
    Name                 string     `db:"name" json:"name"`
    Brand                *string    `db:"brand" json:"brand,omitempty"`
    Manufacturer         *string    `db:"manufacturer" json:"manufacturer,omitempty"`
    CategoryID           string     `db:"category_id" json:"category_id"`
    CategoryPath         string     `db:"category_path" json:"category_path"`
    WeightKg             *float64   `db:"weight_kg" json:"weight_kg,omitempty"`
    PalletQty            *int       `db:"pallet_qty" json:"pallet_qty,omitempty"`
    ShelfLifeDays        *int       `db:"shelf_life_days" json:"shelf_life_days,omitempty"`
    StorageTempMin       *float64   `db:"storage_temp_min" json:"storage_temp_min,omitempty"`
    StorageTempMax       *float64   `db:"storage_temp_max" json:"storage_temp_max,omitempty"`
    RequiresPrescription bool       `db:"requires_prescription" json:"requires_prescription"`
    IsDangerousGoods     bool       `db:"is_dangerous_goods" json:"is_dangerous_goods"`
    Status               string     `db:"status" json:"status"` // active|archived
    CreatedAt            time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
    LoadID               uuid.UUID  `db:"load_id" json:"load_id"`
}

// dto/product_barcode.go
type ProductBarcode struct {
    Barcode       string    `db:"barcode" json:"barcode"`
    ProductID     string    `db:"product_id" json:"product_id"`
    PackQty       int       `db:"pack_qty" json:"pack_qty"`
    IsPrimary     bool      `db:"is_primary" json:"is_primary"`
    CountryOrigin *string   `db:"country_origin" json:"country_origin,omitempty"`
    CreatedAt     time.Time `db:"created_at" json:"created_at"`
    UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
    LoadID        uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/category.go
type Category struct {
    CategoryID string    `db:"category_id" json:"category_id"`
    ParentID   *string   `db:"parent_id" json:"parent_id,omitempty"`
    Level      int16     `db:"level" json:"level"`
    Name       string    `db:"name" json:"name"`
    CreatedAt  time.Time `db:"created_at" json:"created_at"`
    UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
    LoadID     uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/location.go
type Location struct {
    LocationID string    `db:"location_id" json:"location_id"`
    Type       string    `db:"type" json:"type"` // STORE | DC | DARK_STORE
    Name       string    `db:"name" json:"name"`
    Address    *string   `db:"address" json:"address,omitempty"`
    City       *string   `db:"city" json:"city,omitempty"`
    Region     *string   `db:"region" json:"region,omitempty"`
    OpenedAt   *time.Time `db:"opened_at" json:"opened_at,omitempty"`
    ClosedAt   *time.Time `db:"closed_at" json:"closed_at,omitempty"`
    Status     string    `db:"status" json:"status"`
    CreatedAt  time.Time `db:"created_at" json:"created_at"`
    UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
    LoadID     uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/supplier.go
type Supplier struct {
    SupplierID    string    `db:"supplier_id" json:"supplier_id"`
    Name          string    `db:"name" json:"name"`
    INN           *string   `db:"inn" json:"inn,omitempty"`
    GLN           *string   `db:"gln" json:"gln,omitempty"`
    PaymentTerms  *string   `db:"payment_terms" json:"payment_terms,omitempty"`
    EDIProfile    *string   `db:"edi_profile" json:"edi_profile,omitempty"`
    Status        string    `db:"status" json:"status"`
    CreatedAt     time.Time `db:"created_at" json:"created_at"`
    UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
    LoadID        uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/store_assortment.go
type StoreAssortment struct {
    LocationID      string    `db:"location_id" json:"location_id"`
    ProductID       string    `db:"product_id" json:"product_id"`
    LifecycleState  string    `db:"lifecycle_state" json:"lifecycle_state"` // active | phasing_in | phasing_out | inactive
    AssortmentClass *string   `db:"assortment_class" json:"assortment_class,omitempty"`
    PriceMin        *float64  `db:"price_min" json:"price_min,omitempty"`
    PriceMax        *float64  `db:"price_max" json:"price_max,omitempty"`
    EffectiveFrom   time.Time `db:"effective_from" json:"effective_from"`
    EffectiveTo     *time.Time `db:"effective_to" json:"effective_to,omitempty"`
    CreatedAt       time.Time `db:"created_at" json:"created_at"`
    UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
    LoadID          uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/store_assortment_lifecycle.go (внутренняя domain DTO для repository/loader)
type StoreAssortmentLifecycle struct {
    EventID        uuid.UUID `db:"event_id" json:"event_id"`
    LocationID     string    `db:"location_id" json:"location_id"`
    ProductID      string    `db:"product_id" json:"product_id"`
    TransitionType string    `db:"transition_type" json:"transition_type"`
    FromState      *string   `db:"from_state" json:"from_state,omitempty"`
    ToState        string    `db:"to_state" json:"to_state"`
    TransitionAt   time.Time `db:"transition_at" json:"transition_at"`
    Reason         *string   `db:"reason" json:"reason,omitempty"`
    EvidencePath   *string   `db:"evidence_path" json:"evidence_path,omitempty"`
    LoadID         uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/store_assortment_lifecycle.go (response DTO для GET /v1/store_assortment/lifecycle_events)
// ADR-016 (Q-016): зафиксированный публичный контракт события lifecycle для X-Flow ETL.
// JSON Schema: additionalProperties: false; расширение eventType — forward-compatible
// через новую версию контракта.
type StoreAssortmentLifecycleEventResponse struct {
    EventID         string    `json:"eventId"`         // UUID
    EventType       string    `json:"eventType"`       // "started" | "stopped" | "promo_started" | "promo_stopped"
    LocationID      string    `json:"locationId"`
    ProductID       string    `json:"productId"`
    EffectiveAt     time.Time `json:"effectiveAt"`     // когда событие вступило в силу
    Reason          *string   `json:"reason,omitempty"` // напр. "out_of_stock", "promo_id=PR123"
    PromoID         *string   `json:"promoId,omitempty"`
    PriorState      *string   `json:"priorState,omitempty"`  // "active" | "inactive" | "promo"
    NewState        string    `json:"newState"`              // "active" | "inactive" | "promo"
    SourceLoadID    string    `json:"sourceLoadId"`
    CreatedAt       time.Time `json:"createdAt"`
}

// dto/supply_spec.go
type SupplySpec struct {
    SupplierID      string    `db:"supplier_id" json:"supplier_id"`
    ProductID       string    `db:"product_id" json:"product_id"`
    LocationID      string    `db:"location_id" json:"location_id"`
    Priority        int16     `db:"priority" json:"priority"`
    MinOrderQty     int       `db:"min_order_qty" json:"min_order_qty"`
    PurchasePrice   *float64  `db:"purchase_price" json:"purchase_price,omitempty"`
    Currency        string    `db:"currency" json:"currency"`
    LeadTimeDays    int       `db:"lead_time_days" json:"lead_time_days"`
    PackSize        int       `db:"pack_size" json:"pack_size"`
    EffectiveFrom   time.Time `db:"effective_from" json:"effective_from"`
    EffectiveTo     *time.Time `db:"effective_to" json:"effective_to,omitempty"`
    CreatedAt       time.Time `db:"created_at" json:"created_at"`
    UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
    LoadID          uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/promo.go
type Promo struct {
    PromoID            string    `db:"promo_id" json:"promo_id"`
    ProductID          string    `db:"product_id" json:"product_id"`
    LocationID         *string   `db:"location_id" json:"location_id,omitempty"`
    Type               string    `db:"type" json:"type"` // discount | bundle | loyalty_bonus | markdown | gift
    DiscountPct        *float64  `db:"discount_pct" json:"discount_pct,omitempty"`
    PromoPriceWithVAT  *float64  `db:"promo_price_with_vat" json:"promo_price_with_vat,omitempty"`
    DateFrom           time.Time `db:"date_from" json:"date_from"`
    DateTo             time.Time `db:"date_to" json:"date_to"`
    CreatedAt          time.Time `db:"created_at" json:"created_at"`
    UpdatedAt          time.Time `db:"updated_at" json:"updated_at"`
    LoadID             uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/order_rule.go
type OrderRule struct {
    RuleID         string    `db:"rule_id" json:"rule_id"`
    Scope          string    `db:"scope" json:"scope"` // product | category | supplier | global
    ScopeRef       *string   `db:"scope_ref" json:"scope_ref,omitempty"`
    LocationID     *string   `db:"location_id" json:"location_id,omitempty"`
    SafetyStockDays *float64 `db:"safety_stock_days" json:"safety_stock_days,omitempty"`
    ServiceLevelPct *float64 `db:"service_level_pct" json:"service_level_pct,omitempty"`
    OverrideMOQ    *int      `db:"override_moq" json:"override_moq,omitempty"`
    EffectiveFrom  time.Time `db:"effective_from" json:"effective_from"`
    EffectiveTo    *time.Time `db:"effective_to" json:"effective_to,omitempty"`
    LoadID         uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/supply_plan.go
type SupplyPlan struct {
    PlanID         string    `db:"plan_id" json:"plan_id"`
    SupplierID     string    `db:"supplier_id" json:"supplier_id"`
    LocationID     string    `db:"location_id" json:"location_id"`
    PlannedDate    time.Time `db:"planned_date" json:"planned_date"`
    SlotTime       *string   `db:"slot_time" json:"slot_time,omitempty"`
    CutoffAt       *time.Time `db:"cutoff_at" json:"cutoff_at,omitempty"`
    Status         string    `db:"status" json:"status"`
    LoadID         uuid.UUID `db:"load_id" json:"load_id"`
}
```

### 3.2. Facts (partitioned by event_date)

```go
// dto/receipt_line.go
type ReceiptLine struct {
    ReceiptID         string     `db:"receipt_id" json:"receipt_id"`
    LineNo            int        `db:"line_no" json:"line_no"`
    LocationID        string     `db:"location_id" json:"location_id"`
    ProductID         string     `db:"product_id" json:"product_id"`
    BarcodeScanned    *string    `db:"barcode_scanned" json:"barcode_scanned,omitempty"`
    Qty               float64    `db:"qty" json:"qty"`
    LineKind          string     `db:"line_kind" json:"line_kind"` // sale | refund | gift | promo_bonus
    UnitPriceBase     float64    `db:"unit_price_base" json:"unit_price_base"`
    UnitPricePaid     float64    `db:"unit_price_paid" json:"unit_price_paid"`
    DiscountAmount    float64    `db:"discount_amount" json:"discount_amount"`
    MarkdownPct       *float64   `db:"markdown_pct" json:"markdown_pct,omitempty"`
    PromoID           *string    `db:"promo_id" json:"promo_id,omitempty"`
    EventDate         time.Time  `db:"event_date" json:"event_date"` // partition key
    EventTime         time.Time  `db:"event_time" json:"event_time"`
    LoyaltyHash       *string    `db:"loyalty_hash" json:"loyalty_hash,omitempty"`
    ValidFrom         time.Time  `db:"valid_from" json:"valid_from"`
    ValidTo           time.Time  `db:"valid_to" json:"valid_to"` // 'infinity'
    SystemTimeFrom    time.Time  `db:"system_time_from" json:"system_time_from"`
    SystemTimeTo      time.Time  `db:"system_time_to" json:"system_time_to"`
    LoadID            uuid.UUID  `db:"load_id" json:"load_id"`
}

// dto/location_stock_snapshot.go
type LocationStockSnapshot struct {
    LocationID  string    `db:"location_id" json:"location_id"`
    ProductID   string    `db:"product_id" json:"product_id"`
    QtyOnHand   float64   `db:"qty_on_hand" json:"qty_on_hand"`
    QtyReserved float64   `db:"qty_reserved" json:"qty_reserved"`
    QtyAvailable float64  `db:"qty_available" json:"qty_available"`
    EventDate   time.Time `db:"event_date" json:"event_date"` // partition
    SnapshotAt  time.Time `db:"snapshot_at" json:"snapshot_at"`
    SystemTimeFrom time.Time `db:"system_time_from" json:"system_time_from"`
    SystemTimeTo   time.Time `db:"system_time_to" json:"system_time_to"`
    LoadID      uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/stock_movement.go
type StockMovement struct {
    MovementID     string    `db:"movement_id" json:"movement_id"`
    Type           string    `db:"type" json:"type"` // receiving | transfer | write_off | return_to_vendor | damage | inventory_adj
    LocationFrom   *string   `db:"location_from" json:"location_from,omitempty"`
    LocationTo     *string   `db:"location_to" json:"location_to,omitempty"`
    ProductID      string    `db:"product_id" json:"product_id"`
    Qty            float64   `db:"qty" json:"qty"`
    EventDate      time.Time `db:"event_date" json:"event_date"` // partition
    EventTime      time.Time `db:"event_time" json:"event_time"`
    SupplierID     *string   `db:"supplier_id" json:"supplier_id,omitempty"`
    Details        []byte    `db:"details" json:"details,omitempty"` // JSONB companion-данных
    SystemTimeFrom time.Time `db:"system_time_from" json:"system_time_from"`
    SystemTimeTo   time.Time `db:"system_time_to" json:"system_time_to"`
    LoadID         uuid.UUID `db:"load_id" json:"load_id"`
}

// dto/supplier_stock_snapshot.go
type SupplierStockSnapshot struct {
    SupplierID  string    `db:"supplier_id" json:"supplier_id"`
    ProductID   string    `db:"product_id" json:"product_id"`
    QtyAvailable float64  `db:"qty_available" json:"qty_available"`
    SnapshotAt  time.Time `db:"snapshot_at" json:"snapshot_at"`
    EventDate   time.Time `db:"event_date" json:"event_date"`
    LoadID      uuid.UUID `db:"load_id" json:"load_id"`
}
```

### 3.3. Служебные

```go
// dto/load.go
type Load struct {
    ID               uuid.UUID  `db:"id" json:"id"`
    StartedAt        time.Time  `db:"started_at" json:"started_at"`
    FinishedAt       *time.Time `db:"finished_at" json:"finished_at,omitempty"`
    Status           string     `db:"status" json:"status"` // running | committed | failed
    Source           string     `db:"source" json:"source"` // erp_e_zoo | manual | retry
    EntitiesSummary  []byte     `db:"entities_summary" json:"entities_summary,omitempty"` // JSONB
    FailureReason    *string    `db:"failure_reason" json:"failure_reason,omitempty"`
    ParentLoadID     *uuid.UUID `db:"parent_load_id" json:"parent_load_id,omitempty"`
}

// dto/snapshot.go
type SnapshotPointer struct {
    CurrentLoadID  *uuid.UUID `db:"current_load_id" json:"current_load_id,omitempty"`
    PreviousLoadID *uuid.UUID `db:"previous_load_id" json:"previous_load_id,omitempty"`
    CommittedAt    *time.Time `db:"committed_at" json:"committed_at,omitempty"`
}

// dto/reject_log.go
type RejectLogEntry struct {
    ID         uuid.UUID `db:"id" json:"id"`
    LoadID     uuid.UUID `db:"load_id" json:"load_id"`
    Entity     string    `db:"entity" json:"entity"`
    PKValue    []byte    `db:"pk_value" json:"pk_value"` // JSONB
    Severity   string    `db:"severity" json:"severity"` // critical | soft
    Reason     string    `db:"reason" json:"reason"`
    Raw        []byte    `db:"raw" json:"raw"` // JSONB исходного ERP DTO
    DetectedAt time.Time `db:"detected_at" json:"detected_at"`
}

type RejectSummary struct {
    Entity   string `db:"entity" json:"entity"`
    Severity string `db:"severity" json:"severity"`
    Count    int64  `db:"count" json:"count"`
}

// dto/audit.go
type AuditAccessEntry struct {
    ID         uuid.UUID `db:"id" json:"id"`
    Requester  string    `db:"requester" json:"requester"` // JWT issuer/sub
    Endpoint   string    `db:"endpoint" json:"endpoint"`
    Method     string    `db:"method" json:"method"`
    Query      []byte    `db:"query" json:"query"` // JSONB
    BytesOut   int64     `db:"bytes_out" json:"bytes_out"`
    StatusCode int       `db:"status_code" json:"status_code"`
    Ts         time.Time `db:"ts" json:"ts"`
}

// dto/export.go
type Export struct {
    ID         uuid.UUID  `db:"id" json:"id"`
    Entity     string     `db:"entity" json:"entity"`
    SnapshotID uuid.UUID  `db:"snapshot_id" json:"snapshot_id"`
    Format     string     `db:"format" json:"format"` // parquet | ndjson
    Status     string     `db:"status" json:"status"` // queued | running | ready | failed
    Path       *string    `db:"path" json:"path,omitempty"`
    SizeBytes  *int64     `db:"size_bytes" json:"size_bytes,omitempty"`
    Error      *string    `db:"error" json:"error,omitempty"`
    Requester  string     `db:"requester" json:"requester"`
    CreatedAt  time.Time  `db:"created_at" json:"created_at"`
    StartedAt  *time.Time `db:"started_at" json:"started_at,omitempty"`
    FinishedAt *time.Time `db:"finished_at" json:"finished_at,omitempty"`
}

// dto/master_change_log.go
type MasterChangeLogEvent struct {
    EventID   uuid.UUID `db:"event_id" json:"event_id"`
    Entity    string    `db:"entity" json:"entity"` // products | product_barcodes
    EntityPK  []byte    `db:"entity_pk" json:"entity_pk"` // JSONB
    Field     string    `db:"field" json:"field"`
    OldValue  []byte    `db:"old_value" json:"old_value,omitempty"` // JSONB nullable
    NewValue  []byte    `db:"new_value" json:"new_value"` // JSONB
    ChangedAt time.Time `db:"changed_at" json:"changed_at"`
    LoadID    uuid.UUID `db:"load_id" json:"load_id"`
}
```

## 4. Fiber v3 conventions

```go
// handler принимает fiber.Ctx (без указателя в Fiber v3)
func (h *ProductsHandler) List(c fiber.Ctx) error {
    var req ListProductsRequest
    if err := c.Bind().Query(&req); err != nil {
        return errorspkg.WriteJSON(c, fiber.StatusBadRequest, errorspkg.ErrBadRequest.WithDetails(err))
    }

    snapshotLoadID, err := h.svc.CurrentSnapshot(c.Context())
    if err != nil {
        if errors.Is(err, errorspkg.ErrSnapshotNotReady) {
            c.Set("Retry-After", "60")
            return errorspkg.WriteJSON(c, fiber.StatusServiceUnavailable, err)
        }
        return errorspkg.WriteJSON(c, fiber.StatusInternalServerError, err)
    }

    c.Set("Content-Type", "application/x-ndjson")
    c.Set("X-Snapshot-Id", snapshotLoadID.String())
    c.Set("X-Load-Id", snapshotLoadID.String())
    c.Set("Cache-Control", "private, max-age=86400")

    return c.SendStreamWriter(func(w *bufio.Writer) {
        h.svc.StreamProducts(c.Context(), snapshotLoadID, req, w)
    })
}
```

## 5. Загрузка validation_rules.yaml (формат)

```yaml
# config/validation_rules.yaml
entities:
  products:
    optional: false
    rules:
      - id: product_id_not_empty
        field: product_id
        check: not_empty
        severity: critical
      - id: status_in_enum
        field: status
        check: enum
        params: { values: [active, archived] }
        severity: critical

  receipt_line:
    optional: false
    rules:
      - id: qty_not_zero
        field: qty
        check: not_zero
        severity: critical
      - id: event_time_not_future
        field: event_time
        check: not_future
        params: { tolerance_minutes: 15 }
        severity: critical
      - id: unit_price_paid_non_negative
        field: unit_price_paid
        check: gte
        params: { value: 0 }
        severity: critical
      - id: location_id_referenced
        field: location_id
        check: fk_exists
        params: { referenced: location, referenced_field: location_id }
        severity: soft

  location_stock_snapshot:
    optional: false
    rules:
      - id: qty_on_hand_non_negative
        field: qty_on_hand
        check: gte
        params: { value: 0 }
        severity: critical

  supplier_stock_snapshot:
    optional: true   # Q-009: ERP может не отдавать
    rules: []
```
