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

// SupplierRepoAPI — узкий интерфейс repository для /v1/supplier.
type SupplierRepoAPI interface {
	SelectSupplier(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.SupplierRow, error)
}

// SupplierHandler — GET /v1/supplier.
type SupplierHandler struct {
	repo SupplierRepoAPI
	snap SnapshotProvider
}

// NewSupplierHandler — конструктор.
func NewSupplierHandler(repo SupplierRepoAPI, snap SnapshotProvider) *SupplierHandler {
	return &SupplierHandler{repo: repo, snap: snap}
}

// supplierStreamItem — публичный JSON-shape для ETL extractor
// (stg_suppliers: supplier_id, name, status). status derived.
type supplierStreamItem struct {
	SupplierID string `json:"supplier_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
}

// Get — GET /v1/supplier?cursor=&limit=.
func (h *SupplierHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectSupplier(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "supplier", derefOrZeroTime(sp.CommittedAt))
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

	items := make([]supplierStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, supplierStreamItem{
			SupplierID: r.ID,
			Name:       r.Name,
			Status:     "active",
		})
	}
	return StreamNDJSON(c, items)
}
