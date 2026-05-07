package observability

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// HTTPMetricsMiddleware — измеряет latency и status_code, инкрементит метрики.
func HTTPMetricsMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		dur := time.Since(start).Seconds()
		method := c.Method()
		path := c.Route().Path
		if path == "" {
			path = c.Path()
		}
		status := strconv.Itoa(c.Response().StatusCode())
		HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(dur)
		return err
	}
}

// AccessLogMiddleware — пишет одну строку access-лога после c.Next().
func AccessLogMiddleware(logger *slog.Logger) fiber.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		traceID, _ := c.Locals(middleware.LocalsTraceID).(string)
		role, sub := "", ""
		if claims, ok := middleware.ClaimsFromCtx(c); ok && claims != nil {
			role = claims.Issuer
			sub = claims.Subject
		}
		logger.LogAttrs(c.Context(), slog.LevelInfo, "http.access",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("latency", latency),
			slog.String("trace_id", traceID),
			slog.String("actor_role", role),
			slog.String("actor_sub", sub),
		)
		return err
	}
}
