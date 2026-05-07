package handler_test

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/exports_storage"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

type fakeExportsSvc struct {
	mu       sync.Mutex
	startReq dto.PostExportRequest
	startID  uuid.UUID
	getMeta  exports_storage.Meta
	getPath  string
	getErr   error
}

func (f *fakeExportsSvc) Start(_ context.Context, req dto.PostExportRequest, _ string) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startReq = req
	if f.startID == uuid.Nil {
		f.startID = uuid.New()
	}
	return f.startID, nil
}
func (f *fakeExportsSvc) Get(_ context.Context, id uuid.UUID) (string, exports_storage.Meta, error) {
	if f.getErr != nil {
		return "", exports_storage.Meta{}, f.getErr
	}
	if f.getMeta.Entity == "" {
		f.getMeta = exports_storage.Meta{Entity: "products", Format: "ndjson", Status: "ready", CreatedAt: time.Now()}
	}
	if f.getPath == "" {
		f.getPath = "/tmp/" + id.String() + ".ndjson"
	}
	return f.getPath, f.getMeta, nil
}

func TestPostExport_BadFormat_400_ErrInvalidExportFormat(t *testing.T) {
	t.Parallel()
	svc := &fakeExportsSvc{}
	app := fiber.New()
	h := handler.NewExportsHandler(svc)
	app.Post("/v1/exports", h.Post)
	body := `{"entity":"products","format":"csv","snapshot_id":"00000000-0000-0000-0000-000000000001"}`
	req := httptest.NewRequest("POST", "/v1/exports", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

func TestPostExport_HappyPath_Returns202(t *testing.T) {
	t.Parallel()
	svc := &fakeExportsSvc{}
	app := fiber.New()
	h := handler.NewExportsHandler(svc)
	app.Post("/v1/exports", h.Post)
	body := `{"entity":"products","format":"ndjson","snapshot_id":"00000000-0000-0000-0000-000000000001"}`
	req := httptest.NewRequest("POST", "/v1/exports", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 202, resp.StatusCode)
	out, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(out), `"status":"pending"`)
}

func TestGetExport_Pending_Returns202(t *testing.T) {
	t.Parallel()
	svc := &fakeExportsSvc{
		getMeta: exports_storage.Meta{Entity: "products", Format: "ndjson", Status: "pending", CreatedAt: time.Now()},
	}
	app := fiber.New()
	h := handler.NewExportsHandler(svc)
	app.Get("/v1/exports/:id", h.Get)
	req := httptest.NewRequest("GET", "/v1/exports/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 202, resp.StatusCode)
}

func TestGetExport_Ready_Returns200(t *testing.T) {
	t.Parallel()
	svc := &fakeExportsSvc{
		getMeta: exports_storage.Meta{Entity: "products", Format: "ndjson", Status: "ready", SizeBytes: 42, CreatedAt: time.Now()},
		getPath: "/tmp/x.ndjson",
	}
	app := fiber.New()
	h := handler.NewExportsHandler(svc)
	app.Get("/v1/exports/:id", h.Get)
	req := httptest.NewRequest("GET", "/v1/exports/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `"status":"ready"`)
}

func TestGetExport_NotFound_404(t *testing.T) {
	t.Parallel()
	svc := &fakeExportsSvc{getErr: errorspkg.ErrExportNotFound}
	app := fiber.New()
	h := handler.NewExportsHandler(svc)
	app.Get("/v1/exports/:id", h.Get)
	req := httptest.NewRequest("GET", "/v1/exports/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}
