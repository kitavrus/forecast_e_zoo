package dto

import (
	"time"

	"github.com/google/uuid"
)

// Product — публичный DTO товара.
type Product struct {
	ProductID            string    `db:"product_id" json:"product_id"`
	Name                 string    `db:"name" json:"name"`
	Brand                *string   `db:"brand" json:"brand,omitempty"`
	Manufacturer         *string   `db:"manufacturer" json:"manufacturer,omitempty"`
	CategoryID           string    `db:"category_id" json:"category_id"`
	CategoryPath         string    `db:"category_path" json:"category_path"`
	WeightKg             *float64  `db:"weight_kg" json:"weight_kg,omitempty"`
	PalletQty            *int      `db:"pallet_qty" json:"pallet_qty,omitempty"`
	ShelfLifeDays        *int      `db:"shelf_life_days" json:"shelf_life_days,omitempty"`
	StorageTempMin       *float64  `db:"storage_temp_min" json:"storage_temp_min,omitempty"`
	StorageTempMax       *float64  `db:"storage_temp_max" json:"storage_temp_max,omitempty"`
	RequiresPrescription bool      `db:"requires_prescription" json:"requires_prescription"`
	IsDangerousGoods     bool      `db:"is_dangerous_goods" json:"is_dangerous_goods"`
	Status               string    `db:"status" json:"status"`
	CreatedAt            time.Time `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time `db:"updated_at" json:"updated_at"`
	LoadID               uuid.UUID `db:"load_id" json:"load_id"`
}

// ProductBarcode — DTO штрихкода.
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

// Category — DTO категории.
type Category struct {
	CategoryID string    `db:"category_id" json:"category_id"`
	ParentID   *string   `db:"parent_id" json:"parent_id,omitempty"`
	Level      int16     `db:"level" json:"level"`
	Name       string    `db:"name" json:"name"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
	LoadID     uuid.UUID `db:"load_id" json:"load_id"`
}

// Location — DTO локации.
type Location struct {
	LocationID string     `db:"location_id" json:"location_id"`
	Type       string     `db:"type" json:"type"`
	Name       string     `db:"name" json:"name"`
	Address    *string    `db:"address" json:"address,omitempty"`
	City       *string    `db:"city" json:"city,omitempty"`
	Region     *string    `db:"region" json:"region,omitempty"`
	OpenedAt   *time.Time `db:"opened_at" json:"opened_at,omitempty"`
	ClosedAt   *time.Time `db:"closed_at" json:"closed_at,omitempty"`
	Status     string     `db:"status" json:"status"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
	LoadID     uuid.UUID  `db:"load_id" json:"load_id"`
}

// Supplier — DTO поставщика.
type Supplier struct {
	SupplierID   string    `db:"supplier_id" json:"supplier_id"`
	Name         string    `db:"name" json:"name"`
	INN          *string   `db:"inn" json:"inn,omitempty"`
	GLN          *string   `db:"gln" json:"gln,omitempty"`
	PaymentTerms *string   `db:"payment_terms" json:"payment_terms,omitempty"`
	EDIProfile   *string   `db:"edi_profile" json:"edi_profile,omitempty"`
	Status       string    `db:"status" json:"status"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
	LoadID       uuid.UUID `db:"load_id" json:"load_id"`
}

// SupplySpec — DTO спецификации поставки.
type SupplySpec struct {
	SupplierID    string     `db:"supplier_id" json:"supplier_id"`
	ProductID     string     `db:"product_id" json:"product_id"`
	LocationID    string     `db:"location_id" json:"location_id"`
	Priority      int16      `db:"priority" json:"priority"`
	MinOrderQty   int        `db:"min_order_qty" json:"min_order_qty"`
	PurchasePrice *float64   `db:"purchase_price" json:"purchase_price,omitempty"`
	Currency      string     `db:"currency" json:"currency"`
	LeadTimeDays  int        `db:"lead_time_days" json:"lead_time_days"`
	PackSize      int        `db:"pack_size" json:"pack_size"`
	EffectiveFrom time.Time  `db:"effective_from" json:"effective_from"`
	EffectiveTo   *time.Time `db:"effective_to" json:"effective_to,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	LoadID        uuid.UUID  `db:"load_id" json:"load_id"`
}

// Promo — DTO промо/маркдауна.
type Promo struct {
	PromoID           string    `db:"promo_id" json:"promo_id"`
	ProductID         string    `db:"product_id" json:"product_id"`
	LocationID        *string   `db:"location_id" json:"location_id,omitempty"`
	Type              string    `db:"type" json:"type"`
	DiscountPct       *float64  `db:"discount_pct" json:"discount_pct,omitempty"`
	PromoPriceWithVAT *float64  `db:"promo_price_with_vat" json:"promo_price_with_vat,omitempty"`
	DateFrom          time.Time `db:"date_from" json:"date_from"`
	DateTo            time.Time `db:"date_to" json:"date_to"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
	LoadID            uuid.UUID `db:"load_id" json:"load_id"`
}

