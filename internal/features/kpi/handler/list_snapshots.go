package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ListSnapshots — GET /v1/kpi/snapshots?as_of_date=&kpi_name=&scope_type=&scope_id=&limit=&cursor=
func (h *Handler) ListSnapshots(c fiber.Ctx) error {
	f := models.SnapshotFilter{Limit: constants.LimitDefault}

	if v := c.Query("as_of_date"); v != "" {
		t, err := validators.ValidateAsOfDate(v)
		if err != nil {
			return mappers.MapServiceError(c, err)
		}
		f.AsOfDate = &t
	}
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
	if v := c.Query("scope_id"); v != "" {
		sid := v
		f.ScopeID = &sid
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

	rows, nextCursor, err := h.svc.ListSnapshots(c.Context(), f)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}

	items := make([]dto.SnapshotItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toSnapshotItem(r))
	}
	return c.Status(fiber.StatusOK).JSON(dto.ListSnapshotsResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func toSnapshotItem(r models.KpiSnapshot) dto.SnapshotItem {
	return dto.SnapshotItem{
		ID:            r.ID,
		AsOfDate:      r.AsOfDate.Format("2006-01-02"),
		KpiName:       r.KpiName,
		ScopeType:     r.ScopeType,
		ScopeID:       r.ScopeID,
		Value:         r.Value,
		CalibrationID: r.CalibrationID,
		ComputedAt:    r.ComputedAt,
		EtlRunID:      r.EtlRunID,
	}
}
