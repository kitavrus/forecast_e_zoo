package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/constants"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ListRuns — GET /v1/forecast/runs?status=&from=&to=&limit=&cursor=
func (h *Handler) ListRuns(c fiber.Ctx) error {
	f := models.RunFilter{Limit: constants.LimitDefault}

	if v := c.Query("status"); v != "" {
		if err := validators.ValidateRunStatus(v); err != nil {
			return mappers.MapServiceError(c, err)
		}
		st := v
		f.Status = &st
	}
	if v := c.Query("from"); v != "" {
		t, err := validators.ParseTimestamp(v)
		if err != nil {
			return mappers.MapServiceError(c, err)
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := validators.ParseTimestamp(v)
		if err != nil {
			return mappers.MapServiceError(c, err)
		}
		f.To = &t
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid limit"))
		}
		if n > constants.LimitMax {
			n = constants.LimitMax
		}
		f.Limit = n
	}
	f.Cursor = c.Query("cursor")

	rows, nextCursor, err := h.svc.ListRuns(c.Context(), f)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}

	items := make([]dto.RunItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toRunItem(r))
	}
	return c.Status(fiber.StatusOK).JSON(dto.ListRunsResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func toRunItem(r models.ForecastRun) dto.RunItem {
	return dto.RunItem{
		ID:               r.ID,
		StartedAt:        r.StartedAt,
		FinishedAt:       r.FinishedAt,
		Status:           r.Status,
		HorizonDays:      r.HorizonDays,
		SnapshotEtlRunID: r.SnapshotEtlRunID,
		ForecastsCount:   r.ForecastsCount,
		LinesCount:       r.LinesCount,
		PlansCount:       r.PlansCount,
		ErrorMessage:     r.ErrorMessage,
		CreatedAt:        r.CreatedAt,
	}
}
