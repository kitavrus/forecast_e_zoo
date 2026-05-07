// Package middleware содержит общие HTTP-middleware: JWT, role gating, request_id.
package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// LocalsTraceID — ключ для c.Locals(), под которым хранится trace id запроса.
//
// Используется ErrorResponseJSON.TraceID и доменными слогами для связки логов.
const LocalsTraceID = "trace_id"

// HeaderRequestID — имя HTTP-заголовка для request id.
const HeaderRequestID = "X-Request-Id"

// RequestID middleware:
//   - если клиент прислал X-Request-Id — переиспользуем как trace_id;
//   - иначе генерируем uuid v7 (если поддерживается) или v4.
//
// trace_id кладём в c.Locals(LocalsTraceID) и в response header.
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		incoming := c.Get(HeaderRequestID)

		traceID := incoming
		if traceID == "" {
			// uuid v7 даёт временную сортируемость + рандом → удобно для логов.
			if id, err := uuid.NewV7(); err == nil {
				traceID = id.String()
			} else {
				traceID = uuid.NewString()
			}
		}

		c.Locals(LocalsTraceID, traceID)
		c.Set(HeaderRequestID, traceID)

		return c.Next()
	}
}
