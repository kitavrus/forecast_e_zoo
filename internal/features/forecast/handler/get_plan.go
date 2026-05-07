package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetPlan — GET /v1/replenishment/plans/:id (план + его lines).
func (h *Handler) GetPlan(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	pwl, err := h.svc.GetPlanWithLines(c.Context(), id)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	lines := make([]dto.CalculationLineItem, 0, len(pwl.Lines))
	for _, l := range pwl.Lines {
		lines = append(lines, toLineItem(l))
	}
	return c.Status(fiber.StatusOK).JSON(dto.PlanWithLinesResponse{
		Plan:  toPlanItem(pwl.Plan),
		Lines: lines,
	})
}

func toLineItem(l models.CalculationLine) dto.CalculationLineItem {
	return dto.CalculationLineItem{
		ID:           l.ID,
		ProductID:    l.ProductID,
		LocationID:   l.LocationID,
		SupplierID:   l.SupplierID,
		CurrentStock: l.CurrentStock,
		InTransit:    l.InTransit,
		DailyDemand:  l.DailyDemand,
		LeadTimeDays: l.LeadTimeDays,
		SafetyStock:  l.SafetyStock,
		ReorderPoint: l.ReorderPoint,
		TargetStock:  l.TargetStock,
		ReorderQty:   l.ReorderQty,
	}
}