// OrderRule — DTO правила заказа.
type OrderRule struct {
	RuleID          string     `db:"rule_id" json:"rule_id"`
	Scope           string     `db:"scope" json:"scope"`
	ScopeRef        *string    `db:"scope_ref" json:"scope_ref,omitempty"`
	LocationID      *string    `db:"location_id" json:"location_id,omitempty"`
	SafetyStockDays *float64   `db:"safety_stock_days" json:"safety_stock_days,omitempty"`
	ServiceLevelPct *float64   `db:"service_level_pct" json:"service_level_pct,omitempty"`
	OverrideMOQ     *int       `db:"override_moq" json:"override_moq,omitempty"`
	EffectiveFrom   time.Time  `db:"effective_from" json:"effective_from"`
	EffectiveTo     *time.Time `db:"effective_to" json:"effective_to,omitempty"`
	LoadID          uuid.UUID  `db:"load_id" json:"load_id"`
}

// SupplyPlan — DTO слота поставки.
type SupplyPlan struct {
	PlanID      string     `db:"plan_id" json:"plan_id"`
	SupplierID  string     `db:"supplier_id" json:"supplier_id"`
	LocationID  string     `db:"location_id" json:"location_id"`
	PlannedDate time.Time  `db:"planned_date" json:"planned_date"`
	SlotTime    *string    `db:"slot_time" json:"slot_time,omitempty"`
	CutoffAt    *time.Time `db:"cutoff_at" json:"cutoff_at,omitempty"`
	Status      string     `db:"status" json:"status"`
	LoadID      uuid.UUID  `db:"load_id" json:"load_id"`
}

// StoreAssortment — DTO ассортимента магазина.
type StoreAssortment struct {
	LocationID      string     `db:"location_id" json:"location_id"`
	ProductID       string     `db:"product_id" json:"product_id"`
	LifecycleState  string     `db:"lifecycle_state" json:"lifecycle_state"`
	AssortmentClass *string    `db:"assortment_class" json:"assortment_class,omitempty"`
	PriceMin        *float64   `db:"price_min" json:"price_min,omitempty"`
	PriceMax        *float64   `db:"price_max" json:"price_max,omitempty"`
	EffectiveFrom   time.Time  `db:"effective_from" json:"effective_from"`
	EffectiveTo     *time.Time `db:"effective_to" json:"effective_to,omitempty"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	LoadID          uuid.UUID  `db:"load_id" json:"load_id"`
}

// StoreAssortmentLifecycleEventResponse — ADR-016 публичный контракт события
// для X-Flow ETL (lifecycle_events).
type StoreAssortmentLifecycleEventResponse struct {
	EventID      string    `json:"eventId"`
	EventType    string    `json:"eventType"`
	LocationID   string    `json:"locationId"`
	ProductID    string    `json:"productId"`
	EffectiveAt  time.Time `json:"effectiveAt"`
	Reason       *string   `json:"reason,omitempty"`
	PromoID      *string   `json:"promoId,omitempty"`
	PriorState   *string   `json:"priorState,omitempty"`
	NewState     string    `json:"newState"`
	SourceLoadID string    `json:"sourceLoadId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// MasterChangeLogEntry — DTO события master_change_log.
type MasterChangeLogEntry struct {
	EventID   uuid.UUID `db:"event_id" json:"event_id"`
	Entity    string    `db:"entity" json:"entity"`
	EntityPK  []byte    `db:"entity_pk" json:"entity_pk"`
	Field     string    `db:"field" json:"field"`
	OldValue  []byte    `db:"old_value" json:"old_value,omitempty"`
	NewValue  []byte    `db:"new_value" json:"new_value"`
	ChangedAt time.Time `db:"changed_at" json:"changed_at"`
	LoadID    uuid.UUID `db:"load_id" json:"load_id"`
}

// Page-response типы (по сущности — для удобства handler/swagger).
type GetProductsResponse = PageResponse[Product]
type GetProductBarcodesResponse = PageResponse[ProductBarcode]
type GetCategoryResponse = PageResponse[Category]
type GetLocationResponse = PageResponse[Location]
type GetSupplierResponse = PageResponse[Supplier]
type GetSupplySpecResponse = PageResponse[SupplySpec]
type GetPromoResponse = PageResponse[Promo]
type GetOrderRuleResponse = PageResponse[OrderRule]
type GetSupplyPlanResponse = PageResponse[SupplyPlan]
type GetStoreAssortmentResponse = PageResponse[StoreAssortment]
type GetStoreAssortmentLifecycleEventsResponse = PageResponse[StoreAssortmentLifecycleEventResponse]
type GetMasterChangeLogResponse = PageResponse[MasterChangeLogEntry]
