package observability_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/observability"
)

func TestHTTPMetricsMiddleware_RecordsLatency(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(observability.HTTPMetricsMiddleware())
	app.Get("/foo", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest("GET", "/foo", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	_, _ = io.Copy(io.Discard, resp.Body)
}

func TestAccessLogMiddleware_WritesEntry(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	app := fiber.New()
	app.Use(observability.AccessLogMiddleware(logger))
	app.Get("/bar", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest("GET", "/bar", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	_, _ = io.Copy(io.Discard, resp.Body)

	require.True(t, strings.Contains(buf.String(), `"msg":"http.access"`))
	require.True(t, strings.Contains(buf.String(), `"path":"/bar"`))
}
