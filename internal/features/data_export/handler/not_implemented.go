package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// NotImplementedHandlers — заглушки для эндпоинтов, чьи repository-методы
// не реализованы в MVP (12 master-сущностей вне products + facts вне
// receipt_line). Возвращают 501 с понятным сообщением — это явно лучше,
// чем тихий 404 или fake-ответ.
//
// План: каждый из этих хендлеров будет получать собственный repository-метод
// в фазе пост-MVP, как только подключится та или иная сущность.
type NotImplementedHandlers struct{}

// Make returns a fiber.Handler that always responds 501.
func notImplemented(entity string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return errorspkg.WriteJSON(c, errorspkg.ErrNotImplemented.WithMessage(
			entity+": handler not yet implemented (MVP scope: products, receipt_line; см. фазу post-MVP)").WithDetails(
			errorspkg.Detail{Field: "entity", Message: entity}))
	}
}

// Below — public accessors so the router (фаза 15) может цеплять их по имени.

func (NotImplementedHandlers) ProductBarcodes() fiber.Handler { return notImplemented("product_barcodes") }
func (NotImplementedHandlers) Category() fiber.Handler        { return notImplemented("category") }
func (NotImplementedHandlers) Location() fiber.Handler        { return notImplemented("location") }
func (NotImplementedHandlers) Supplier() fiber.Handler        { return notImplemented("supplier") }
func (NotImplementedHandlers) StoreAssortment() fiber.Handler { return notImplemented("store_assortment") }
func (NotImplementedHandlers) StoreAssortmentLifecycle() fiber.Handler {
	return notImplemented("store_assortment_lifecycle_events")
}
func (NotImplementedHandlers) MasterChangeLog() fiber.Handler { return notImplemented("master_change_log") }
func (NotImplementedHandlers) SupplySpec() fiber.Handler      { return notImplemented("supply_spec") }
func (NotImplementedHandlers) Promo() fiber.Handler           { return notImplemented("promo") }
func (NotImplementedHandlers) OrderRule() fiber.Handler       { return notImplemented("order_rule") }
func (NotImplementedHandlers) SupplyPlan() fiber.Handler      { return notImplemented("supply_plan") }
func (NotImplementedHandlers) LocationStockSnapshot() fiber.Handler {
	return notImplemented("location_stock_snapshot")
}
func (NotImplementedHandlers) StockMovement() fiber.Handler { return notImplemented("stock_movement") }
func (NotImplementedHandlers) SupplierStockSnapshot() fiber.Handler {
	return notImplemented("supplier_stock_snapshot")
}
