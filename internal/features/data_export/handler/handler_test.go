package handler_test

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// --- common mocks ---

type fakePinger struct{ err error }

func (f *fakePinger) Ping(_ context.Context) error { return f.err }

type fakeSnap struct {
	sp  models.SnapshotPointer
	err error
}

func (f *fakeSnap) Current(_ context.Context) (models.SnapshotPointer, error) {
	return f.sp, f.err
}

type fakeProductsRepo struct {
	rows []repository.ProductRow
	err  error
}

func (f *fakeProductsRepo) SelectProducts(_ context.Context, _ uuid.UUID, _ string, _ int) ([]repository.ProductRow, error) {
	return f.rows, f.err
}

type fakeReceiptRepo struct {
	rows []repository.ReceiptLineRow
	err  error
}

func (f *fakeReceiptRepo) SelectReceiptLine(_ context.Context, _ uuid.UUID, _ string, _ int, _ time.Time, _ time.Time) ([]repository.ReceiptLineRow, error) {
	return f.rows, f.err
}

func newApp() *fiber.App { return fiber.New() }

// --- healthz ---

func TestHealthz_OK_DBPing(t *testing.T) {
	t.Parallel()
	app := newApp()
	h := handler.NewHealthzHandler(&fakePinger{})
	app.Get("/healthz", h.Get)

	resp, err := app.Test(httptest.NewRequest("GET", "/healthz", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `"status":"ok"`)
}

func TestHealthz_DBError_503(t *testing.T) {
	t.Parallel()
	app := newApp()
	h := handler.NewHealthzHandler(&fakePinger{err: errors.New("conn refused")})
	app.Get("/healthz", h.Get)

	resp, err := app.Test(httptest.NewRequest("GET", "/healthz", nil))
	require.NoError(t, err)
	require.Equal(t, 503, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `"db":"unreachable"`)
}

// --- snapshots ---

func TestSnapshotsCurrent_HappyPath(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	app := newApp()
	h := handler.NewSnapshotsHandler(&fakeSnap{sp: models.SnapshotPointer{CurrentLoadID: &id}})
	app.Get("/v1/snapshots/current", h.GetCurrent)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/snapshots/current", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, id.String(), resp.Header.Get("X-Snapshot-Id"))
}

func TestSnapshotsCurrent_NotReady_503(t *testing.T) {
	t.Parallel()
	app := newApp()
	h := handler.NewSnapshotsHandler(&fakeSnap{err: errorspkg.ErrSnapshotNotReady})
	app.Get("/v1/snapshots/current", h.GetCurrent)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/snapshots/current", nil))
	require.NoError(t, err)
	require.Equal(t, 503, resp.StatusCode)
	require.Equal(t, "60", resp.Header.Get("Retry-After"))
}

// --- products ---

func TestProducts_HappyPath_Returns200_NDJSON(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	app := newApp()
	now := time.Now()
	h := handler.NewProductsHandler(
		&fakeProductsRepo{rows: []repository.ProductRow{
			{ID: "P-1", SKU: "S1", Name: "n1", Unit: "kg", IsActive: true},
			{ID: "P-2", SKU: "S2", Name: "n2", Unit: "kg", IsActive: false},
		}},
		&fakeSnap{sp: models.SnapshotPointer{CurrentLoadID: &id, CommittedAt: &now}},
	)
	app.Get("/v1/products", h.Get)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/products", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/x-ndjson")
	require.NotEmpty(t, resp.Header.Get("ETag"))

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	require.Len(t, lines, 2)
}

