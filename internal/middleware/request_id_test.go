package middleware_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/middleware"
)

func TestRequestID_Generates(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Get("/x", func(c fiber.Ctx) error {
		v := c.Locals(middleware.LocalsTraceID)
		s, _ := v.(string)
		return c.SendString(s)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/x", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	id := resp.Header.Get(middleware.HeaderRequestID)
	assert.NotEmpty(t, id)
	assert.Greater(t, len(id), 8) // uuid строчка длиннее 8 символов
}

func TestRequestID_Reuses(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/x", nil)
	req.Header.Set(middleware.HeaderRequestID, "trace-from-client")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, "trace-from-client", resp.Header.Get(middleware.HeaderRequestID))
}
