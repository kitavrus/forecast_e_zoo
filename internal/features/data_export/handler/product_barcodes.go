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

// ProductBarcodesRepoAPI — узкий интерфейс repository для /v1/product_barcodes.
type ProductBarcodesRepoAPI interface {
	SelectProductBarcodes(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.ProductBarcodeRow, error)
}

// ProductBarcodesHandler — GET /v1/product_barcodes.
type ProductBarcodesHandler struct {
	repo ProductBarcodesRepoAPI
	snap SnapshotProvider
}

// NewProductBarcodesHandler — конструктор.
func NewProductBarcodesHandler(repo ProductBarcodesRepoAPI, snap SnapshotProvider) *ProductBarcodesHandler {
	return &ProductBarcodesHandler{repo: repo, snap: snap}
}

// productBarcodeStreamItem — публичный JSON-shape.
type productBarcodeStreamItem struct {
	ProductID string `json:"product_id"`
	Barcode   string `json:"barcode"`
	IsPrimary bool   `json:"is_primary"`
}

// Get — GET /v1/product_barcodes?cursor=&limit=.
func (h *ProductBarcodesHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectProductBarcodes(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "product_barcodes", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)
	// X-Next-Cursor: если страница «полная» (len == limit), вероятно есть продолжение.
	if len(rows) == limit && limit > 0 {
		last := rows[len(rows)-1]
		WriteNextCursor(c, loadID, last.ProductID + "|" + last.Barcode)
	}

	items := make([]productBarcodeStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, productBarcodeStreamItem{
			ProductID: r.ProductID,
			Barcode:   r.Barcode,
			IsPrimary: r.IsPrimary,
		})
	}
	return StreamNDJSON(c, items)
}
