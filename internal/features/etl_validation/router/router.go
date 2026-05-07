// Package router регистрирует HTTP-роуты фичи etl_validation в Fiber v3 group.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/handler"
)

// Middlewares — middleware-цепочка, передаваемая Register.
//
// Admin: для /admin/* (например, X-Admin-Secret или JWT с ролью admin-cli).
// Audit: после ответа пишет в marts.audit_access (опциональный).
type Middlewares struct {
	Admin fiber.Handler
	Audit fiber.Handler
}

// Register регистрирует роуты фичи в group.
//
// Маршруты:
//   - public: GET /healthz
//   - admin:  POST/GET /admin/etl-runs[/:id[/retry]], POST /admin/marts/:name/refresh,
//             GET /admin/reject-log
func Register(group fiber.Router, h *handler.Handler, mw Middlewares) {
	group.Get("/healthz", h.Healthz)

	g := group.Group("/admin")
	if mw.Admin != nil {
		g.Use(mw.Admin)
	}
	if mw.Audit != nil {
		g.Use(mw.Audit)
	}
	g.Post("/etl-runs", h.PostEtlRun)
	g.Post("/etl-runs/:id/retry", h.RetryEtlRun)
	g.Get("/etl-runs/:id", h.GetEtlRun)
	g.Get("/etl-runs", h.ListEtlRuns)
	g.Post("/marts/:name/refresh", h.RefreshMart)
	g.Get("/reject-log", h.ListRejectLog)
}
