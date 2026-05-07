// Package router регистрирует HTTP-маршруты фичи data_marts.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/handler"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// Deps — зависимости для регистрации routes data_marts.
//
// Поскольку data_marts встраивается в существующий source-adapter binary
// (не отдельный binary), JWT конфиг тот же что у data_export — передаётся
// явно для self-containment.
type Deps struct {
	JWTConfig middleware.JWTConfig
	Handler   *handler.Handler
}

// Register регистрирует /v1/marts/* поверх *fiber.App.
//
// Группа /v1/marts получает: JWT middleware → RequireAnyOf(RoleXFlowETL, RoleITRead).
// Если deps.Handler == nil — регистрация молча пропускается.
func Register(app *fiber.App, deps Deps) {
	if deps.Handler == nil {
		return
	}
	jwt := middleware.JWT(deps.JWTConfig)
	readRoles := middleware.RequireAnyOf(middleware.RoleXFlowETL, middleware.RoleITRead)

	marts := app.Group("/v1/marts", jwt, readRoles)
	marts.Get("/", deps.Handler.List)
	marts.Get("/:name", deps.Handler.GetMart)
	marts.Get("/:name/version", deps.Handler.GetVersion)
	marts.Get("/:name/schema", deps.Handler.GetSchema)
}
