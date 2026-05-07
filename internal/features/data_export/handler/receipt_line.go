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

// ReceiptLineRepoAPI — узкий интерфейс repository для /v1/receipt_line.
type ReceiptLineRepoAPI interface {
	SelectReceiptLine(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]repository.ReceiptLineRow, error)
}

// ReceiptLineHandler — GET /v1/receipt_line.
type ReceiptLineHandler struct {
	repo ReceiptLineRepoAPI
	snap SnapshotProvider
}

// NewReceiptLineHandler — конструктор.
func NewReceiptLineHandler(repo ReceiptLineRepoAPI, snap SnapshotProvider) *ReceiptLineHandler {
	return &ReceiptLineHandler{repo: repo, snap: snap}
}

// Get — GET /v1/receipt_line?event_date_from=&event_date_to=&cursor=&limit=.
// event_date_from / event_date_to обязательны (партиционирование).
func (h *ReceiptLineHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectReceiptLine(c.Context(), loadID, cursor.AfterPK, limit, dateFrom, dateTo)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "receipt_line", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)

	items := make([]dto.ReceiptLine, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.ReceiptLine{
			ReceiptID:     r.ReceiptID,
			LineNo:        int(r.ID),
			LocationID:    r.LocationID,
			ProductID:     r.ProductID,
			Qty:           r.Qty,
			UnitPriceBase: r.Price,
			UnitPricePaid: r.Price,
			EventDate:     r.EventDate,
			EventTime:     r.EventTime,
			LoadID:        derefOrNil(r.LoadID),
		})
	}
	return StreamNDJSON(c, items)
}