func TestProducts_BadCursor_Returns400(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	app := newApp()
	now := time.Now()
	h := handler.NewProductsHandler(
		&fakeProductsRepo{},
		&fakeSnap{sp: models.SnapshotPointer{CurrentLoadID: &id, CommittedAt: &now}},
	)
	app.Get("/v1/products", h.Get)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/products?cursor=@@@bad@@@", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

func TestProducts_IfNoneMatchSameETag_Returns304(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	app := newApp()
	now := time.Now()
	h := handler.NewProductsHandler(
		&fakeProductsRepo{rows: []repository.ProductRow{{ID: "P-1", SKU: "S1", Name: "n", Unit: "kg"}}},
		&fakeSnap{sp: models.SnapshotPointer{CurrentLoadID: &id, CommittedAt: &now}},
	)
	app.Get("/v1/products", h.Get)

	expected := handler.ComputeETag(id, "products", now)
	req := httptest.NewRequest("GET", "/v1/products", nil)
	req.Header.Set(fiber.HeaderIfNoneMatch, expected)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 304, resp.StatusCode)
}

func TestProducts_SnapshotNotReady_503(t *testing.T) {
	t.Parallel()
	app := newApp()
	h := handler.NewProductsHandler(
		&fakeProductsRepo{},
		&fakeSnap{err: errorspkg.ErrSnapshotNotReady},
	)
	app.Get("/v1/products", h.Get)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/products", nil))
	require.NoError(t, err)
	require.Equal(t, 503, resp.StatusCode)
}

// --- receipt_line ---

func TestReceiptLine_RequiresEventDateRange_Returns400IfMissing(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	app := newApp()
	now := time.Now()
	h := handler.NewReceiptLineHandler(
		&fakeReceiptRepo{},
		&fakeSnap{sp: models.SnapshotPointer{CurrentLoadID: &id, CommittedAt: &now}},
	)
	app.Get("/v1/receipt_line", h.Get)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/receipt_line", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

func TestReceiptLine_DateRange_HappyPath(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	app := newApp()
	now := time.Now()
	h := handler.NewReceiptLineHandler(
		&fakeReceiptRepo{rows: []repository.ReceiptLineRow{{
			ID: 1, ReceiptID: "R-1", LocationID: "L-1", ProductID: "P-1",
			Qty: 1, Price: 10, EventTime: now, EventDate: now,
		}}},
		&fakeSnap{sp: models.SnapshotPointer{CurrentLoadID: &id, CommittedAt: &now}},
	)
	app.Get("/v1/receipt_line", h.Get)

	resp, err := app.Test(httptest.NewRequest(
		"GET",
		"/v1/receipt_line?event_date_from=2026-05-01&event_date_to=2026-05-07",
		nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// --- ETag and streaming ---

func TestComputeETag_DeterministicForSameInputs(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	a := handler.ComputeETag(id, "products", now)
	b := handler.ComputeETag(id, "products", now)
	require.Equal(t, a, b)
}

func TestComputeETag_ChangesWhenLoadIDChanges(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := handler.ComputeETag(uuid.New(), "products", now)
	b := handler.ComputeETag(uuid.New(), "products", now)
	require.NotEqual(t, a, b)
}

func TestStreamNDJSON_WritesLineByLine(t *testing.T) {
	t.Parallel()
	app := newApp()
	app.Get("/items", func(c fiber.Ctx) error {
		return handler.StreamNDJSON(c, []map[string]any{{"a": 1}, {"a": 2}})
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/items", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "{\"a\":1}\n{\"a\":2}\n", string(body))
}

// --- not implemented placeholder ---

// TestNotImplemented_Returns501WithEntityName — после фикса блокера #1 ревью
// (см. design-errors.md ErrNotImplemented) заглушки 14 entity отдают 501,
// а не 500. Это нужно для алертов: 5xx остаётся внутренней ошибкой,
// а 501 — ожидаемый «фича пост-MVP».
func TestNotImplemented_Returns501WithEntityName(t *testing.T) {
	t.Parallel()
	app := newApp()
	ni := handler.NotImplementedHandlers{}
	app.Get("/v1/category", ni.Category())
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/category", nil))
	require.NoError(t, err)
	require.Equal(t, 501, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "category")
	require.Contains(t, string(body), "not_implemented")
}
