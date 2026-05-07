// Package mappers содержит mapping service-ошибок в HTTP-ответы.
package mappers

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// MapServiceError — единая точка преобразования ошибки в HTTP JSON-ответ.
//
// Известные sentinel'ы (ErrBadRequest/ErrNotFound/ErrServiceUnavailable/...)
// уже несут HTTP-код через WriteJSON. Любая другая → 500.
//
// Для 503 (mart not yet populated) добавляем header Retry-After.
func MapServiceError(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errorspkg.ErrServiceUnavailable) {
		c.Set("Retry-After", "60")
	}
	return errorspkg.WriteJSON(c, err)
}
