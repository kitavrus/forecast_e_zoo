package models

import "time"

// Staging-структуры — представление сущностей источника (Module 1 source-adapter)
// до того, как они попадут в TEMP TABLE pg_temp.stg_*.
//
// Поля совпадают с DTO Modul 1 (data_export). Здесь — минимальный
// набор для transformer/validation; раскраска по mart-ам — в transformer/.

// StgReceiptLine — строка чека (источник для mart_demand_history, mart_kpi_daily).
type StgReceiptLine struct {
	ReceiptID       string    `json:"receipt_id"`
	LocationID      string    `json:"location_id"`
	ProductID       string    `json:"product_id"`
	LineKind        string    `json:"line_kind"` // sale|return|gift|promo_bonus
	Qty             float64   `json:"qty"`
	UnitPriceList   float64   `json:"unit_price_list"`
	UnitPricePaid   float64   `json:"unit_price_paid"`
	DiscountAmount  float64   `json:"discount_amount"`
	HadPromo        bool      `json:"had_promo"`
	PromoType       *string   `json:"promo_type,omitempty"`
	EventTime       time.Time `json:"event_time"`
}

// StgStockOnHand — текущий остаток (источник для mart_calculation_input.on_hand).
type StgStockOnHand struct {
	ProductID  string  `json:"product_id"`
	LocationID string  `json:"location_id"`
	OnHand     float64 `json:"on_hand"`
	InTransit  float64 `json:"in_transit"`
}

// StgProduct — справочник товаров (mart_master_current).
type StgProduct struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CategoryID *string `json:"category_id,omitempty"`
	UnitID     *string `json:"unit_id,omitempty"`
	IsActive   bool    `json:"is_active"`
}

// StgLocation — справочник локаций.
type StgLocation struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// StgSupplier — справочник поставщиков.
type StgSupplier struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// StgOrderRule — правило заказа (источник для mart_calculation_input).
type StgOrderRule struct {
	ID                  string   `json:"id"`
	ProductID           string   `json:"product_id"`
	LocationID          string   `json:"location_id"`
	Formula             string   `json:"formula"`
	SafetyStock         *float64 `json:"safety_stock,omitempty"`
	ForecastHorizonDays *int     `json:"forecast_horizon_days,omitempty"`
	MinQty              *float64 `json:"min_qty,omitempty"`
	MaxQty              *float64 `json:"max_qty,omitempty"`
	IsActive            bool     `json:"is_active"`
}

// StgSupplySpec — спецификация поставки (альтернатива order_rule).
type StgSupplySpec struct {
	ID           string   `json:"id"`
	SupplierID   string   `json:"supplier_id"`
	ProductID    string   `json:"product_id"`
	LeadTimeDays *int     `json:"lead_time_days,omitempty"`
	MinQty       *float64 `json:"min_qty,omitempty"`
	MaxQty       *float64 `json:"max_qty,omitempty"`
	IsActive     bool     `json:"is_active"`
}

// StgReceivingDetail — деталь приёмки (источник для mart_supplier_scorecard).
type StgReceivingDetail struct {
	SupplierID       string    `json:"supplier_id"`
	ProductID        string    `json:"product_id"`
	DeliveryDate     time.Time `json:"delivery_date"`
	FillRate         float64   `json:"fill_rate"`
	OnTimeInFull     bool      `json:"otif"`
	LeadTimeActual   float64   `json:"lead_time_actual"`
	QtyShort         float64   `json:"qty_short"`
	QtyDamaged       float64   `json:"qty_damaged"`
	QtyReturned      float64   `json:"qty_returned"`
	Late             bool      `json:"late"`
}
