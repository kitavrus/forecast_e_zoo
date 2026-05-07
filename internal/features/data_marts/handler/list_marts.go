package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models/dto"
)

// List — GET /v1/marts.
//
// Возвращает список всех mart'ов + текущую версию для каждого.
// Если mart не наполнен (нет committed run'а) — populated=false, EtlRunID/CommittedAt пустые.
func (h *Handler) List(c fiber.Ctx) error {
	infos, err := h.svc.List(c.Context())
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	items := make([]dto.MartInfoItem, 0, len(infos))
	for _, info := range infos {
		populated := info.EtlRunID != uuid.Nil
		items = append(items, dto.MartInfoItem{
			Name:        info.Name,
			EtlRunID:    info.EtlRunID,
			CommittedAt: info.CommittedAt,
			Populated:   populated,
		})
	}
	return c.Status(fiber.StatusOK).JSON(dto.ListMartsResponse{Marts: items})
}
