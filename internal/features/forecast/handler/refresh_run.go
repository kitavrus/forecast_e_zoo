package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// RefreshRun — POST /v1/forecast/runs/refresh (admin-cli).
//
// Возвращает 202 если запущен, 409 если другой run в процессе,
// 503 если scheduler не сконфигурирован.
func (h *Handler) RefreshRun(c fiber.Ctx) error {
	var req dto.RefreshRunRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().JSON(&req); err != nil {
			return mappers.MapServiceError(c,
				errorspkg.ErrBadRequest.WithMessage("invalid json body"))
		}
	}
	horizon, err := validators.ValidateRefreshRequest(&req)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}

	runID, started, err := h.svc.TriggerRefresh(c.Context(), horizon)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	if !started {
		return mappers.MapServiceError(c, errorspkg.ErrForecastRunInProgress)
	}
	return c.Status(fiber.StatusAccepted).JSON(dto.RefreshRunResponse{
		RunID:       runID,
		Started:     true,
		HorizonDays: horizon,
	})
}
