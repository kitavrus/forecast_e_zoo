package loader

// Реальный ERP-stack/auth контракт пока не выбран (см. открытые вопросы
// Q-001..Q-003 в design-adr.md): нужен ответ E-Zoo IT по протоколу (REST?
// SOAP? SFTP CSV?), формату DTO, авторизации (mTLS? Bearer? IP-allowlist).
// До этого момента используется in-memory ErpEZooReader (см. этот пакет),
// который грузит fixtures из testdata/. Замена на реальный reader делается
// без изменения SourceReader-интерфейса и loader-pipeline (фаза 10).

import (
	"context"
	"net/http"
	"time"
)

// ERP DTO-types — namespace ERP. Эти структуры — что отдаёт ERP _до_ маппинга
// в domain. В MVP они близки к domain-row из repository, но публичный
// контракт reader-а не должен зависеть от схемы БД — поэтому отдельные types.

type ErpProduct struct {
	ID         string         `json:"id"`
	SKU        string         `json:"sku"`
	Name       string         `json:"name"`
	CategoryID *string        `json:"category_id,omitempty"`
	Unit       string         `json:"unit"`
	PackSize   *float64       `json:"pack_size,omitempty"`
	IsActive   bool           `json:"is_active"`
	Attributes map[string]any `json:"attributes,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type ErpProductBarcode struct {
	ProductID string `json:"product_id"`
	Barcode   string `json:"barcode"`
	IsPrimary bool   `json:"is_primary"`
}

type ErpCategory struct {
	ID        string    `json:"id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	Path      string    `json:"path,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ErpLocation struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Region    *string   `json:"region,omitempty"`
	Address   *string   `json:"address,omitempty"`
	Lat       *float64  `json:"lat,omitempty"`
	Lon       *float64  `json:"lon,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ErpSupplier struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	INN       *string   `json:"inn,omitempty"`
	KPP       *string   `json:"kpp,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ErpSupplySpec struct {
	ProductID    string     `json:"product_id"`
	SupplierID   string     `json:"supplier_id"`
	PackQty      *float64   `json:"pack_qty,omitempty"`
	LeadTimeDays *int       `json:"lead_time_days,omitempty"`
	MinOrderQty  *float64   `json:"min_order_qty,omitempty"`
	Multiple     *float64   `json:"multiple,omitempty"`
	ValidFrom    time.Time  `json:"valid_from"`
	ValidTo      *time.Time `json:"valid_to,omitempty"`
}

