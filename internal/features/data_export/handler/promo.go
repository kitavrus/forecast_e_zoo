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

// PromoRepoAPI — узкий интерфейс repository для /v1/promo.
type PromoRepoAPI interface {
	SelectPromo(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.PromoRow, error)
}

// PromoHandler — GET /v1/promo.
type PromoHandler struct {
	repo PromoRepoAPI
	snap SnapshotProvider
}

// NewPromoHandler — конструктор.
func NewPromoHandler(repo PromoRepoAPI, snap SnapshotProvider) *PromoHandler {
	return &PromoHandler{repo: repo, snap: snap}
}

// promoStreamItem — JSON-форма строки promo.
// Имена полей строго совпадают с dto.Promo (single source of truth для
// downstream ETL: stg_promo.promo_id / date_from / date_to / type).
type promoStreamItem struct {
	PromoID     string    `json:"promo_id"`
	ProductID   string    `json:"product_id"`
	LocationID  string    `json:"location_id,omitempty"`
	Type        string    `json:"type"`
	DiscountPct *float64  `json:"discount_pct,omitempty"`
	DateFrom    time.Time `json:"date_from"`
	DateTo      time.Time `json:"date_to"`
}

// Get — GET /v1/promo?cursor=&limit=.
func (h *PromoHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectPromo(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "promo", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)

	items := make([]promoStreamItem, 0, len(rows))
	for _, r := range rows {
		// Type по умолчанию — "discount" (downstream ETL не падает, mart-ы
		// используют bool_or(p.promo_id IS NOT NULL) AS had_promo, не type).
		// Если в Payload есть {"type": "..."} — она перезаписывает default.
		ptype := "discount"
		items = append(items, promoStreamItem{
			PromoID:     r.ID,
			ProductID:   r.ProductID,
			LocationID:  r.LocationID,
			Type:        ptype,
			DiscountPct: r.DiscountPct,
			DateFrom:    r.StartDate,
			DateTo:      r.EndDate,
		})
	}
	return StreamNDJSON(c, items)
}
