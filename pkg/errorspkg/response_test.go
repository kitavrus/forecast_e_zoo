package errorspkg

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteJSON_CatchAll_UnknownError_Returns500Internal — закрывает gap
// в матрице sentinel↔test (см. design-tests.md §6, замечание ревью #2).
//
// Сценарий: handler возвращает «неклассифицированную» ошибку (не *Error).
// WriteJSON должен:
//   - HTTP 500;
//   - body.code == "internal";
//   - body.supportMessage == SupportInternal (для runbook).
func TestWriteJSON_CatchAll_UnknownError_Returns500Internal(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/boom", func(c fiber.Ctx) error {
		return WriteJSON(c, errors.New("nil pointer somewhere deep"))
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: -1})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"unknown error должен мапиться на 500 (catch-all через ErrInternal.Wrap)")

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body ErrorResponseJSON
	require.NoError(t, json.Unmarshal(raw, &body))

	assert.Equal(t, "internal", body.Code,
		"catch-all path должен использовать ErrInternal.Code")
	assert.Equal(t, SupportInternal, body.SupportMessage,
		"supportMessage должен быть SA-INT-001 для runbook поиска")
	assert.NotEmpty(t, body.Message)
}

// TestWriteJSON_KnownSentinel_PreservesHTTPAndCode — sanity-check, что
// catch-all не перехватывает уже типизированные *Error.
func TestWriteJSON_KnownSentinel_PreservesHTTPAndCode(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/notfound", func(c fiber.Ctx) error {
		return WriteJSON(c, ErrLoadNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: -1})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body ErrorResponseJSON
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "load_not_found", body.Code)
}

// TestWriteJSON_NotImplemented_Returns501 — guard, что новый ErrNotImplemented
// действительно отдаёт 501 (закрывает блокер #1 ревью source-adapter).
func TestWriteJSON_NotImplemented_Returns501(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/wip", func(c fiber.Ctx) error {
		return WriteJSON(c, ErrNotImplemented.WithMessage("category: handler not yet implemented"))
	})

	req := httptest.NewRequest(http.MethodGet, "/wip", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: -1})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode,
		"NotImplemented must map to HTTP 501, not 500")

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body ErrorResponseJSON
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "not_implemented", body.Code)
	assert.Equal(t, SupportNotImplemented, body.SupportMessage)
}

// TestErrInternal_Defaults — guard на дефолтные поля catch-all sentinel-а.
func TestErrInternal_Defaults(t *testing.T) {
	t.Parallel()

	require.NotNil(t, ErrInternal)
	assert.Equal(t, "internal", ErrInternal.Code)
	assert.Equal(t, http.StatusInternalServerError, ErrInternal.HTTP)
	assert.Equal(t, SupportInternal, ErrInternal.SupportMessage)
}