type ErpPromo struct {
	ID          string         `json:"id"`
	LocationID  string         `json:"location_id"`
	ProductID   string         `json:"product_id"`
	StartDate   time.Time      `json:"start_date"`
	EndDate     time.Time      `json:"end_date"`
	DiscountPct *float64       `json:"discount_pct,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type ErpOrderRule struct {
	ID         string         `json:"id"`
	LocationID string         `json:"location_id"`
	ProductID  *string        `json:"product_id,omitempty"`
	CategoryID *string        `json:"category_id,omitempty"`
	RuleType   string         `json:"rule_type"`
	Payload    map[string]any `json:"payload,omitempty"`
	ValidFrom  time.Time      `json:"valid_from"`
	ValidTo    *time.Time     `json:"valid_to,omitempty"`
}

type ErpSupplyPlan struct {
	ID         string         `json:"id"`
	LocationID string         `json:"location_id"`
	ProductID  string         `json:"product_id"`
	SupplierID string         `json:"supplier_id"`
	PlanDate   time.Time      `json:"plan_date"`
	Qty        float64        `json:"qty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type ErpStoreAssortment struct {
	LocationID string     `json:"location_id"`
	ProductID  string     `json:"product_id"`
	StartDate  time.Time  `json:"start_date"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	IsActive   bool       `json:"is_active"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ErpStoreAssortmentLifecycleEvent struct {
	LocationID string         `json:"location_id"`
	ProductID  string         `json:"product_id"`
	EventType  string         `json:"event_type"`
	EventDate  time.Time      `json:"event_date"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type ErpMasterChangeLog struct {
	Entity    string          `json:"entity"`
	EntityPK  map[string]any  `json:"entity_pk"`
	Field     string          `json:"field"`
	OldValue  any             `json:"old_value,omitempty"`
	NewValue  any             `json:"new_value,omitempty"`
	ChangedAt time.Time       `json:"changed_at"`
}

type ErpReceiptLine struct {
	ID         int64          `json:"id"`
	ReceiptID  string         `json:"receipt_id"`
	LocationID string         `json:"location_id"`
	ProductID  string         `json:"product_id"`
	Qty        float64        `json:"qty"`
	Price      float64        `json:"price"`
	EventTime  time.Time      `json:"event_time"`
	EventDate  time.Time      `json:"event_date"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type ErpLocationStockSnapshot struct {
	EventDate   time.Time `json:"event_date"`
	LocationID  string    `json:"location_id"`
	ProductID   string    `json:"product_id"`
	QtyOnHand   float64   `json:"qty_on_hand"`
	QtyReserved float64   `json:"qty_reserved"`
	AsOf        time.Time `json:"as_of"`
}

type ErpStockMovement struct {
	ID           int64          `json:"id"`
	EventDate    time.Time      `json:"event_date"`
	EventTime    time.Time      `json:"event_time"`
	LocationID   string         `json:"location_id"`
	ProductID    string         `json:"product_id"`
	MovementType string         `json:"movement_type"`
	Qty          float64        `json:"qty"`
	RefID        *string        `json:"ref_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type ErpSupplierStockSnapshot struct {
	EventDate    time.Time `json:"event_date"`
	SupplierID   string    `json:"supplier_id"`
	ProductID    string    `json:"product_id"`
	QtyAvailable float64   `json:"qty_available"`
	AsOf         time.Time `json:"as_of"`
}

// SourceReader — порт к ERP. По методу на каждую из 16 сущностей.
// Все методы возвращают итератор; loader (фаза 10) проходит через Next()
// и UPSERT'ит в БД пакетами.
type SourceReader interface {
	// master entities
	ReadProducts(ctx context.Context, since time.Time) (PageIterator[ErpProduct], error)
	ReadProductBarcodes(ctx context.Context, since time.Time) (PageIterator[ErpProductBarcode], error)
	ReadCategory(ctx context.Context, since time.Time) (PageIterator[ErpCategory], error)
	ReadLocation(ctx context.Context, since time.Time) (PageIterator[ErpLocation], error)
	ReadSupplier(ctx context.Context, since time.Time) (PageIterator[ErpSupplier], error)
	ReadSupplySpec(ctx context.Context, since time.Time) (PageIterator[ErpSupplySpec], error)
	ReadPromo(ctx context.Context, since time.Time) (PageIterator[ErpPromo], error)
	ReadOrderRule(ctx context.Context, since time.Time) (PageIterator[ErpOrderRule], error)
	ReadSupplyPlan(ctx context.Context, since time.Time) (PageIterator[ErpSupplyPlan], error)
	ReadStoreAssortment(ctx context.Context, since time.Time) (PageIterator[ErpStoreAssortment], error)
	ReadStoreAssortmentLifecycleEvents(ctx context.Context, since time.Time) (PageIterator[ErpStoreAssortmentLifecycleEvent], error)
	ReadMasterChangeLog(ctx context.Context, since time.Time) (PageIterator[ErpMasterChangeLog], error)

	// facts (partitioned by event_date)
	ReadReceiptLine(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpReceiptLine], error)
	ReadLocationStockSnapshot(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpLocationStockSnapshot], error)
	ReadStockMovement(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpStockMovement], error)
	ReadSupplierStockSnapshot(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpSupplierStockSnapshot], error)

	Close(ctx context.Context) error
}

// SourceAuth — заготовка для будущих REST/SOAP реализаций
// (mTLS / Bearer / signed JWT). In-memory stub её не использует.
type SourceAuth interface {
	Apply(req *http.Request) error
}
