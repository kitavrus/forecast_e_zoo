package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetVersion — GET /v1/marts/:name/version.
func (h *Handler) GetVersion(c fiber.Ctx) error {
	name := c.Params("name")
	if !constants.IsKnownMart(name) {
		return errorspkg.WriteJSON(c, errorspkg.ErrNotFound.WithMessage("mart not found: "+name))
	}
	v, err := h.svc.GetVersion(c.Context(), name)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	c.Set("X-Etl-Run-Id", v.EtlRunID.String())
	return c.Status(fiber.StatusOK).JSON(dto.MartVersionResponse{
		Name:        v.Name,
		EtlRunID:    v.EtlRunID,
		CommittedAt: v.CommittedAt,
	})
}
