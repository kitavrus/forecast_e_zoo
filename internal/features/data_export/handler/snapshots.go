package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// SnapshotProvider — интерфейс snapshot-сервиса для handler.
type SnapshotProvider interface {
	Current(ctx context.Context) (models.SnapshotPointer, error)
}

// SnapshotsHandler — GET /v1/snapshots/current.
type SnapshotsHandler struct {
	svc SnapshotProvider
}

// NewSnapshotsHandler — конструктор.
func NewSnapshotsHandler(svc SnapshotProvider) *SnapshotsHandler {
	return &SnapshotsHandler{svc: svc}
}

// GetCurrent — GET /v1/snapshots/current.
func (h *SnapshotsHandler) GetCurrent(c fiber.Ctx) error {
	sp, err := h.svc.Current(c.Context())
	if err != nil {
		// snapshot not ready → 503 + Retry-After.
		c.Set("Retry-After", "60")
		return errorspkg.WriteJSON(c, err)
	}
	resp := dto.GetSnapshotsCurrentResponse{
		SnapshotID:  derefOrNil(sp.CurrentLoadID),
		CommittedAt: derefOrZeroTime(sp.CommittedAt),
		Entities:    nil, // оставляем пустым; счётчики появятся после реализации Stats (вне MVP)
	}
	WritePageHeaders(c, resp.SnapshotID, resp.SnapshotID, "")
	return c.Status(fiber.StatusOK).JSON(resp)
}

func derefOrNil(p *uuid.UUID) uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return *p
}

func derefOrZeroTime(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}
