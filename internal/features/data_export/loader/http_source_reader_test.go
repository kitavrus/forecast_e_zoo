package loader

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestReader(srvURL string, opts ...func(*HTTPSourceReaderConfig)) *HTTPSourceReader {
	cfg := HTTPSourceReaderConfig{
		BaseURL:      srvURL,
		HTTPTimeout:  2 * time.Second,
		RetryMax:     3,
		BackoffCap:   100 * time.Millisecond,
		RequestLimit: 100,
	}
	for _, o := range opts {
		o(&cfg)
	}
	r := NewHTTPSourceReader(cfg)
	r.sleepFn = func(time.Duration) {} // no-sleep в тестах
	return r
}

func collect[T any](t *testing.T, it PageIterator[T], err error) []T {
	t.Helper()
	if err != nil {
		t.Fatalf("reader err: %v", err)
	}
	defer func() { _ = it.Close() }()
	out := make([]T, 0)
	for it.Next(context.Background()) {
		out = append(out, it.Item())
	}
	if iterErr := it.Err(); iterErr != nil {
		t.Fatalf("iterator err: %v", iterErr)
	}
	return out
}

func TestHTTPSourceReader_ReadProducts_HappyPath(t *testing.T) {
	t.Parallel()

	products := []ErpProduct{
		{ID: "p1", SKU: "SKU-1", Name: "A", Unit: "ea", IsActive: true, UpdatedAt: time.Now().UTC()},
		{ID: "p2", SKU: "SKU-2", Name: "B", Unit: "ea", IsActive: true, UpdatedAt: time.Now().UTC()},
		{ID: "p3", SKU: "SKU-3", Name: "C", Unit: "ea", IsActive: false, UpdatedAt: time.Now().UTC()},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/products" {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(products)
	}))
	defer srv.Close()

	r := newTestReader(srv.URL)
	defer func() { _ = r.Close(context.Background()) }()

	it, err := r.ReadProducts(context.Background(), time.Time{})
	got := collect(t, it, err)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	if got[0].ID != "p1" || got[2].ID != "p3" {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestHTTPSourceReader_RetryOn5xx(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	r := newTestReader(srv.URL)
	defer func() { _ = r.Close(context.Background()) }()

	it, err := r.ReadProducts(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	_ = it.Close()
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls (1 failed + 1 success), got %d", calls.Load())
	}
}

func TestHTTPSourceReader_NoRetryOn4xx(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `unauthorized`)
	}))
	defer srv.Close()

	r := newTestReader(srv.URL)
	defer func() { _ = r.Close(context.Background()) }()

	_, err := r.ReadProducts(context.Background(), time.Time{})
	if err == nil {
		t.Fatalf("expected 401 error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected error to contain 401, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call (no retry on 4xx), got %d", calls.Load())
	}
}

func TestHTTPSourceReader_Pagination(t *testing.T) {
	t.Parallel()

	page1 := []ErpProduct{{ID: "p1", SKU: "S1", Name: "A", Unit: "ea", UpdatedAt: time.Now().UTC()}}
	page2 := []ErpProduct{{ID: "p2", SKU: "S2", Name: "B", Unit: "ea", UpdatedAt: time.Now().UTC()}}

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			if got := req.URL.Query().Get("cursor"); got != "" {
				t.Errorf("first page should have empty cursor, got %q", got)
			}
			w.Header().Set("X-Next-Cursor", "page2-cursor")
			_ = json.NewEncoder(w).Encode(page1)
		case 2:
			if got := req.URL.Query().Get("cursor"); got != "page2-cursor" {
				t.Errorf("second page cursor mismatch: %q", got)
			}
			// no X-Next-Cursor → end.
			_ = json.NewEncoder(w).Encode(page2)
		default:
			t.Errorf("unexpected 3rd call")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestReader(srv.URL)
	defer func() { _ = r.Close(context.Background()) }()

	it, err := r.ReadProducts(context.Background(), time.Time{})
	got := collect(t, it, err)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows across 2 pages, got %d", len(got))
	}
	if got[0].ID != "p1" || got[1].ID != "p2" {
		t.Fatalf("page order broken: %+v", got)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", calls.Load())
	}
}

func TestHTTPSourceReader_APIKey(t *testing.T) {
	t.Parallel()

	const apiKey = "secret-key-123"
	var sawKey atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-API-Key") == apiKey {
			sawKey.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	r := newTestReader(srv.URL, func(c *HTTPSourceReaderConfig) { c.APIKey = apiKey })
	defer func() { _ = r.Close(context.Background()) }()

	it, err := r.ReadProducts(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_ = it.Close()
	if !sawKey.Load() {
		t.Fatalf("X-API-Key header was not delivered")
	}
}

func TestHTTPSourceReader_FactsQueryParams(t *testing.T) {
	t.Parallel()

	var got struct {
		dateFrom string
		dateTo   string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		got.dateFrom = req.URL.Query().Get("date_from")
		got.dateTo = req.URL.Query().Get("date_to")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	r := newTestReader(srv.URL)
	defer func() { _ = r.Close(context.Background()) }()

	df := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	dt := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	it, err := r.ReadReceiptLine(context.Background(), df, dt)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_ = it.Close()

	if got.dateFrom != "2026-04-01" || got.dateTo != "2026-04-30" {
		t.Fatalf("query mismatch: from=%q to=%q", got.dateFrom, got.dateTo)
	}
}

func TestHTTPSourceReader_RetryExhausted(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	r := newTestReader(srv.URL, func(c *HTTPSourceReaderConfig) { c.RetryMax = 2 })
	defer func() { _ = r.Close(context.Background()) }()

	_, err := r.ReadProducts(context.Background(), time.Time{})
	if err == nil {
		t.Fatalf("expected retry exhausted error")
	}
	if !strings.Contains(err.Error(), "retry exhausted") {
		t.Fatalf("expected retry exhausted in error, got %v", err)
	}
}

// Sanity: sinceQuery формирует ISO-8601 для master.
func TestHTTPSourceReader_MasterSinceParam(t *testing.T) {
	t.Parallel()

	var sinceParam string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		sinceParam = req.URL.Query().Get("since")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	r := newTestReader(srv.URL)
	defer func() { _ = r.Close(context.Background()) }()

	since := time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC)
	it, err := r.ReadProducts(context.Background(), since)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_ = it.Close()

	want := since.Format(time.RFC3339)
	if sinceParam != want {
		t.Fatalf("since param mismatch: got=%q want=%q", sinceParam, want)
	}
}

// Compile-time assertion: HTTPSourceReader реализует SourceReader.
var _ SourceReader = (*HTTPSourceReader)(nil)
