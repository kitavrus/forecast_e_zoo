package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// RefreshSnapshots — POST /v1/kpi/snapshots/refresh (admin-cli).
//
// Triggers async recompute. Возвращает 202 Accepted если запущен,
// 409 Conflict если другой run уже идёт.
func (h *Handler) RefreshSnapshots(c fiber.Ctx) error {
	var req dto.RefreshSnapshotsRequest
	// Body опционален — пустое тело допустимо.
	if len(c.Body()) > 0 {
		if err := c.Bind().JSON(&req); err != nil {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
		}
	}
	asOf, kpis, err := validators.ValidateRefreshRequest(&req)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}

	runID, started, err := h.svc.TriggerRefresh(c.Context(), asOf, kpis)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	if !started {
		return mappers.MapServiceError(c,
			errorspkg.ErrConflict.WithMessage("another kpi run is already in progress"))
	}

	effectiveKpis := kpis
	if len(effectiveKpis) == 0 {
		effectiveKpis = []string{constants.KpiOSA, constants.KpiOTIF, constants.KpiStockDays}
	}
	return c.Status(fiber.StatusAccepted).JSON(dto.RefreshSnapshotsResponse{
		RunID:    runID,
		Started:  true,
		KpiNames: effectiveKpis,
		FromDate: asOf.Format("2006-01-02"),
	})
}
