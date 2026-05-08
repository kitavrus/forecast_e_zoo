package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// StockMovementRepoAPI — узкий интерфейс repository для /v1/stock_movement.
type StockMovementRepoAPI interface {
	SelectStockMovement(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]repository.StockMovementRow, error)
}

// StockMovementHandler — GET /v1/stock_movement.
type StockMovementHandler struct {
	repo StockMovementRepoAPI
	snap SnapshotProvider
}

// NewStockMovementHandler — конструктор.
func NewStockMovementHandler(repo StockMovementRepoAPI, snap SnapshotProvider) *StockMovementHandler {
	return &StockMovementHandler{repo: repo, snap: snap}
}

type stockMovementStreamItem struct {
	ID           int64     `json:"id"`
	EventDate    time.Time `json:"event_date"`
	EventTime    time.Time `json:"event_time"`
	LocationID   string    `json:"location_id"`
	ProductID    string    `json:"product_id"`
	MovementType string    `json:"movement_type"`
	Qty          float64   `json:"qty"`
	RefID        *string   `json:"ref_id,omitempty"`
}

// Get — GET /v1/stock_movement?event_date_from=&event_date_to=&cursor=&limit=.
func (h *StockMovementHandler) Get(c fiber.Ctx) error {
	from := c.Query("event_date_from")
	to := c.Query("event_date_to")
	if from == "" || to == "" {
		return errorspkg.WriteJSON(c, errorspkg.ErrInvalidQuery.WithMessage(
			"event_date_from and event_date_to are required"))
	}
	dateFrom, dateTo, err := validators.ParseEventDateRange(from, to)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
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

	rows, err := h.repo.SelectStockMovement(c.Context(), loadID, cursor.AfterPK, limit, dateFrom, dateTo)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "stock_movement", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)
	// X-Next-Cursor: если страница «полная» (len == limit), вероятно есть продолжение.
	if len(rows) == limit && limit > 0 {
		last := rows[len(rows)-1]
		WriteNextCursor(c, loadID, fmt.Sprintf("%s|%d", last.EventDate.UTC().Format("2006-01-02"), last.ID))
	}

	items := make([]stockMovementStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, stockMovementStreamItem{
			ID:           r.ID,
			EventDate:    r.EventDate,
			EventTime:    r.EventTime,
			LocationID:   r.LocationID,
			ProductID:    r.ProductID,
			MovementType: r.MovementType,
			Qty:          r.Qty,
			RefID:        r.RefID,
		})
	}
	return StreamNDJSON(c, items)
}
