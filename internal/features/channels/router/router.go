// Package router регистрирует HTTP-маршруты фичи channels (Module 7).
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/channels/handler"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// Deps — зависимости для регистрации routes channels.
type Deps struct {
	JWTConfig middleware.JWTConfig
	Handler   *handler.Handler
}

// Register регистрирует /v1/channels/* поверх *fiber.App.
//
// Read endpoints (GET) — RoleITRead | RoleAdminCLI | RoleXFlowETL.
// Mutating endpoints (POST/PUT) — RoleAdminCLI.
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

	read := app.Group("/v1/channels", jwt, readRoles)
	read.Get("/send-attempts", deps.Handler.ListSendAttempts)
	read.Get("/send-attempts/:id", deps.Handler.GetSendAttempt)
	read.Get("/configs", deps.Handler.ListConfigs)

	admin := app.Group("/v1/channels", jwt, adminRole)
	admin.Post("/send", deps.Handler.TriggerSend)
	admin.Post("/send/:po_id/retry", deps.Handler.Retry)
	admin.Put("/configs/:supplier_id", deps.Handler.UpsertConfig)
}
