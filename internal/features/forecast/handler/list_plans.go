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

// ListPlans — GET /v1/replenishment/plans?supplier_id=&location_id=&plan_date=&status=&limit=&cursor=
func (h *Handler) ListPlans(c fiber.Ctx) error {
	f := models.PlanFilter{Limit: constants.LimitDefault}

	if v := c.Query("supplier_id"); v != "" {
		s := v
		f.SupplierID = &s
	}
	if v := c.Query("location_id"); v != "" {
		s := v
		f.LocationID = &s
	}
	if v := c.Query("plan_date"); v != "" {
		t, err := validators.ParseDate(v)
		if err != nil {
			return mappers.MapServiceError(c, err)
		}
		f.PlanDate = &t
	}
	if v := c.Query("status"); v != "" {
		if err := validators.ValidatePlanStatus(v); err != nil {
			return mappers.MapServiceError(c, err)
		}
		s := v
		f.Status = &s
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

	rows, nextCursor, err := h.svc.ListPlans(c.Context(), f)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}

	items := make([]dto.PlanItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toPlanItem(r))
	}
	return c.Status(fiber.StatusOK).JSON(dto.ListPlansResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func toPlanItem(p models.ReplenishmentPlan) dto.PlanItem {
	return dto.PlanItem{
		ID:         p.ID,
		RunID:      p.RunID,
		SupplierID: p.SupplierID,
		LocationID: p.LocationID,
		PlanDate:   p.PlanDate.Format("2006-01-02"),
		TotalQty:   p.TotalQty,
		LinesCount: p.LinesCount,
		Status:     p.Status,
		ApprovedAt: p.ApprovedAt,
		ApprovedBy: p.ApprovedBy,
		CreatedAt:  p.CreatedAt,
	}
}
