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

// ProductsRepoAPI — узкий интерфейс repository для /v1/products.
type ProductsRepoAPI interface {
	SelectProducts(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.ProductRow, error)
}

// ProductsHandler — GET /v1/products.
type ProductsHandler struct {
	repo ProductsRepoAPI
	snap SnapshotProvider
}

// NewProductsHandler — конструктор.
func NewProductsHandler(repo ProductsRepoAPI, snap SnapshotProvider) *ProductsHandler {
	return &ProductsHandler{repo: repo, snap: snap}
}

// Get — GET /v1/products?cursor=&limit=.
func (h *ProductsHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectProducts(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	// ETag — на (load_id, entity, lastUpdated). Здесь lastUpdated = committed_at.
	etag := ComputeETag(loadID, "products", derefOrZeroTime(sp.CommittedAt))
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

	items := make([]dto.Product, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.Product{
			ProductID: r.ID,
			Name:      r.Name,
			Status: func() string {
				if r.IsActive {
					return "active"
				}
				return "archived"
			}(),
			LoadID: derefOrNil(r.LoadID),
		})
	}
	return StreamNDJSON(c, items)
}
