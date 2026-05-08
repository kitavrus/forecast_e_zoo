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

// SupplyPlanRepoAPI — узкий интерфейс repository для /v1/supply_plan.
type SupplyPlanRepoAPI interface {
	SelectSupplyPlan(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.SupplyPlanRow, error)
}

// SupplyPlanHandler — GET /v1/supply_plan.
type SupplyPlanHandler struct {
	repo SupplyPlanRepoAPI
	snap SnapshotProvider
}

// NewSupplyPlanHandler — конструктор.
func NewSupplyPlanHandler(repo SupplyPlanRepoAPI, snap SnapshotProvider) *SupplyPlanHandler {
	return &SupplyPlanHandler{repo: repo, snap: snap}
}

type supplyPlanStreamItem struct {
	ID         string    `json:"id"`
	LocationID string    `json:"location_id"`
	ProductID  string    `json:"product_id"`
	SupplierID string    `json:"supplier_id"`
	PlanDate   time.Time `json:"plan_date"`
	Qty        float64   `json:"qty"`
}

// Get — GET /v1/supply_plan?cursor=&limit=.
func (h *SupplyPlanHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectSupplyPlan(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "supply_plan", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)

	items := make([]supplyPlanStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, supplyPlanStreamItem{
			ID:         r.ID,
			LocationID: r.LocationID,
			ProductID:  r.ProductID,
			SupplierID: r.SupplierID,
			PlanDate:   r.PlanDate,
			Qty:        r.Qty,
		})
	}
	return StreamNDJSON(c, items)
}
