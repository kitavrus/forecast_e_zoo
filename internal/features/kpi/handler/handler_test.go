package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/handler"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// fakeSvc — in-memory заглушка KpiService для handler-юнит-тестов.
type fakeSvc struct {
	listSnapshots func(ctx context.Context, f models.SnapshotFilter) ([]models.KpiSnapshot, string, error)
	getSnapshot   func(ctx context.Context, id uuid.UUID) (models.KpiSnapshot, error)
	listCalibs    func(ctx context.Context, f models.CalibrationFilter) ([]models.KpiCalibration, error)
	updateCalib   func(ctx context.Context, id uuid.UUID, params json.RawMessage) (models.KpiCalibration, error)
	trigger       func(ctx context.Context, asOf time.Time, names []string) (uuid.UUID, bool, error)
}

func (f *fakeSvc) ListSnapshots(ctx context.Context, fil models.SnapshotFilter) ([]models.KpiSnapshot, string, error) {
	return f.listSnapshots(ctx, fil)
}
func (f *fakeSvc) GetSnapshot(ctx context.Context, id uuid.UUID) (models.KpiSnapshot, error) {
	return f.getSnapshot(ctx, id)
}
func (f *fakeSvc) ListCalibrations(ctx context.Context, fil models.CalibrationFilter) ([]models.KpiCalibration, error) {
	return f.listCalibs(ctx, fil)
}
func (f *fakeSvc) UpdateCalibration(ctx context.Context, id uuid.UUID, p json.RawMessage) (models.KpiCalibration, error) {
	return f.updateCalib(ctx, id, p)
}
func (f *fakeSvc) TriggerRefresh(ctx context.Context, asOf time.Time, names []string) (uuid.UUID, bool, error) {
	return f.trigger(ctx, asOf, names)
}

func newApp(svc handler.KpiService) *fiber.App {
	h := handler.NewHandler(svc)
	app := fiber.New()
	app.Get("/snapshots", h.ListSnapshots)
	app.Get("/snapshots/:id", h.GetSnapshot)
	app.Get("/calibrations", h.ListCalibrations)
	app.Put("/calibrations/:id", h.UpdateCalibration)
	app.Post("/snapshots/refresh", h.RefreshSnapshots)
	return app
}

func readBody(t *testing.T, r io.ReadCloser) []byte {
	t.Helper()
	defer func() { _ = r.Close() }()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return b
}

func TestListSnapshots_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &fakeSvc{
		listSnapshots: func(_ context.Context, _ models.SnapshotFilter) ([]models.KpiSnapshot, string, error) {
			return []models.KpiSnapshot{
				{ID: uuid.New(), AsOfDate: time.Now(), KpiName: "osa", ScopeType: "global", Value: 0.92},
			}, "", nil
		},
	}
	app := newApp(svc)
	req := httptest.NewRequest("GET", "/snapshots", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Contains(t, string(readBody(t, resp.Body)), `"kpi_name":"osa"`)
}

func TestListSnapshots_InvalidKpiName(t *testing.T) {
	t.Parallel()
	app := newApp(&fakeSvc{})
	req := httptest.NewRequest("GET", "/snapshots?kpi_name=bogus", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

func TestGetSnapshot_NotFound(t *testing.T) {
	t.Parallel()
	svc := &fakeSvc{
		getSnapshot: func(_ context.Context, _ uuid.UUID) (models.KpiSnapshot, error) {
			return models.KpiSnapshot{}, errorspkg.ErrKpiSnapshotNotFound
		},
	}
	app := newApp(svc)
	req := httptest.NewRequest("GET", "/snapshots/"+uuid.New().String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}

func TestGetSnapshot_BadUUID(t *testing.T) {
	t.Parallel()
	app := newApp(&fakeSvc{})
	req := httptest.NewRequest("GET", "/snapshots/not-a-uuid", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

func TestUpdateCalibration_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &fakeSvc{
		updateCalib: func(_ context.Context, id uuid.UUID, p json.RawMessage) (models.KpiCalibration, error) {
			return models.KpiCalibration{
				ID: id, KpiName: "osa", ScopeType: "global", Params: p,
			}, nil
		},
	}
	app := newApp(svc)
	body := []byte(`{"params":{"lookback_days":14}}`)
	req := httptest.NewRequest("PUT", "/calibrations/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func TestUpdateCalibration_EmptyParams(t *testing.T) {
	t.Parallel()
	app := newApp(&fakeSvc{})
	body := []byte(`{}`)
	req := httptest.NewRequest("PUT", "/calibrations/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

func TestRefreshSnapshots_LockBusy409(t *testing.T) {
	t.Parallel()
	svc := &fakeSvc{
		trigger: func(_ context.Context, _ time.Time, _ []string) (uuid.UUID, bool, error) {
			return uuid.Nil, false, nil
		},
	}
	app := newApp(svc)
	req := httptest.NewRequest("POST", "/snapshots/refresh", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 409, resp.StatusCode)
}

func TestRefreshSnapshots_Started202(t *testing.T) {
	t.Parallel()
	svc := &fakeSvc{
		trigger: func(_ context.Context, _ time.Time, _ []string) (uuid.UUID, bool, error) {
			return uuid.New(), true, nil
		},
	}
	app := newApp(svc)
	req := httptest.NewRequest("POST", "/snapshots/refresh", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 202, resp.StatusCode)
}

func TestRefreshSnapshots_TriggerErrorMappedTo500(t *testing.T) {
	t.Parallel()
	svc := &fakeSvc{
		trigger: func(_ context.Context, _ time.Time, _ []string) (uuid.UUID, bool, error) {
			return uuid.Nil, false, errors.New("boom")
		},
	}
	app := newApp(svc)
	req := httptest.NewRequest("POST", "/snapshots/refresh", bytes.NewReader(nil))
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)
}
