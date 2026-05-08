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

// CategoryRepoAPI — узкий интерфейс repository для /v1/category.
type CategoryRepoAPI interface {
	SelectCategory(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.CategoryRow, error)
}

// CategoryHandler — GET /v1/category.
type CategoryHandler struct {
	repo CategoryRepoAPI
	snap SnapshotProvider
}

// NewCategoryHandler — конструктор.
func NewCategoryHandler(repo CategoryRepoAPI, snap SnapshotProvider) *CategoryHandler {
	return &CategoryHandler{repo: repo, snap: snap}
}

// categoryStreamItem — публичный JSON-shape для ETL extractor (stg_category, не используется
// напрямую в mart_calculation_input, но включён в AllowedEntities).
type categoryStreamItem struct {
	CategoryID string  `json:"category_id"`
	Name       string  `json:"name"`
	ParentID   *string `json:"parent_id,omitempty"`
}

// Get — GET /v1/category?cursor=&limit=.
func (h *CategoryHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectCategory(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "category", derefOrZeroTime(sp.CommittedAt))
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

	items := make([]categoryStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, categoryStreamItem{
			CategoryID: r.ID,
			Name:       r.Name,
			ParentID:   r.ParentID,
		})
	}
	return StreamNDJSON(c, items)
}
