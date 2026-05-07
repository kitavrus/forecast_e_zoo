package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/channels/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
)

// ListConfigs — GET /v1/channels/configs (admin).
//
// @Summary Список channel configs
// @Tags channels
// @Produce json
// @Success 200 {object} dto.ListChannelConfigsResponse
// @Failure 401 {object} errorspkg.ErrorResponse
// @Failure 403 {object} errorspkg.ErrorResponse
// @Router /v1/channels/configs [get].
func (h *Handler) ListConfigs(c fiber.Ctx) error {
	items, err := h.svc.ListConfigs(c.Context())
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.ListChannelConfigsResponse{
		Items: toConfigItems(items),
	})
}

func toConfigItems(in []models.SupplierChannelConfig) []dto.ChannelConfigItem {
	out := make([]dto.ChannelConfigItem, 0, len(in))
	for _, c := range in {
		out = append(out, dto.ChannelConfigItem{
			SupplierID:         c.SupplierID,
			ChannelType:        c.ChannelType,
			EndpointURL:        c.EndpointURL,
			AuthMode:           c.AuthMode,
			AuthCredentialsRef: c.AuthCredentialsRef,
			TimeoutSec:         c.TimeoutSec,
			RetryMax:           c.RetryMax,
			IsActive:           c.IsActive,
			CreatedAt:          c.CreatedAt,
			UpdatedAt:          c.UpdatedAt,
		})
	}
	return out
}

func toConfigItem(c models.SupplierChannelConfig) dto.ChannelConfigItem {
	return dto.ChannelConfigItem{
		SupplierID:         c.SupplierID,
		ChannelType:        c.ChannelType,
		EndpointURL:        c.EndpointURL,
		AuthMode:           c.AuthMode,
		AuthCredentialsRef: c.AuthCredentialsRef,
		TimeoutSec:         c.TimeoutSec,
		RetryMax:           c.RetryMax,
		IsActive:           c.IsActive,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	}
}
