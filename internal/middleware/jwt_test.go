package middleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/test/helpers"
)

const testSecret = "unit-test-secret-do-not-use-in-prod"

// newApp поднимает мини-Fiber с JWT-middleware и одним эндпоинтом /protected.
func newApp(t *testing.T, cfg middleware.JWTConfig) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(middleware.JWT(cfg))
	app.Get("/protected", func(c fiber.Ctx) error {
		// Проверим, что claims доступны.
		if cl, ok := middleware.ClaimsFromCtx(c); ok && cl != nil {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"iss": cl.Issuer,
				"sub": cl.Subject,
			})
		}
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func doRequest(t *testing.T, app *fiber.App, authHeader string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set(fiber.HeaderAuthorization, authHeader)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestJWT_NoHeader_Returns401(t *testing.T) {
	t.Parallel()
	app := newApp(t, middleware.JWTConfig{Alg: middleware.AlgHS256, Secret: testSecret})
	code, body := doRequest(t, app, "")
	assert.Equal(t, fiber.StatusUnauthorized, code)
	assert.Contains(t, body, "auth_invalid_token")
}

func TestJWT_MalformedToken_Returns401(t *testing.T) {
	t.Parallel()
	app := newApp(t, middleware.JWTConfig{Alg: middleware.AlgHS256, Secret: testSecret})
	code, body := doRequest(t, app, "Bearer not-a-jwt")
	assert.Equal(t, fiber.StatusUnauthorized, code)
	assert.Contains(t, body, "auth_invalid_token")
}

func TestJWT_BadSchema_Returns401(t *testing.T) {
	t.Parallel()
	app := newApp(t, middleware.JWTConfig{Alg: middleware.AlgHS256, Secret: testSecret})
	code, _ := doRequest(t, app, "Basic abc")
	assert.Equal(t, fiber.StatusUnauthorized, code)
}

func TestJWT_ExpiredToken_Returns401(t *testing.T) {
	t.Parallel()
	app := newApp(t, middleware.JWTConfig{Alg: middleware.AlgHS256, Secret: testSecret})
	tok := helpers.SignTestJWT(t, testSecret, middleware.RoleXFlowETL, "tester", -time.Minute)
	code, body := doRequest(t, app, "Bearer "+tok)
	assert.Equal(t, fiber.StatusUnauthorized, code)
	assert.Contains(t, body, "auth_invalid_token")
}

func TestJWT_HS256_ValidToken_Passes(t *testing.T) {
	t.Parallel()
	app := newApp(t, middleware.JWTConfig{Alg: middleware.AlgHS256, Secret: testSecret})
	tok := helpers.SignTestJWT(t, testSecret, middleware.RoleXFlowETL, "tester", time.Hour)
	code, body := doRequest(t, app, "Bearer "+tok)
	assert.Equal(t, fiber.StatusOK, code)
	assert.Contains(t, body, middleware.RoleXFlowETL)
}

func TestJWT_HS256_WrongSecret_Returns401(t *testing.T) {
	t.Parallel()
	app := newApp(t, middleware.JWTConfig{Alg: middleware.AlgHS256, Secret: testSecret})
	tok := helpers.SignTestJWT(t, "another-secret", middleware.RoleXFlowETL, "tester", time.Hour)
	code, _ := doRequest(t, app, "Bearer "+tok)
	assert.Equal(t, fiber.StatusUnauthorized, code)
}

func TestJWT_RS256_ValidToken_Passes(t *testing.T) {
	t.Parallel()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)

	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	dir := t.TempDir()
	pubPath := filepath.Join(dir, "pub.pem")
	require.NoError(t, os.WriteFile(pubPath, pubPEM, 0o600))

	app := newApp(t, middleware.JWTConfig{
		Alg:           middleware.AlgRS256,
		PublicKeyPath: pubPath,
	})

	tok := helpers.SignTestJWTRSA(t, priv, middleware.RoleAdminCLI, "rsa-tester", time.Hour)
	code, body := doRequest(t, app, "Bearer "+tok)
	assert.Equal(t, fiber.StatusOK, code)
	assert.Contains(t, body, middleware.RoleAdminCLI)
}

func TestJWT_AlgConfusion_HS256TokenAgainstRS256_Returns401(t *testing.T) {
	t.Parallel()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	dir := t.TempDir()
	pubPath := filepath.Join(dir, "pub.pem")
	require.NoError(t, os.WriteFile(pubPath, pubPEM, 0o600))

	app := newApp(t, middleware.JWTConfig{Alg: middleware.AlgRS256, PublicKeyPath: pubPath})

	// Подсовываем HS256-токен, подписанный публичным ключом — классическая alg confusion.
	tok := helpers.SignTestJWT(t, "anything", middleware.RoleAdminCLI, "evil", time.Hour)
	code, _ := doRequest(t, app, "Bearer "+tok)
	assert.Equal(t, fiber.StatusUnauthorized, code)
}

func TestJWT_ClaimsInLocals(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(middleware.JWT(middleware.JWTConfig{Alg: middleware.AlgHS256, Secret: testSecret}))
	app.Get("/echo", func(c fiber.Ctx) error {
		cl, ok := middleware.ClaimsFromCtx(c)
		require.True(t, ok)
		require.NotNil(t, cl)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"sub": cl.Subject})
	})

	tok := helpers.SignTestJWT(t, testSecret, middleware.RoleITRead, "user-42", time.Hour)
	req := httptest.NewRequest(fiber.MethodGet, "/echo", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+tok)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "user-42")
}
