package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// LifecycleEventsRepoAPI — узкий интерфейс repository для /v1/store_assortment/lifecycle_events.
type LifecycleEventsRepoAPI interface {
	SelectLifecycleEvents(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.LifecycleEventReadRow, error)
}

// LifecycleEventsHandler — GET /v1/store_assortment/lifecycle_events.
type LifecycleEventsHandler struct {
	repo LifecycleEventsRepoAPI
	snap SnapshotProvider
}

// NewLifecycleEventsHandler — конструктор.
func NewLifecycleEventsHandler(repo LifecycleEventsRepoAPI, snap SnapshotProvider) *LifecycleEventsHandler {
	return &LifecycleEventsHandler{repo: repo, snap: snap}
}

type lifecycleEventStreamItem struct {
	ID         int64           `json:"id"`
	LocationID string          `json:"location_id"`
	ProductID  string          `json:"product_id"`
	EventType  string          `json:"event_type"`
	EventDate  time.Time       `json:"event_date"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// Get — GET /v1/store_assortment/lifecycle_events?cursor=&limit=.
func (h *LifecycleEventsHandler) Get(c fiber.Ctx) error {
	cursor, err := validators.ParseCursor(c.Query("cursor"))
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	limit, err := validators.ParseLimit(c.Query("limit"), dto.LimitDefault, dto.LimitMax)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	sp, err := h.snap.Current(c.Context())
	if err != nil {
		c.Set("Retry-After", "60")
		return errorspkg.WriteJSON(c, err)
	}
	if sp.CurrentLoadID == nil {
		c.Set("Retry-After", "60")
		return errorspkg.WriteJSON(c, errorspkg.ErrSnapshotNotReady)
	}
	loadID := *sp.CurrentLoadID

	rows, err := h.repo.SelectLifecycleEvents(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "store_assortment_lifecycle_events", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)
	// X-Next-Cursor: если страница «полная» (len == limit), вероятно есть продолжение.
	if len(rows) == limit && limit > 0 {
		last := rows[len(rows)-1]
		WriteNextCursor(c, loadID, fmt.Sprintf("%d", last.ID))
	}

	items := make([]lifecycleEventStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, lifecycleEventStreamItem{
			ID:         r.ID,
			LocationID: r.LocationID,
			ProductID:  r.ProductID,
			EventType:  r.EventType,
			EventDate:  r.EventDate,
			Payload:    json.RawMessage(r.Payload),
		})
	}
	return StreamNDJSON(c, items)
}
