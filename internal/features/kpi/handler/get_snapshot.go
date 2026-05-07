package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/mappers"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetSnapshot — GET /v1/kpi/snapshots/:id.
func (h *Handler) GetSnapshot(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	r, err := h.svc.GetSnapshot(c.Context(), id)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(toSnapshotItem(r))
}
