package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// UpdateCalibration — PUT /v1/kpi/calibrations/:id (admin-cli).
func (h *Handler) UpdateCalibration(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	var req dto.UpdateCalibrationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
	}
	if err := validators.ValidateUpdateCalibration(&req); err != nil {
		return mappers.MapServiceError(c, err)
	}

	updated, err := h.svc.UpdateCalibration(c.Context(), id, req.Params)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.CalibrationItem{
		ID:        updated.ID,
		KpiName:   updated.KpiName,
		ScopeType: updated.ScopeType,
		ScopeID:   updated.ScopeID,
		Params:    updated.Params,
		CreatedAt: updated.CreatedAt,
		UpdatedAt: updated.UpdatedAt,
	})
}
