package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/channels/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetSendAttempt — GET /v1/channels/send-attempts/:id.
//
// @Summary Детали send-attempt с request/response logs
// @Tags channels
// @Produce json
// @Param id path string true "Send attempt ID (UUID)"
// @Success 200 {object} dto.SendAttemptDetailResponse
// @Failure 400 {object} errorspkg.ErrorResponse
// @Failure 404 {object} errorspkg.ErrorResponse
// @Router /v1/channels/send-attempts/{id} [get].
func (h *Handler) GetSendAttempt(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid id"))
	}
	a, err := h.svc.GetSendAttempt(c.Context(), id)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.SendAttemptDetailResponse{
		Attempt: toSendAttemptDetail(a),
	})
}
