// Package router регистрирует HTTP-маршруты фичи kpi.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/handler"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// Deps — зависимости для регистрации routes kpi.
type Deps struct {
	JWTConfig middleware.JWTConfig
	Handler   *handler.Handler
}

// Register регистрирует /v1/kpi/* поверх *fiber.App.
//
// Read endpoints (GET) — RoleITRead | RoleAdminCLI | RoleXFlowETL.
// Mutating endpoints (PUT, POST refresh) — RoleAdminCLI.
//
// Если deps.Handler == nil — регистрация пропускается (slim-binary случаи).
func Register(app *fiber.App, deps Deps) {
	if deps.Handler == nil {
		return
	}
	jwt := middleware.JWT(deps.JWTConfig)
	readRoles := middleware.RequireAnyOf(
		middleware.RoleITRead, middleware.RoleAdminCLI, middleware.RoleXFlowETL,
	)
	adminRole := middleware.RequireRole(middleware.RoleAdminCLI)

	read := app.Group("/v1/kpi", jwt, readRoles)
	read.Get("/snapshots", deps.Handler.ListSnapshots)
	read.Get("/snapshots/:id", deps.Handler.GetSnapshot)
	read.Get("/calibrations", deps.Handler.ListCalibrations)

	admin := app.Group("/v1/kpi", jwt, adminRole)
	admin.Put("/calibrations/:id", deps.Handler.UpdateCalibration)
	admin.Post("/snapshots/refresh", deps.Handler.RefreshSnapshots)
}
