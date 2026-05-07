package middleware_test

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/test/helpers"
)

// roleTestApp поднимает Fiber c JWT() + переданным role-guard'ом.
func roleTestApp(t *testing.T, guard fiber.Handler) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(middleware.JWT(middleware.JWTConfig{
		Alg:    middleware.AlgHS256,
		Secret: testSecret,
	}))
	app.Use(guard)
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	return app
}

func roleDo(t *testing.T, app *fiber.App, token string) int {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, "/x", nil)
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func TestRequireRole_MissingClaims_Returns401AtJWT(t *testing.T) {
	t.Parallel()
	// Без токена JWT-middleware вернёт 401 раньше, чем role-guard 403.
	// Это намеренно: без аутентификации мы не различаем "кто", чтобы не делать enumeration.
	app := roleTestApp(t, middleware.RequireXFlowETL())
	code := roleDo(t, app, "")
	assert.Equal(t, fiber.StatusUnauthorized, code)
}

func TestRequireRole_WrongIssuer_Forbidden(t *testing.T) {
	t.Parallel()
	app := roleTestApp(t, middleware.RequireAdmin())
	tok := helpers.SignTestJWT(t, testSecret, middleware.RoleXFlowETL, "tester", time.Hour)
	code := roleDo(t, app, tok)
	assert.Equal(t, fiber.StatusForbidden, code)
}

func TestRequireRole_AllowedRole_Passes(t *testing.T) {
	t.Parallel()
	app := roleTestApp(t, middleware.RequireAnyOf(middleware.RoleXFlowETL, middleware.RoleITRead))
	tok := helpers.SignTestJWT(t, testSecret, middleware.RoleITRead, "tester", time.Hour)
	code := roleDo(t, app, tok)
	assert.Equal(t, fiber.StatusOK, code)
}

func TestRequireXFlowETL_AllowsXFlow(t *testing.T) {
	t.Parallel()
	app := roleTestApp(t, middleware.RequireXFlowETL())
	tok := helpers.SignTestJWT(t, testSecret, middleware.RoleXFlowETL, "etl", time.Hour)
	code := roleDo(t, app, tok)
	assert.Equal(t, fiber.StatusOK, code)
}

func TestRequireAdmin_BlocksXFlow(t *testing.T) {
	t.Parallel()
	app := roleTestApp(t, middleware.RequireAdmin())
	tok := helpers.SignTestJWT(t, testSecret, middleware.RoleXFlowETL, "etl", time.Hour)
	code := roleDo(t, app, tok)
	assert.Equal(t, fiber.StatusForbidden, code)
}
