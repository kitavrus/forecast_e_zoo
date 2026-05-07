package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/channels/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/channels/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// UpsertConfig — PUT /v1/channels/configs/:supplier_id (admin-cli).
//
// @Summary Создать/обновить channel config для поставщика
// @Tags channels
// @Accept json
// @Produce json
// @Param supplier_id path string true "Supplier ID"
// @Param body body dto.UpsertChannelConfigRequest true "Channel config"
// @Success 200 {object} dto.ChannelConfigItem
// @Failure 400 {object} errorspkg.ErrorResponse
// @Failure 401 {object} errorspkg.ErrorResponse
// @Failure 403 {object} errorspkg.ErrorResponse
// @Router /v1/channels/configs/{supplier_id} [put].
func (h *Handler) UpsertConfig(c fiber.Ctx) error {
	supplierID := c.Params("supplier_id")
	if supplierID == "" {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("supplier_id is required"))
	}
	var req dto.UpsertChannelConfigRequest
	if err := c.Bind().JSON(&req); err != nil {
		return mappers.MapServiceError(c, errorspkg.ErrBadRequest.WithMessage("invalid json body"))
	}
	in, err := validators.ValidateUpsertConfig(supplierID, &req)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	out, err := h.svc.UpsertConfig(c.Context(), in)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(toConfigItem(out))
}
