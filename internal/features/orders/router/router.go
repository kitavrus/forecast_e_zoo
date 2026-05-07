// Package router регистрирует HTTP-маршруты фичи orders.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/orders/handler"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// Deps — зависимости для регистрации routes orders.
type Deps struct {
	JWTConfig middleware.JWTConfig
	Handler   *handler.Handler
}

// Register регистрирует /v1/orders/* поверх *fiber.App.
//
// Read endpoints (GET) — RoleITRead | RoleAdminCLI | RoleXFlowETL.
// Mutating endpoints (POST build/cancel/regenerate) — RoleAdminCLI.
//
// Если deps.Handler == nil — регистрация пропускается.
func Register(app *fiber.App, deps Deps) {
	if deps.Handler == nil {
		return
	}
	jwt := middleware.JWT(deps.JWTConfig)
	readRoles := middleware.RequireAnyOf(
		middleware.RoleITRead, middleware.RoleAdminCLI, middleware.RoleXFlowETL,
	)
	adminRole := middleware.RequireRole(middleware.RoleAdminCLI)

	read := app.Group("/v1/orders", jwt, readRoles)
	read.Get("/purchase-orders", deps.Handler.ListPOs)
	read.Get("/purchase-orders/:id", deps.Handler.GetPO)

	admin := app.Group("/v1/orders", jwt, adminRole)
	admin.Post("/purchase-orders/build", deps.Handler.BuildOnDemand)
	admin.Post("/purchase-orders/:id/cancel", deps.Handler.Cancel)
	admin.Post("/purchase-orders/:id/regenerate", deps.Handler.Regenerate)
}
