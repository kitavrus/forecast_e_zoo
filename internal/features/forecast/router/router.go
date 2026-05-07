// Package router регистрирует HTTP-маршруты фичи forecast.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/handler"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// Deps — зависимости для регистрации routes forecast.
type Deps struct {
	JWTConfig middleware.JWTConfig
	Handler   *handler.Handler
}

// Register регистрирует /v1/forecast/* и /v1/replenishment/* поверх *fiber.App.
//
// Read endpoints (GET) — RoleITRead | RoleAdminCLI | RoleXFlowETL.
// Mutating endpoints (POST refresh, POST approve) — RoleAdminCLI.
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

	readForecast := app.Group("/v1/forecast", jwt, readRoles)
	readForecast.Get("/runs", deps.Handler.ListRuns)
	readForecast.Get("/runs/:id", deps.Handler.GetRun)

	adminForecast := app.Group("/v1/forecast", jwt, adminRole)
	adminForecast.Post("/runs/refresh", deps.Handler.RefreshRun)

	readPlans := app.Group("/v1/replenishment", jwt, readRoles)
	readPlans.Get("/plans", deps.Handler.ListPlans)
	readPlans.Get("/plans/:id", deps.Handler.GetPlan)

	adminPlans := app.Group("/v1/replenishment", jwt, adminRole)
	adminPlans.Post("/plans/:id/approve", deps.Handler.ApprovePlan)
}
