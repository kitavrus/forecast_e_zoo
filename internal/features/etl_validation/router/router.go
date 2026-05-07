// Package router регистрирует HTTP-роуты фичи etl_validation в Fiber v3 group.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/handler"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// Middlewares — middleware-цепочка, передаваемая Register.
//
// JWTConfig: настройка JWT-валидатора (alg, secret/public key); обязательно.
// Audit: после ответа пишет в marts.audit_access (опциональный).
type Middlewares struct {
	JWTConfig middleware.JWTConfig
	Audit     fiber.Handler
}

// Register регистрирует роуты фичи в group.
//
// Маршруты (ADR-022, design-integrations.md §2.2):
//   - public: GET /healthz
//   - admin (admin-cli):       POST /admin/etl-runs, POST /admin/etl-runs/:id/retry,
//     POST /admin/marts/:name/refresh
//   - admin (admin-cli|it-read): GET /admin/etl-runs[/:id], GET /admin/reject-log
func Register(group fiber.Router, h *handler.Handler, mw Middlewares) {
	group.Get("/healthz", h.Healthz)

	g := group.Group("/admin")
	// JWT — общий guard для всей admin-группы.
	g.Use(middleware.JWT(mw.JWTConfig))
	if mw.Audit != nil {
		g.Use(mw.Audit)
	}

	// Write-роуты — только admin-cli.
	writeRole := middleware.RequireRole(middleware.RoleAdminCLI)
	g.Post("/etl-runs", h.PostEtlRun, writeRole)
	g.Post("/etl-runs/:id/retry", h.RetryEtlRun, writeRole)
	g.Post("/marts/:name/refresh", h.RefreshMart, writeRole)

	// Read-роуты — admin-cli ИЛИ it-read.
	readRole := middleware.RequireAnyOf(middleware.RoleAdminCLI, middleware.RoleITRead)
	g.Get("/etl-runs/:id", h.GetEtlRun, readRole)
	g.Get("/etl-runs", h.ListEtlRuns, readRole)
	g.Get("/reject-log", h.ListRejectLog, readRole)
}
