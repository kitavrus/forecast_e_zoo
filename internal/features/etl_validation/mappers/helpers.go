// Package mappers содержит mapping ошибок service → HTTP response.
package mappers

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// MapServiceError — общий dispatcher: оборачивает любую ошибку через errorspkg.WriteJSON.
func MapServiceError(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	return errorspkg.WriteJSON(c, err)
}

// MapTriggerRunError — POST /admin/etl-runs.
//
// Если ErrEtlRunAlreadyRunning, возвращаем 409 + supportMessage=EV-001.
// Дополнительные поля (current_run_id) добавит handler сам, через Locals.
func MapTriggerRunError(c fiber.Ctx, err error) error {
	if errors.Is(err, errorspkg.ErrEtlRunAlreadyRunning) {
		return errorspkg.WriteJSON(c, errorspkg.ErrEtlRunAlreadyRunning)
	}
	return MapServiceError(c, err)
}

// MapRetryError — POST /admin/etl-runs/:id/retry.
func MapRetryError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, errorspkg.ErrEtlRunNotFound):
		return errorspkg.WriteJSON(c, errorspkg.ErrEtlRunNotFound)
	case errors.Is(err, errorspkg.ErrCannotRetryEtl):
		return errorspkg.WriteJSON(c, errorspkg.ErrCannotRetryEtl)
	}
	return MapServiceError(c, err)
}

// MapMartRefreshError — POST /admin/marts/:name/refresh.
func MapMartRefreshError(c fiber.Ctx, err error) error {
	if errors.Is(err, errorspkg.ErrMartRefreshNotSupported) {
		return errorspkg.WriteJSON(c, errorspkg.ErrMartRefreshNotSupported)
	}
	if errors.Is(err, errorspkg.ErrSourceUnavailable) {
		return errorspkg.WriteJSON(c, errorspkg.ErrSourceUnavailable)
	}
	return MapServiceError(c, err)
}
