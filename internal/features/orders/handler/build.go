package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/orders/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/orders/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// BuildOnDemand — POST /v1/orders/purchase-orders/build (admin-cli).
//
// Возвращает 202 Accepted (started=true) с run_id если lock получен;
// 409 если другая build-операция уже идёт.
func (h *Handler) BuildOnDemand(c fiber.Ctx) error {
	var req dto.BuildRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().JSON(&req); err != nil {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
		}
	}
	maxPlans, err := validators.ValidateBuildRequest(&req)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	runID, started, err := h.svc.TriggerBuild(c.Context(), maxPlans)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	if !started {
		return mappers.MapServiceError(c, errorspkg.ErrOrderBuilderInProgress)
	}
	if runID == uuid.Nil {
		runID = uuid.New()
	}
	return c.Status(fiber.StatusAccepted).JSON(dto.BuildResponse{
		RunID:   runID,
		Started: started,
	})
}
