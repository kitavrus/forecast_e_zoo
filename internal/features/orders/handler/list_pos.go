package handler

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/orders/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ListPOs — GET /v1/orders/purchase-orders.
//
//nolint:cyclop // multi-filter parsing
func (h *Handler) ListPOs(c fiber.Ctx) error {
	f := models.POFilter{Limit: constants.LimitDefault}

	if v := c.Query("status"); v != "" {
		if err := validators.ValidatePOStatus(v); err != nil {
			return mappers.MapServiceError(c, err)
		}
		f.Status = &v
	}
	if v := c.Query("supplier_id"); v != "" {
		f.SupplierID = &v
	}
	if v := c.Query("plan_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid plan_id"))
		}
		f.PlanID = &id
	}
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid from"))
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid to"))
		}
		f.To = &t
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid limit"))
		}
		f.Limit = n
	}
	if v := c.Query("cursor"); v != "" {
		f.Cursor = v
	}

	items, next, err := h.svc.ListPOs(c.Context(), f)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	resp := dto.ListPurchaseOrdersResponse{
		Items:      make([]dto.PurchaseOrderItem, 0, len(items)),
		NextCursor: next,
	}
	for _, po := range items {
		resp.Items = append(resp.Items, toPurchaseOrderItem(po))
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}
