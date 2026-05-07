package models

import (
	"time"

	"github.com/google/uuid"
)

// ReceiptLine — строка чека (партиционируется по event_date).
type ReceiptLine struct {
	ReceiptID      string    `db:"receipt_id" json:"receipt_id"`
	LineNo         int       `db:"line_no" json:"line_no"`
	LocationID     string    `db:"location_id" json:"location_id"`
	ProductID      string    `db:"product_id" json:"product_id"`
	BarcodeScanned *string   `db:"barcode_scanned" json:"barcode_scanned,omitempty"`
	Qty            float64   `db:"qty" json:"qty"`
	LineKind       string    `db:"line_kind" json:"line_kind"`
	UnitPriceBase  float64   `db:"unit_price_base" json:"unit_price_base"`
	UnitPricePaid  float64   `db:"unit_price_paid" json:"unit_price_paid"`
	DiscountAmount float64   `db:"discount_amount" json:"discount_amount"`
	MarkdownPct    *float64  `db:"markdown_pct" json:"markdown_pct,omitempty"`
	PromoID        *string   `db:"promo_id" json:"promo_id,omitempty"`
	EventDate      time.Time `db:"event_date" json:"event_date"`
	EventTime      time.Time `db:"event_time" json:"event_time"`
	LoyaltyHash    *string   `db:"loyalty_hash" json:"loyalty_hash,omitempty"`
	ValidFrom      time.Time `db:"valid_from" json:"valid_from"`
	ValidTo        time.Time `db:"valid_to" json:"valid_to"`
	SystemTimeFrom time.Time `db:"system_time_from" json:"system_time_from"`
	SystemTimeTo   time.Time `db:"system_time_to" json:"system_time_to"`
	LoadID         uuid.UUID `db:"load_id" json:"load_id"`
}

// LocationStockSnapshot — суточный снимок остатков по локации (партиционируется по event_date).
type LocationStockSnapshot struct {
	LocationID     string    `db:"location_id" json:"location_id"`
	ProductID      string    `db:"product_id" json:"product_id"`
	QtyOnHand      float64   `db:"qty_on_hand" json:"qty_on_hand"`
	QtyReserved    float64   `db:"qty_reserved" json:"qty_reserved"`
	QtyAvailable   float64   `db:"qty_available" json:"qty_available"`
	EventDate      time.Time `db:"event_date" json:"event_date"`
	SnapshotAt     time.Time `db:"snapshot_at" json:"snapshot_at"`
	SystemTimeFrom time.Time `db:"system_time_from" json:"system_time_from"`
	SystemTimeTo   time.Time `db:"system_time_to" json:"system_time_to"`
	LoadID         uuid.UUID `db:"load_id" json:"load_id"`
}

// StockMovement — товародвижение (партиционируется по event_date).
type StockMovement struct {
	MovementID     string    `db:"movement_id" json:"movement_id"`
	Type           string    `db:"type" json:"type"`
	LocationFrom   *string   `db:"location_from" json:"location_from,omitempty"`
	LocationTo     *string   `db:"location_to" json:"location_to,omitempty"`
	ProductID      string    `db:"product_id" json:"product_id"`
	Qty            float64   `db:"qty" json:"qty"`
	EventDate      time.Time `db:"event_date" json:"event_date"`
	EventTime      time.Time `db:"event_time" json:"event_time"`
	SupplierID     *string   `db:"supplier_id" json:"supplier_id,omitempty"`
	Details        []byte    `db:"details" json:"details,omitempty"`
	SystemTimeFrom time.Time `db:"system_time_from" json:"system_time_from"`
	SystemTimeTo   time.Time `db:"system_time_to" json:"system_time_to"`
	LoadID         uuid.UUID `db:"load_id" json:"load_id"`
}

// SupplierStockSnapshot — снимок остатков у поставщика (партиционируется по event_date).
type SupplierStockSnapshot struct {
	SupplierID   string    `db:"supplier_id" json:"supplier_id"`
	ProductID    string    `db:"product_id" json:"product_id"`
	QtyAvailable float64   `db:"qty_available" json:"qty_available"`
	SnapshotAt   time.Time `db:"snapshot_at" json:"snapshot_at"`
	EventDate    time.Time `db:"event_date" json:"event_date"`
	LoadID       uuid.UUID `db:"load_id" json:"load_id"`
}
