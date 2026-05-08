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

// LocationStockSnapshotRepoAPI — узкий интерфейс repository для
// /v1/location_stock_snapshot.
type LocationStockSnapshotRepoAPI interface {
	SelectLocationStockSnapshot(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]repository.LocationStockSnapshotRow, error)
}

// LocationStockSnapshotHandler — GET /v1/location_stock_snapshot.
type LocationStockSnapshotHandler struct {
	repo LocationStockSnapshotRepoAPI
	snap SnapshotProvider
}

// NewLocationStockSnapshotHandler — конструктор.
func NewLocationStockSnapshotHandler(repo LocationStockSnapshotRepoAPI, snap SnapshotProvider) *LocationStockSnapshotHandler {
	return &LocationStockSnapshotHandler{repo: repo, snap: snap}
}

// locationStockSnapshotStreamItem — публичный JSON-shape для ETL extractor
// (stg_stock_on_hand columns: product_id, location_id, qty_on_hand).
// EventDate / AsOf отдаём для корректной date-фильтрации на стороне ETL.
type locationStockSnapshotStreamItem struct {
	EventDate   time.Time `json:"event_date"`
	LocationID  string    `json:"location_id"`
	ProductID   string    `json:"product_id"`
	QtyOnHand   float64   `json:"qty_on_hand"`
	QtyReserved float64   `json:"qty_reserved"`
	AsOf        time.Time `json:"as_of"`
}

// Get — GET /v1/location_stock_snapshot?event_date_from=&event_date_to=&cursor=&limit=.
// event_date_from / event_date_to обязательны (партиционирование).
func (h *LocationStockSnapshotHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectLocationStockSnapshot(c.Context(), loadID, cursor.AfterPK, limit, dateFrom, dateTo)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "location_stock_snapshot", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)

	items := make([]locationStockSnapshotStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, locationStockSnapshotStreamItem{
			EventDate:   r.EventDate,
			LocationID:  r.LocationID,
			ProductID:   r.ProductID,
			QtyOnHand:   r.QtyOnHand,
			QtyReserved: r.QtyReserved,
			AsOf:        r.AsOf,
		})
	}
	return StreamNDJSON(c, items)
}
