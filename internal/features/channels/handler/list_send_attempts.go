package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/channels/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/channels/validators"
)

// ListSendAttempts — GET /v1/channels/send-attempts.
//
// Query params: po_id, supplier_id, status, from, to, cursor, limit.
//
// @Summary Список send-attempts
// @Tags channels
// @Produce json
// @Param po_id query string false "PO ID (UUID)"
// @Param supplier_id query string false "Supplier ID"
// @Param status query string false "Status" Enums(pending, success, failed, skipped)
// @Param from query string false "RFC3339 from"
// @Param to query string false "RFC3339 to"
// @Param cursor query string false "Cursor"
// @Param limit query int false "Limit (max 500, default 50)"
// @Success 200 {object} dto.ListSendAttemptsResponse
// @Failure 400 {object} errorspkg.ErrorResponse
// @Failure 401 {object} errorspkg.ErrorResponse
// @Failure 403 {object} errorspkg.ErrorResponse
// @Router /v1/channels/send-attempts [get].
func (h *Handler) ListSendAttempts(c fiber.Ctx) error {
	limit := 0
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	f, err := validators.ValidateListFilter(
		c.Query("po_id"),
		c.Query("supplier_id"),
		c.Query("status"),
		c.Query("from"),
		c.Query("to"),
		c.Query("cursor"),
		limit,
	)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}

	items, cursor, err := h.svc.ListSendAttempts(c.Context(), f)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.ListSendAttemptsResponse{
		Items:      toSendAttemptItems(items),
		NextCursor: cursor,
	})
}

func toSendAttemptItems(in []models.SendAttempt) []dto.SendAttemptItem {
	out := make([]dto.SendAttemptItem, 0, len(in))
	for _, a := range in {
		out = append(out, dto.SendAttemptItem{
			ID:             a.ID,
			POID:           a.POID,
			SupplierID:     a.SupplierID,
			ChannelType:    a.ChannelType,
			StartedAt:      a.StartedAt,
			FinishedAt:     a.FinishedAt,
			Status:         a.Status,
			HTTPStatusCode: a.HTTPStatusCode,
			ErrorMessage:   a.ErrorMessage,
			RetryCount:     a.RetryCount,
			ExternalRef:    a.ExternalRef,
		})
	}
	return out
}

func toSendAttemptDetail(a models.SendAttempt) dto.SendAttemptDetail {
	return dto.SendAttemptDetail{
		SendAttemptItem: dto.SendAttemptItem{
			ID:             a.ID,
			POID:           a.POID,
			SupplierID:     a.SupplierID,
			ChannelType:    a.ChannelType,
			StartedAt:      a.StartedAt,
			FinishedAt:     a.FinishedAt,
			Status:         a.Status,
			HTTPStatusCode: a.HTTPStatusCode,
			ErrorMessage:   a.ErrorMessage,
			RetryCount:     a.RetryCount,
			ExternalRef:    a.ExternalRef,
		},
		RequestBody:  a.RequestBody,
		ResponseBody: a.ResponseBody,
	}
}
