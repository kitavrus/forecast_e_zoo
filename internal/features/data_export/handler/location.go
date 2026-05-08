package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// LocationRepoAPI — узкий интерфейс repository для /v1/location.
type LocationRepoAPI interface {
	SelectLocation(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.LocationRow, error)
}

// LocationHandler — GET /v1/location.
type LocationHandler struct {
	repo LocationRepoAPI
	snap SnapshotProvider
}

// NewLocationHandler — конструктор.
func NewLocationHandler(repo LocationRepoAPI, snap SnapshotProvider) *LocationHandler {
	return &LocationHandler{repo: repo, snap: snap}
}

// locationStreamItem — публичный JSON-shape для ETL extractor (stg_locations:
// location_id, name, status). status — derived (active = is_active).
type locationStreamItem struct {
	LocationID string `json:"location_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

// Get — GET /v1/location?cursor=&limit=.
func (h *LocationHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectLocation(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "location", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)
	// X-Next-Cursor: если страница «полная» (len == limit), вероятно есть продолжение.
	if len(rows) == limit && limit > 0 {
		last := rows[len(rows)-1]
		WriteNextCursor(c, loadID, last.ID)
	}

	items := make([]locationStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, locationStreamItem{
			LocationID: r.ID,
			Name:       r.Name,
			Status:     "active",
		})
	}
	return StreamNDJSON(c, items)
}
