package router

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// AdminSecretMiddleware — простой guard через заголовок X-Admin-Secret
// (соответствует общему паттерну проекта).
//
// Если secret не сконфигурирован (пустая строка) — middleware no-op
// (используется в dev/test mode).
func AdminSecretMiddleware(secret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if secret == "" {
			return c.Next()
		}
		got := c.Get("X-Admin-Secret")
		if got != secret {
			return errorspkg.WriteJSON(c, errorspkg.ErrForbidden)
		}
		return c.Next()
	}
}
