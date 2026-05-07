// Package models — domain types фичи orders (Module 6).
package models

import (
	"time"

	"github.com/google/uuid"
)

// PurchaseOrder — одна строка orders.purchase_orders.
type PurchaseOrder struct {
	ID            uuid.UUID
	PONumber      string
	PlanID        uuid.UUID
	SupplierID    string
	LocationID    string
	Status        string
	TotalQty      float64
	TotalAmount   *float64
	Currency      string
	DeliveryDate  *time.Time
	Notes         *string
	SentAt        *time.Time
	SentToChannel *string
	CancelReason  *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// POLine — одна строка orders.po_lines.
type POLine struct {
	ID            uuid.UUID
	POID          uuid.UUID
	ProductID     string
	Qty           float64
	UnitPrice     *float64
	LineAmount    *float64
	PricingSource *string
	Notes         *string
	CreatedAt     time.Time
}

// POStatusHistory — одна строка orders.po_status_history.
type POStatusHistory struct {
	ID         uuid.UUID
	POID       uuid.UUID
	FromStatus *string
	ToStatus   string
	Reason     *string
	ChangedBy  *string
	ChangedAt  time.Time
}

// POFilter — фильтр для list-запроса PO.
type POFilter struct {
	Status     *string
	SupplierID *string
	PlanID     *uuid.UUID
	From       *time.Time
	To         *time.Time
	Limit      int
	Cursor     string
}

// POWithDetails — PO + lines + history.
type POWithDetails struct {
	Order   PurchaseOrder
	Lines   []POLine
	History []POStatusHistory
}

// ApprovedPlan — облегчённый view forecast.replenishment_plans для builder-а.
type ApprovedPlan struct {
	ID         uuid.UUID
	RunID      uuid.UUID
	SupplierID string
	LocationID string
	PlanDate   time.Time
	TotalQty   float64
	LinesCount int
}

// PlanLine — элемент plan'а: то, что нужно build-у про каждый product.
//
// Поля совпадают с forecast.calculation_lines, но мы достаём только то,
// что нужно для PO (qty + product/location/supplier).
type PlanLine struct {
	ProductID  string
	LocationID string
	SupplierID *string
	ReorderQty float64
}

// SupplierMaster — выжимка из marts.mart_master_current.payload по supplier.
type SupplierMaster struct {
	SupplierID         string
	Currency           string
	LeadTimeDays       int
	DefaultUnitPrice   *float64
	HasMartRow         bool
}

// ProductMaster — выжимка из marts.mart_master_current.payload по product.
type ProductMaster struct {
	ProductID string
	UnitPrice *float64
	HasMartRow bool
}

// BuildResult — итог одного build run-а.
type BuildResult struct {
	RunID          uuid.UUID
	PlansProcessed int
	POsCreated     int
	Skipped        int
	Errors         int
}

// CancelInput — параметры cancel.
type CancelInput struct {
	POID      uuid.UUID
	Reason    string
	ChangedBy string
}

// RegenerateInput — параметры regenerate.
type RegenerateInput struct {
	POID      uuid.UUID
	Reason    string
	ChangedBy string
}

// RegenerateResult — итог regenerate.
type RegenerateResult struct {
	OldPOID      uuid.UUID
	NewPOID      uuid.UUID
	NewPONumber  string
}
