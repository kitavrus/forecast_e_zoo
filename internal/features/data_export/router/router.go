// Package router регистрирует HTTP-маршруты фичи data_export.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// Deps — все зависимости, нужные для регистрации маршрутов.
type Deps struct {
	JWTConfig          middleware.JWTConfig
	HealthzHandler     *handler.HealthzHandler
	SnapshotsHandler   *handler.SnapshotsHandler
	ProductsHandler    *handler.ProductsHandler
	ReceiptLineHandler *handler.ReceiptLineHandler
	ExportsHandler     *handler.ExportsHandler
	AdminLoadsHandler  *handler.AdminLoadsHandler
	AuditMiddleware    fiber.Handler

	// NotImplemented — placeholder для прочих 14 entity (Phase 13).
	NotImplemented handler.NotImplementedHandlers
}

// Register регистрирует все маршруты source-adapter.
//
//nolint:funlen // регистрация маршрутов — длинный список по природе
func Register(app *fiber.App, deps Deps) {
	// /healthz и /metrics — без JWT.
	if deps.HealthzHandler != nil {
		app.Get("/healthz", deps.HealthzHandler.Get)
	}

	jwt := middleware.JWT(deps.JWTConfig)

	// /v1/* — JWT обязателен.
	v1 := app.Group("/v1", jwt)

	if deps.SnapshotsHandler != nil {
		v1.Get("/snapshots/current", deps.SnapshotsHandler.GetCurrent,
			middleware.RequireAnyOf(middleware.RoleXFlowETL, middleware.RoleITRead))
	}

	// Master entities — RoleXFlowETL OR RoleITRead.
	readRoles := middleware.RequireAnyOf(middleware.RoleXFlowETL, middleware.RoleITRead)

	if deps.ProductsHandler != nil {
		v1.Get("/products", deps.ProductsHandler.Get, readRoles)
	}
	v1.Get("/product_barcodes", deps.NotImplemented.ProductBarcodes(), readRoles)
	v1.Get("/category", deps.NotImplemented.Category(), readRoles)
	v1.Get("/location", deps.NotImplemented.Location(), readRoles)
	v1.Get("/supplier", deps.NotImplemented.Supplier(), readRoles)
	v1.Get("/store_assortment", deps.NotImplemented.StoreAssortment(), readRoles)
	v1.Get("/store_assortment/lifecycle_events", deps.NotImplemented.StoreAssortmentLifecycle(), readRoles)
	v1.Get("/master_change_log", deps.NotImplemented.MasterChangeLog(), readRoles)
	v1.Get("/supply_spec", deps.NotImplemented.SupplySpec(), readRoles)
	v1.Get("/promo", deps.NotImplemented.Promo(), readRoles)
	v1.Get("/order_rule", deps.NotImplemented.OrderRule(), readRoles)
	v1.Get("/supply_plan", deps.NotImplemented.SupplyPlan(), readRoles)

	// Facts.
	if deps.ReceiptLineHandler != nil {
		v1.Get("/receipt_line", deps.ReceiptLineHandler.Get, readRoles)
	}
	v1.Get("/location_stock_snapshot", deps.NotImplemented.LocationStockSnapshot(), readRoles)
	v1.Get("/stock_movement", deps.NotImplemented.StockMovement(), readRoles)
	v1.Get("/supplier_stock_snapshot", deps.NotImplemented.SupplierStockSnapshot(), readRoles)

	// Exports.
	if deps.ExportsHandler != nil {
		v1.Post("/exports", deps.ExportsHandler.Post, middleware.RequireXFlowETL())
		v1.Get("/exports/:id", deps.ExportsHandler.Get, readRoles)
	}

	// /admin/* — JWT + admin role + audit middleware.
	adminMW := []fiber.Handler{jwt, middleware.RequireAdmin()}
	if deps.AuditMiddleware != nil {
		adminMW = append(adminMW, deps.AuditMiddleware)
	}
	admin := app.Group("/admin", adminMW...)

	if deps.AdminLoadsHandler != nil {
		admin.Post("/loads", deps.AdminLoadsHandler.PostLoads)
		admin.Post("/loads/:id/retry", deps.AdminLoadsHandler.PostLoadsRetry)
		admin.Get("/loads/:id", deps.AdminLoadsHandler.GetLoadByID)
		admin.Get("/reject-log", deps.AdminLoadsHandler.GetRejectLog)
	}
}
