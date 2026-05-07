package errorspkg

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// ErrorResponseJSON — единый формат тела ответа на ошибку.
//
// Поле TraceID заполняется из middleware/request_id.
type ErrorResponseJSON struct {
	Code           string   `json:"code"`
	Message        string   `json:"message"`
	SupportMessage string   `json:"supportMessage,omitempty"`
	TraceID        string   `json:"traceId,omitempty"`
	Details        []Detail `json:"details,omitempty"`
}

// WriteJSON — отдаёт клиенту JSON-ответ на ошибку.
//
// Поведение:
//   - если err — это *Error, используем его HTTP/Code/Message/SupportMessage/Details;
//   - иначе — отдаём 500 internal с маскированием деталей (saвые поля скрываем для клиента).
//
// TraceID берётся из Locals("traceId") (выставляется middleware/request_id).
func WriteJSON(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	var de *Error
	if !errors.As(err, &de) {
		de = ErrInternal.Wrap(err)
		// Catch-all: незарегистрированная ошибка маскируется под 500 internal,
		// но обязана попасть в slog с полным err — иначе runbook по
		// supportMessage=SA-INT-001 не имеет точки входа в логи.
		slog.ErrorContext(c.Context(), "errorspkg: unmapped error → 500 internal",
			"err", err.Error(),
			"path", c.Path(),
			"method", c.Method(),
		)
	}

	traceID := ""
	if v := c.Locals("traceId"); v != nil {
		if s, ok := v.(string); ok {
			traceID = s
		}
	}

	status := de.HTTP
	if status == 0 {
		status = http.StatusInternalServerError
	}

	body := ErrorResponseJSON{
		Code:           de.Code,
		Message:        de.Message,
		SupportMessage: de.SupportMessage,
		TraceID:        traceID,
		Details:        de.Details,
	}

	return c.Status(status).JSON(body)
}
