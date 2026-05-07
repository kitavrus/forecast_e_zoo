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

// Regenerate — POST /v1/orders/purchase-orders/:id/regenerate (admin-cli).
func (h *Handler) Regenerate(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	var req dto.RegenerateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
	}
	if err := validators.ValidateRegenerateRequest(&req); err != nil {
		return mappers.MapServiceError(c, err)
	}
	res, err := h.svc.Regenerate(c.Context(), models.RegenerateInput{
		POID:      id,
		Reason:    req.Reason,
		ChangedBy: req.ChangedBy,
	})
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.RegenerateResponse{
		OldPOID:     res.OldPOID,
		NewPOID:     res.NewPOID,
		NewPONumber: res.NewPONumber,
	})
}
