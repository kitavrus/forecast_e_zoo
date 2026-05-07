// Package mappers — mapping service-ошибок в HTTP-ответы фичи forecast.
package mappers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// MapServiceError — единая точка преобразования error → HTTP JSON.
func MapServiceError(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	return errorspkg.WriteJSON(c, err)
}
