// Package dto — request/response DTO фичи orders (Module 6).
package dto

import (
	"time"

	"github.com/google/uuid"
)

// PurchaseOrderItem — DTO одной строки purchase_orders для API.
type PurchaseOrderItem struct {
	ID            uuid.UUID  `json:"id"`
	PONumber      string     `json:"po_number"`
	PlanID        uuid.UUID  `json:"plan_id"`
	SupplierID    string     `json:"supplier_id"`
	LocationID    string     `json:"location_id"`
	Status        string     `json:"status" enums:"draft,ready_to_send,sent,confirmed_by_erp,received,cancelled"`
	TotalQty      float64    `json:"total_qty"`
	TotalAmount   *float64   `json:"total_amount,omitempty"`
	Currency      string     `json:"currency"`
	DeliveryDate  *string    `json:"delivery_date,omitempty"` // YYYY-MM-DD
	Notes         *string    `json:"notes,omitempty"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	SentToChannel *string    `json:"sent_to_channel,omitempty"`
	CancelReason  *string    `json:"cancel_reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// POLineItem — DTO одной строки po_lines.
type POLineItem struct {
	ID            uuid.UUID `json:"id"`
	ProductID     string    `json:"product_id"`
	Qty           float64   `json:"qty"`
	UnitPrice     *float64  `json:"unit_price,omitempty"`
	LineAmount    *float64  `json:"line_amount,omitempty"`
	PricingSource *string   `json:"pricing_source,omitempty" enums:"product,supplier_default,missing"`
	Notes         *string   `json:"notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// POStatusHistoryItem — DTO одной строки po_status_history.
type POStatusHistoryItem struct {
	ID         uuid.UUID `json:"id"`
	FromStatus *string   `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status" enums:"draft,ready_to_send,sent,confirmed_by_erp,received,cancelled"`
	Reason     *string   `json:"reason,omitempty"`
	ChangedBy  *string   `json:"changed_by,omitempty"`
	ChangedAt  time.Time `json:"changed_at"`
}

// ListPurchaseOrdersResponse — ответ GET /v1/orders/purchase-orders.
type ListPurchaseOrdersResponse struct {
	Items      []PurchaseOrderItem `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

// PurchaseOrderWithLinesResponse — ответ GET /v1/orders/purchase-orders/:id.
type PurchaseOrderWithLinesResponse struct {
	Order   PurchaseOrderItem     `json:"order"`
	Lines   []POLineItem          `json:"lines"`
	History []POStatusHistoryItem `json:"history,omitempty"`
}

// BuildRequest — body POST /v1/orders/purchase-orders/build.
type BuildRequest struct {
	// MaxPlans — лимит planов за один build (0 = default).
	MaxPlans int `json:"max_plans,omitempty"`
}

// BuildResponse — ответ POST /v1/orders/purchase-orders/build.
type BuildResponse struct {
	RunID          uuid.UUID `json:"run_id"`
	Started        bool      `json:"started"`
	PlansProcessed int       `json:"plans_processed"`
	POsCreated     int       `json:"pos_created"`
	Skipped        int       `json:"skipped"`
	Errors         int       `json:"errors"`
}

// CancelRequest — body POST /:id/cancel.
type CancelRequest struct {
	Reason    string `json:"reason"`
	ChangedBy string `json:"changed_by"`
}

// CancelResponse — ответ cancel.
type CancelResponse struct {
	Order PurchaseOrderItem `json:"order"`
}

// RegenerateRequest — body POST /:id/regenerate.
type RegenerateRequest struct {
	Reason    string `json:"reason"`
	ChangedBy string `json:"changed_by"`
}

// RegenerateResponse — ответ regenerate.
type RegenerateResponse struct {
	OldPOID     uuid.UUID `json:"old_po_id"`
	NewPOID     uuid.UUID `json:"new_po_id"`
	NewPONumber string    `json:"new_po_number"`
}
