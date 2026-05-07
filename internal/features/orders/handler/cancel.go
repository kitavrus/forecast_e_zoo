package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/orders/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/orders/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Cancel — POST /v1/orders/purchase-orders/:id/cancel (admin-cli).
func (h *Handler) Cancel(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	var req dto.CancelRequest
	if err := c.Bind().JSON(&req); err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
	}
	if err := validators.ValidateCancelRequest(&req); err != nil {
		return mappers.MapServiceError(c, err)
	}
	po, err := h.svc.Cancel(c.Context(), models.CancelInput{
		POID:      id,
		Reason:    req.Reason,
		ChangedBy: req.ChangedBy,
	})
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.CancelResponse{
		Order: toPurchaseOrderItem(po),
	})
}
