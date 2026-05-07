// Package handler — Fiber v3 хендлеры фичи orders (Module 6).
//
// Один action = один файл (snake_case): list_pos.go, get_po.go,
// build.go, cancel.go, regenerate.go.
package handler

import (
	"context"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models/dto"
)

// OrderService — узкий interface (DI seam).
type OrderService interface {
	ListPOs(ctx context.Context, f models.POFilter) ([]models.PurchaseOrder, string, error)
	GetPOWithDetails(ctx context.Context, id uuid.UUID) (models.POWithDetails, error)
	TriggerBuild(ctx context.Context, maxPlans int) (uuid.UUID, bool, error)
	Cancel(ctx context.Context, in models.CancelInput) (models.PurchaseOrder, error)
	Regenerate(ctx context.Context, in models.RegenerateInput) (models.RegenerateResult, error)
}

// Handler — все orders endpoints.
type Handler struct {
	svc OrderService
}

// NewHandler создаёт Handler.
func NewHandler(svc OrderService) *Handler { return &Handler{svc: svc} }

// --- DTO converters ---

func toPurchaseOrderItem(po models.PurchaseOrder) dto.PurchaseOrderItem {
	var deliveryDate *string
	if po.DeliveryDate != nil {
		s := po.DeliveryDate.Format("2006-01-02")
		deliveryDate = &s
	}
	return dto.PurchaseOrderItem{
		ID:            po.ID,
		PONumber:      po.PONumber,
		PlanID:        po.PlanID,
		SupplierID:    po.SupplierID,
		LocationID:    po.LocationID,
		Status:        po.Status,
		TotalQty:      po.TotalQty,
		TotalAmount:   po.TotalAmount,
		Currency:      po.Currency,
		DeliveryDate:  deliveryDate,
		Notes:         po.Notes,
		SentAt:        po.SentAt,
		SentToChannel: po.SentToChannel,
		CancelReason:  po.CancelReason,
		CreatedAt:     po.CreatedAt,
		UpdatedAt:     po.UpdatedAt,
	}
}

func toPOLineItems(lines []models.POLine) []dto.POLineItem {
	out := make([]dto.POLineItem, 0, len(lines))
	for _, l := range lines {
		out = append(out, dto.POLineItem{
			ID:            l.ID,
			ProductID:     l.ProductID,
			Qty:           l.Qty,
			UnitPrice:     l.UnitPrice,
			LineAmount:    l.LineAmount,
			PricingSource: l.PricingSource,
			Notes:         l.Notes,
			CreatedAt:     l.CreatedAt,
		})
	}
	return out
}

func toPOHistoryItems(history []models.POStatusHistory) []dto.POStatusHistoryItem {
	out := make([]dto.POStatusHistoryItem, 0, len(history))
	for _, h := range history {
		out = append(out, dto.POStatusHistoryItem{
			ID:         h.ID,
			FromStatus: h.FromStatus,
			ToStatus:   h.ToStatus,
			Reason:     h.Reason,
			ChangedBy:  h.ChangedBy,
			ChangedAt:  h.ChangedAt,
		})
	}
	return out
}
