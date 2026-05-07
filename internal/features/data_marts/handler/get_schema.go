package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetSchema — GET /v1/marts/:name/schema.
func (h *Handler) GetSchema(c fiber.Ctx) error {
	name := c.Params("name")
	if !constants.IsKnownMart(name) {
		return errorspkg.WriteJSON(c, errorspkg.ErrNotFound.WithMessage("mart not found: "+name))
	}
	s, err := h.svc.GetSchema(c.Context(), name)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	fields := make([]dto.MartFieldDTO, 0, len(s.Fields))
	for _, f := range s.Fields {
		fields = append(fields, dto.MartFieldDTO{Name: f.Name, Type: f.Type})
	}
	return c.Status(fiber.StatusOK).JSON(dto.MartSchemaResponse{
		Name:   s.Name,
		Fields: fields,
	})
}
