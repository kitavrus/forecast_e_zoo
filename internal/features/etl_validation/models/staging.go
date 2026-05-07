package models

import "time"

// Staging-структуры — представление сущностей источника (Module 1 source-adapter)
// до того, как они попадут в TEMP TABLE pg_temp.stg_*.
//
// json-теги совпадают с json-полями DTO source-adapter
// (internal/features/data_export/models/dto/*.go) — single-source-of-truth.

// StgReceiptLine — строка чека (источник для mart_demand_history, mart_kpi_daily).
//
// dto.ReceiptLine выдаёт unit_price_base / unit_price_paid (раньше staging
// ожидал unit_price_list — поле, которого нет в DTO, что приводило к
// NOT NULL violation на COPY). HadPromo / PromoType derived (источник не
// отдаёт), здесь зарезервированы для совместимости с transformer-ом.
type StgReceiptLine struct {
	ReceiptID      string    `json:"receipt_id"`
	LocationID     string    `json:"location_id"`
	ProductID      string    `json:"product_id"`
	LineKind       string    `json:"line_kind"` // sale|return|gift|promo_bonus
	Qty            float64   `json:"qty"`
	UnitPriceBase  float64   `json:"unit_price_base"`
	UnitPricePaid  float64   `json:"unit_price_paid"`
	DiscountAmount float64   `json:"discount_amount"`
	PromoID        *string   `json:"promo_id,omitempty"`
	EventTime      time.Time `json:"event_time"`
}

// StgStockOnHand — текущий остаток (источник для mart_calculation_input.on_hand).
//
// MVP source-adapter отдаёт через dto.LocationStockSnapshot (qty_on_hand);
// in_transit отсутствует в DTO — поле зарезервировано для будущего расширения.
type StgStockOnHand struct {
	ProductID  string  `json:"product_id"`
	LocationID string  `json:"location_id"`
	QtyOnHand  float64 `json:"qty_on_hand"`
}

// StgProduct — справочник товаров (mart_master_current).
//
// PK source-adapter — product_id (НЕ id); см. dto.Product.
type StgProduct struct {
	ProductID  string  `json:"product_id"`
	Name       string  `json:"name"`
	CategoryID *string `json:"category_id,omitempty"`
	Status     string  `json:"status"`
}

// StgLocation — справочник локаций (PK location_id).
type StgLocation struct {
	LocationID string `json:"location_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

// StgSupplier — справочник поставщиков (PK supplier_id).
type StgSupplier struct {
	SupplierID string `json:"supplier_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

// StgOrderRule — правило заказа (источник для mart_calculation_input).
//
// dto.OrderRule: PK rule_id, продукт через scope/scope_ref. product_id /
// formula derived (источник их не отдаёт).
type StgOrderRule struct {
	RuleID          string   `json:"rule_id"`
	Scope           string   `json:"scope"`
	ScopeRef        *string  `json:"scope_ref,omitempty"`
	LocationID      *string  `json:"location_id,omitempty"`
	SafetyStockDays *float64 `json:"safety_stock_days,omitempty"`
	ServiceLevelPct *float64 `json:"service_level_pct,omitempty"`
	OverrideMOQ     *int     `json:"override_moq,omitempty"`
}

// StgSupplySpec — спецификация поставки (альтернатива order_rule).
//
// dto.SupplySpec: composite-PK (supplier_id, product_id, location_id).
type StgSupplySpec struct {
	SupplierID    string   `json:"supplier_id"`
	ProductID     string   `json:"product_id"`
	LocationID    string   `json:"location_id"`
	LeadTimeDays  *int     `json:"lead_time_days,omitempty"`
	MinOrderQty   *int     `json:"min_order_qty,omitempty"`
	PurchasePrice *float64 `json:"purchase_price,omitempty"`
	Currency      string   `json:"currency"`
	PackSize      *int     `json:"pack_size,omitempty"`
}

// StgReceivingDetail — деталь приёмки (источник для mart_supplier_scorecard).
//
// Entity не реализован в source-adapter MVP (handler returns 501); структура
// сохранена для совместимости с mart_supplier_scorecard_insert.sql.
type StgReceivingDetail struct {
	SupplierID     string    `json:"supplier_id"`
	ProductID      string    `json:"product_id"`
	DeliveryDate   time.Time `json:"delivery_date"`
	FillRate       float64   `json:"fill_rate"`
	OnTimeInFull   bool      `json:"otif"`
	LeadTimeActual float64   `json:"lead_time_actual"`
	QtyShort       float64   `json:"qty_short"`
	QtyDamaged     float64   `json:"qty_damaged"`
	QtyReturned    float64   `json:"qty_returned"`
	Late           bool      `json:"late"`
}

// StgPromo — promotion (PK promo_id; см. dto.Promo).
type StgPromo struct {
	PromoID    string    `json:"promo_id"`
	ProductID  string    `json:"product_id"`
	LocationID *string   `json:"location_id,omitempty"`
	Type       string    `json:"type"`
	DateFrom   time.Time `json:"date_from"`
	DateTo     time.Time `json:"date_to"`
}

// StgStoreAssortment — ассортимент магазина (см. dto.StoreAssortment).
//
// Source-adapter отдаёт effective_from / effective_to; marts читают
// valid_from / valid_to (зарезервированы под derive-step transformer-а).
type StgStoreAssortment struct {
	ProductID      string     `json:"product_id"`
	LocationID     string     `json:"location_id"`
	LifecycleState string     `json:"lifecycle_state"`
	EffectiveFrom  time.Time  `json:"effective_from"`
	EffectiveTo    *time.Time `json:"effective_to,omitempty"`
}
