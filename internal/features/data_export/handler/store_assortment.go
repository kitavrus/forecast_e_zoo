package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// StoreAssortmentRepoAPI — узкий интерфейс repository для /v1/store_assortment.
type StoreAssortmentRepoAPI interface {
	SelectStoreAssortment(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.StoreAssortmentRow, error)
}

// StoreAssortmentHandler — GET /v1/store_assortment.
type StoreAssortmentHandler struct {
	repo StoreAssortmentRepoAPI
	snap SnapshotProvider
}

// NewStoreAssortmentHandler — конструктор.
func NewStoreAssortmentHandler(repo StoreAssortmentRepoAPI, snap SnapshotProvider) *StoreAssortmentHandler {
	return &StoreAssortmentHandler{repo: repo, snap: snap}
}

// storeAssortmentStreamItem — JSON-форма строки store_assortment.
// Имена полей строго совпадают с dto.StoreAssortment (single source of truth
// для downstream ETL: stg_store_assortment.effective_from / effective_to /
// lifecycle_state).
type storeAssortmentStreamItem struct {
	LocationID     string     `json:"location_id"`
	ProductID      string     `json:"product_id"`
	LifecycleState string     `json:"lifecycle_state"`
	EffectiveFrom  time.Time  `json:"effective_from"`
	EffectiveTo    *time.Time `json:"effective_to,omitempty"`
}

// Get — GET /v1/store_assortment?cursor=&limit=.
func (h *StoreAssortmentHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectStoreAssortment(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "store_assortment", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)

	items := make([]storeAssortmentStreamItem, 0, len(rows))
	for _, r := range rows {
		state := "active"
		if !r.IsActive {
			state = "inactive"
		}
		items = append(items, storeAssortmentStreamItem{
			LocationID:     r.LocationID,
			ProductID:      r.ProductID,
			LifecycleState: state,
			EffectiveFrom:  r.StartDate,
			EffectiveTo:    r.EndDate,
		})
	}
	return StreamNDJSON(c, items)
}
