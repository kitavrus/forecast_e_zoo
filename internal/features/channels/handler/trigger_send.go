package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/channels/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/channels/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// TriggerSend — POST /v1/channels/send (admin-cli, on-demand).
//
// 202 Accepted с run_id если lock получен; 409 если другая send-операция уже идёт.
//
// @Summary Запустить отправку всех ready_to_send PO
// @Tags channels
// @Accept json
// @Produce json
// @Param body body dto.TriggerSendRequest false "Limit"
// @Success 202 {object} dto.TriggerSendResponse
// @Failure 409 {object} errorspkg.ErrorResponse
// @Failure 503 {object} errorspkg.ErrorResponse
// @Router /v1/channels/send [post].
func (h *Handler) TriggerSend(c fiber.Ctx) error {
	var req dto.TriggerSendRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().JSON(&req); err != nil {
			return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
		}
	}
	maxPOs, err := validators.ValidateTriggerSend(&req)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	runID, started, err := h.svc.TriggerSendAll(c.Context(), maxPOs)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	if !started {
		return mappers.MapServiceError(c, errorspkg.ErrChannelRoutingInProgress)
	}
	if runID == uuid.Nil {
		runID = uuid.New()
	}
	return c.Status(fiber.StatusAccepted).JSON(dto.TriggerSendResponse{
		RunID:   runID,
		Started: started,
	})
}
