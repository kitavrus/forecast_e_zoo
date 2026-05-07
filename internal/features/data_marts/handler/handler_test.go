package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/handler"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// stubSvc — минимальный mock MartsService для unit-тестов handler.
type stubSvc struct {
	listResp    []models.MartInfo
	listErr     error
	readRows    []models.MartRow
	readNext    string
	readVersion models.MartVersion
	readErr     error
	verResp     models.MartVersion
	verErr      error
	schemaResp  models.MartSchema
	schemaErr   error
}

func (s *stubSvc) List(_ context.Context) ([]models.MartInfo, error) {
	return s.listResp, s.listErr
}
func (s *stubSvc) Read(_ context.Context, _ string, _ string, _ int) (
	[]models.MartRow, string, models.MartVersion, error,
) {
	return s.readRows, s.readNext, s.readVersion, s.readErr
}
func (s *stubSvc) GetVersion(_ context.Context, _ string) (models.MartVersion, error) {
	return s.verResp, s.verErr
}
func (s *stubSvc) GetSchema(_ context.Context, _ string) (models.MartSchema, error) {
	return s.schemaResp, s.schemaErr
}

func newApp(h *handler.Handler) *fiber.App {
	app := fiber.New()
	app.Get("/v1/marts", h.List)
	app.Get("/v1/marts/:name", h.GetMart)
	app.Get("/v1/marts/:name/version", h.GetVersion)
	app.Get("/v1/marts/:name/schema", h.GetSchema)
	return app
}

// --- List ---

func TestList_HappyPath(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := time.Now().UTC()
	svc := &stubSvc{listResp: []models.MartInfo{
		{Name: constants.MartKpiDaily, EtlRunID: id, CommittedAt: now},
		{Name: constants.MartMasterCurrent}, // не наполнен
	}}
	app := newApp(handler.NewHandler(svc))

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		Marts []struct {
			Name      string `json:"name"`
			Populated bool   `json:"populated"`
		} `json:"marts"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Len(t, got.Marts, 2)
	assert.True(t, got.Marts[0].Populated)
	assert.False(t, got.Marts[1].Populated)
}

// --- GetVersion ---

func TestGetVersion_HappyPath(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := &stubSvc{verResp: models.MartVersion{
		Name: constants.MartKpiDaily, EtlRunID: id, CommittedAt: time.Now().UTC(),
	}}
	app := newApp(handler.NewHandler(svc))

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartKpiDaily+"/version", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, id.String(), resp.Header.Get("X-Etl-Run-Id"))
}

func TestGetVersion_UnknownMart_404(t *testing.T) {
	t.Parallel()
	app := newApp(handler.NewHandler(&stubSvc{}))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/unknown_mart/version", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestGetVersion_NotPopulated_503(t *testing.T) {
	t.Parallel()
	svc := &stubSvc{verErr: errorspkg.ErrServiceUnavailable.WithMessage("no committed etl run yet")}
	app := newApp(handler.NewHandler(svc))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartKpiDaily+"/version", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "60", resp.Header.Get("Retry-After"))
}

// --- GetSchema ---

func TestGetSchema_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &stubSvc{schemaResp: models.MartSchema{
		Name: constants.MartKpiDaily,
		Fields: []models.MartField{
			{Name: "as_of_date", Type: "date"},
			{Name: "kpi_value", Type: "numeric"},
		},
	}}
	app := newApp(handler.NewHandler(svc))

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartKpiDaily+"/schema", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "as_of_date")
}

func TestGetSchema_UnknownMart_404(t *testing.T) {
	t.Parallel()
	app := newApp(handler.NewHandler(&stubSvc{}))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/unknown_mart/schema", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// --- GetMart (NDJSON streaming) ---

func TestGetMart_HappyPath_NDJSON(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := &stubSvc{
		readRows: []models.MartRow{
			{"product_id": "P1", "location_id": "L1"},
			{"product_id": "P2", "location_id": "L1"},
		},
		readNext:    "next-token",
		readVersion: models.MartVersion{Name: constants.MartDemandHistory, EtlRunID: id},
	}
	app := newApp(handler.NewHandler(svc))

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartDemandHistory+"?limit=2", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get(fiber.HeaderContentType), "application/x-ndjson")
	assert.Equal(t, id.String(), resp.Header.Get("X-Etl-Run-Id"))
	assert.Equal(t, "next-token", resp.Header.Get("X-Next-Cursor"))

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	assert.Len(t, lines, 2)
}

func TestGetMart_UnknownMart_404(t *testing.T) {
	t.Parallel()
	app := newApp(handler.NewHandler(&stubSvc{}))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/unknown_mart", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestGetMart_BadLimit_400(t *testing.T) {
	t.Parallel()
	app := newApp(handler.NewHandler(&stubSvc{}))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartKpiDaily+"?limit=abc", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestGetMart_LimitTooLarge_400(t *testing.T) {
	t.Parallel()
	app := newApp(handler.NewHandler(&stubSvc{}))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartKpiDaily+"?limit=99999", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestGetMart_BadCursor_400(t *testing.T) {
	t.Parallel()
	svc := &stubSvc{readErr: errorspkg.ErrBadRequest.WithMessage("invalid cursor")}
	app := newApp(handler.NewHandler(svc))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartKpiDaily+"?cursor=@@bad@@", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestGetMart_LastPage_NoCursorHeader(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := &stubSvc{
		readRows:    []models.MartRow{{"x": "y"}},
		readNext:    "", // last page
		readVersion: models.MartVersion{EtlRunID: id},
	}
	app := newApp(handler.NewHandler(svc))
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/marts/"+constants.MartKpiDaily, nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Empty(t, resp.Header.Get("X-Next-Cursor"))
}
