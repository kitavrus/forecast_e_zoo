// Package mappers — mapping service-ошибок в HTTP-ответы фичи kpi.
package mappers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// MapServiceError — единая точка преобразования error → HTTP JSON.
//
// Известные sentinel'ы (ErrKpiSnapshotNotFound, ErrKpiCalibrationNotFound,
// ErrInvalidKpiName, общие *errorspkg.Error) уже несут HTTP/code/supportMessage.
// Любая нераспознанная ошибка → 500.
func MapServiceError(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	return errorspkg.WriteJSON(c, err)
}
