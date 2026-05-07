package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/channels/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Retry — POST /v1/channels/send/:po_id/retry (admin-cli).
//
// Возвращает 200 с status='success' (idempotent: если уже отправлен — возвращает existing).
// 404 если PO не найден; 409 если PO не в ready_to_send И не было успешного attempt.
//
// @Summary Повторно отправить конкретный PO
// @Tags channels
// @Accept json
// @Produce json
// @Param po_id path string true "PO ID (UUID)"
// @Success 200 {object} dto.RetryResponse
// @Failure 404 {object} errorspkg.ErrorResponse
// @Failure 409 {object} errorspkg.ErrorResponse
// @Failure 503 {object} errorspkg.ErrorResponse
// @Router /v1/channels/send/{po_id}/retry [post].
func (h *Handler) Retry(c fiber.Ctx) error {
	idStr := c.Params("po_id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid po_id"))
	}
	attemptID, status, ext, err := h.svc.RetryByID(c.Context(), id)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.RetryResponse{
		AttemptID:   attemptID,
		Status:      status,
		ExternalRef: ext,
	})
}
