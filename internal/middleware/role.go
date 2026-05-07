package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Стандартные роли проекта (issuer в JWT).
const (
	RoleXFlowETL = "x-flow-etl"
	RoleAdminCLI = "admin-cli"
	RoleITRead   = "it-read"
)

// RequireRole возвращает middleware, который пропускает только запросы
// с issuer'ом из разрешённого списка. Должен идти ПОСЛЕ JWT().
//
// На fail (нет claims или issuer не в списке) — 403 ErrAuthForbidden.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c fiber.Ctx) error {
		claims, ok := ClaimsFromCtx(c)
		if !ok || claims == nil {
			return errorspkg.WriteJSON(c, errorspkg.ErrAuthForbidden)
		}
		if _, ok := allowed[claims.Issuer]; !ok {
			return errorspkg.WriteJSON(c, errorspkg.ErrAuthForbidden)
		}
		return c.Next()
	}
}

// RequireXFlowETL — guard для эндпоинтов чтения, доступных X-Flow-ETL.
func RequireXFlowETL() fiber.Handler {
	return RequireRole(RoleXFlowETL)
}

// RequireAdmin — guard для /admin/* (внутренний CLI/runbook).
func RequireAdmin() fiber.Handler {
	return RequireRole(RoleAdminCLI)
}

// RequireITRead — guard для read-only IT-консоли.
func RequireITRead() fiber.Handler {
	return RequireRole(RoleITRead)
}

// RequireAnyOf — несколько ролей сразу, удобно для эндпоинтов /v1/*,
// доступных и X-Flow-ETL, и IT-Read.
func RequireAnyOf(roles ...string) fiber.Handler {
	return RequireRole(roles...)
}
