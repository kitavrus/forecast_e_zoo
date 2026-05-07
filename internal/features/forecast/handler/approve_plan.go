package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ApprovePlan — POST /v1/replenishment/plans/:id/approve (admin-cli).
func (h *Handler) ApprovePlan(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	var req dto.ApprovePlanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
	}
	if err := validators.ValidateApproveRequest(&req); err != nil {
		return mappers.MapServiceError(c, err)
	}
	plan, err := h.svc.ApprovePlan(c.Context(), id, req.ApprovedBy)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.ApprovePlanResponse{
		Plan: toPlanItem(plan),
	})
}
