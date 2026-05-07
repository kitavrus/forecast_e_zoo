// Package router содержит helper-конструкторы middleware-цепочек для admin-эндпоинтов.
package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// AdminMiddlewares собирает цепочку middleware для /admin/*-эндпоинтов
// согласно ADR-022:
//   - JWT (HS256/RS256) — обязателен;
//   - RequireRole("admin-cli") — для write-эндпоинтов (POST/retry/refresh, reject-log).
//
// Список ролей читается из интерфейса вызывающего: для read-only (list/get etl-runs)
// caller передаёт несколько ролей через RequireAnyOf. Роуты, требующие только admin-cli,
// получают единственное значение.
func AdminMiddlewares(jwtCfg middleware.JWTConfig, roles ...string) []fiber.Handler {
	if len(roles) == 0 {
		roles = []string{middleware.RoleAdminCLI}
	}
	return []fiber.Handler{
		middleware.JWT(jwtCfg),
		middleware.RequireRole(roles...),
	}
}
