package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/validators"
)

// ListCalibrations — GET /v1/kpi/calibrations?kpi_name=&scope_type=
func (h *Handler) ListCalibrations(c fiber.Ctx) error {
	f := models.CalibrationFilter{}
	if v := c.Query("kpi_name"); v != "" {
		if err := validators.ValidateKpiName(v); err != nil {
			return mappers.MapServiceError(c, err)
		}
		k := v
		f.KpiName = &k
	}
	if v := c.Query("scope_type"); v != "" {
		if err := validators.ValidateScopeType(v); err != nil {
			return mappers.MapServiceError(c, err)
		}
		st := v
		f.ScopeType = &st
	}

	rows, err := h.svc.ListCalibrations(c.Context(), f)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	items := make([]dto.CalibrationItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.CalibrationItem{
			ID:        r.ID,
			KpiName:   r.KpiName,
			ScopeType: r.ScopeType,
			ScopeID:   r.ScopeID,
			Params:    r.Params,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return c.Status(fiber.StatusOK).JSON(dto.ListCalibrationsResponse{Items: items})
}
