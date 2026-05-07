package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/orders/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetPO — GET /v1/orders/purchase-orders/:id.
func (h *Handler) GetPO(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	details, err := h.svc.GetPOWithDetails(c.Context(), id)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.PurchaseOrderWithLinesResponse{
		Order:   toPurchaseOrderItem(details.Order),
		Lines:   toPOLineItems(details.Lines),
		History: toPOHistoryItems(details.History),
	})
}
