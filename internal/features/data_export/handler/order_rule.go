package handler

import (
	"context"
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// OrderRuleRepoAPI — узкий интерфейс repository для /v1/order_rule.
type OrderRuleRepoAPI interface {
	SelectOrderRuleFanout(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.OrderRuleFannedRow, error)
}

// OrderRuleHandler — GET /v1/order_rule.
type OrderRuleHandler struct {
	repo OrderRuleRepoAPI
	snap SnapshotProvider
}

// NewOrderRuleHandler — конструктор.
func NewOrderRuleHandler(repo OrderRuleRepoAPI, snap SnapshotProvider) *OrderRuleHandler {
	return &OrderRuleHandler{repo: repo, snap: snap}
}

// orderRuleStreamItem — публичный JSON-shape, потребляемый ETL extractor-ом
// (см. etl_validation/service/staging.go колонки stg_order_rule).
//
// product_id всегда set (репозиторий fan-out'ит location/category-wide правила
// в per-product строки). scope = rule_type, scope_ref = product_id.
// safety_stock_days / service_level_pct / override_moq — из payload.
type orderRuleStreamItem struct {
	RuleID          string   `json:"rule_id"`
	Scope           string   `json:"scope"`
	ScopeRef        *string  `json:"scope_ref,omitempty"`
	ProductID       string   `json:"product_id"`
	LocationID      string   `json:"location_id"`
	SafetyStockDays *float64 `json:"safety_stock_days,omitempty"`
	ServiceLevelPct *float64 `json:"service_level_pct,omitempty"`
	OverrideMOQ     *int     `json:"override_moq,omitempty"`
}

// Get — GET /v1/order_rule?cursor=&limit=.
func (h *OrderRuleHandler) Get(c fiber.Ctx) error {
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

	rows, err := h.repo.SelectOrderRuleFanout(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "order_rule", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)

	items := make([]orderRuleStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapOrderRuleRow(r))
	}
	return StreamNDJSON(c, items)
}

// mapOrderRuleRow конвертирует fanned-out repo-row в публичный stream-item.
// scope_ref = product_id (после fan-out всегда set). payload — best-effort.
func mapOrderRuleRow(r repository.OrderRuleFannedRow) orderRuleStreamItem {
	productID := r.ProductID
	out := orderRuleStreamItem{
		RuleID:     r.RuleID + "-" + r.ProductID,
		Scope:      r.RuleType,
		ScopeRef:   &productID,
		ProductID:  r.ProductID,
		LocationID: r.LocationID,
	}

	if len(r.Payload) > 0 {
		var pl orderRulePayload
		if err := json.Unmarshal(r.Payload, &pl); err == nil {
			if pl.Days != nil {
				v := *pl.Days
				out.SafetyStockDays = &v
			}
			if pl.SafetyStockDays != nil {
				out.SafetyStockDays = pl.SafetyStockDays
			}
			out.ServiceLevelPct = pl.ServiceLevelPct
			out.OverrideMOQ = pl.OverrideMOQ
		}
	}
	return out
}

// orderRulePayload — известные поля jsonb.payload, которые мы маппим в
// публичный API. Лишние ключи игнорируются.
type orderRulePayload struct {
	Days            *float64 `json:"days,omitempty"`
	SafetyStockDays *float64 `json:"safety_stock_days,omitempty"`
	ServiceLevelPct *float64 `json:"service_level_pct,omitempty"`
	OverrideMOQ     *int     `json:"override_moq,omitempty"`
}
