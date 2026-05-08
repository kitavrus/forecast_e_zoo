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

// SupplierStockSnapshotRepoAPI — узкий интерфейс repository для /v1/supplier_stock_snapshot.
type SupplierStockSnapshotRepoAPI interface {
	SelectSupplierStockSnapshot(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]repository.SupplierStockSnapshotRow, error)
}

// SupplierStockSnapshotHandler — GET /v1/supplier_stock_snapshot.
type SupplierStockSnapshotHandler struct {
	repo SupplierStockSnapshotRepoAPI
	snap SnapshotProvider
}

// NewSupplierStockSnapshotHandler — конструктор.
func NewSupplierStockSnapshotHandler(repo SupplierStockSnapshotRepoAPI, snap SnapshotProvider) *SupplierStockSnapshotHandler {
	return &SupplierStockSnapshotHandler{repo: repo, snap: snap}
}

type supplierStockSnapshotStreamItem struct {
	EventDate    time.Time `json:"event_date"`
	SupplierID   string    `json:"supplier_id"`
	ProductID    string    `json:"product_id"`
	QtyAvailable float64   `json:"qty_available"`
	AsOf         time.Time `json:"as_of"`
}

// Get — GET /v1/supplier_stock_snapshot?event_date_from=&event_date_to=&cursor=&limit=.
func (h *SupplierStockSnapshotHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectSupplierStockSnapshot(c.Context(), loadID, cursor.AfterPK, limit, dateFrom, dateTo)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "supplier_stock_snapshot", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)
	// X-Next-Cursor: если страница «полная» (len == limit), вероятно есть продолжение.
	if len(rows) == limit && limit > 0 {
		last := rows[len(rows)-1]
		WriteNextCursor(c, loadID, fmt.Sprintf("%s|%s|%s", last.EventDate.UTC().Format("2006-01-02"), last.SupplierID, last.ProductID))
	}

	items := make([]supplierStockSnapshotStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, supplierStockSnapshotStreamItem{
			EventDate:    r.EventDate,
			SupplierID:   r.SupplierID,
			ProductID:    r.ProductID,
			QtyAvailable: r.QtyAvailable,
			AsOf:         r.AsOf,
		})
	}
	return StreamNDJSON(c, items)
}
