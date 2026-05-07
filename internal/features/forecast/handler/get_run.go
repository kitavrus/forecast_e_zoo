package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/mappers"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetRun — GET /v1/forecast/runs/:id.
func (h *Handler) GetRun(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	run, err := h.svc.GetRun(c.Context(), id)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(toRunItem(run))
}
