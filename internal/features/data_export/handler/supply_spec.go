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

// SupplySpecRepoAPI — узкий интерфейс repository для /v1/supply_spec.
type SupplySpecRepoAPI interface {
	SelectSupplySpecFanout(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.SupplySpecFannedRow, error)
}

// SupplySpecHandler — GET /v1/supply_spec.
type SupplySpecHandler struct {
	repo SupplySpecRepoAPI
	snap SnapshotProvider
}

// NewSupplySpecHandler — конструктор.
func NewSupplySpecHandler(repo SupplySpecRepoAPI, snap SnapshotProvider) *SupplySpecHandler {
	return &SupplySpecHandler{repo: repo, snap: snap}
}

// supplySpecStreamItem — публичный JSON-shape для ETL extractor
// (stg_supply_spec columns).
//
// LocationID после CROSS JOIN с location всегда set (см. SelectSupplySpecFanout).
// lead_time_days/min_order_qty/pack_size — int (numeric→int).
type supplySpecStreamItem struct {
	SupplierID    string   `json:"supplier_id"`
	ProductID     string   `json:"product_id"`
	LocationID    string   `json:"location_id"`
	LeadTimeDays  *int     `json:"lead_time_days,omitempty"`
	MinOrderQty   *int     `json:"min_order_qty,omitempty"`
	PurchasePrice *float64 `json:"purchase_price,omitempty"`
	Currency      *string  `json:"currency,omitempty"`
	PackSize      *int     `json:"pack_size,omitempty"`
}

// Get — GET /v1/supply_spec?cursor=&limit=.
func (h *SupplySpecHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectSupplySpecFanout(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "supply_spec", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)

	items := make([]supplySpecStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapSupplySpecRow(r))
	}
	return StreamNDJSON(c, items)
}

// mapSupplySpecRow конвертирует fanned-out repo-row в публичный stream-item.
func mapSupplySpecRow(r repository.SupplySpecFannedRow) supplySpecStreamItem {
	out := supplySpecStreamItem{
		SupplierID:   r.SupplierID,
		ProductID:    r.ProductID,
		LocationID:   r.LocationID,
		LeadTimeDays: r.LeadTimeDays,
	}
	if r.MinOrderQty != nil {
		v := int(*r.MinOrderQty)
		out.MinOrderQty = &v
	}
	if r.PackQty != nil {
		v := int(*r.PackQty)
		out.PackSize = &v
	}
	return out
}
