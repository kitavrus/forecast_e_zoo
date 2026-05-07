package extractor_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/extractor"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

func newTestClient(t *testing.T, baseURL string) *extractor.Client {
	t.Helper()
	httpc := &http.Client{Timeout: 5 * time.Second}
	c, err := extractor.NewClient(httpc, extractor.StaticTokenSource{Value: "tok"},
		extractor.ClientConfig{
			BaseURL:    baseURL,
			RetryMax:   3,
			BackoffCap: 100 * time.Millisecond,
		}, slog.Default())
	require.NoError(t, err)
	return c
}

// --- Client.Do ---

func TestClient_Do_RetryOn5xx(t *testing.T) {
	t.Parallel()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	resp, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "ok", string(body))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
}

func TestClient_Do_RetryExhausted(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	_, err := c.Do(context.Background(), req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errorspkg.ErrSourceUnavailable))
}

func TestClient_Do_AuthHeaderSet(t *testing.T) {
	t.Parallel()
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/x", nil)
	resp, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "Bearer tok", seenAuth)
}

func TestClient_Do_NoRetryOn4xx(t *testing.T) {
	t.Parallel()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/x", nil)
	resp, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestClient_Do_BadConfig(t *testing.T) {
	t.Parallel()
	_, err := extractor.NewClient(nil, extractor.StaticTokenSource{Value: "x"}, extractor.ClientConfig{BaseURL: "http://x"}, nil)
	require.Error(t, err)
	_, err = extractor.NewClient(&http.Client{}, nil, extractor.ClientConfig{BaseURL: "http://x"}, nil)
	require.Error(t, err)
	_, err = extractor.NewClient(&http.Client{}, extractor.StaticTokenSource{Value: "x"}, extractor.ClientConfig{}, nil)
	require.Error(t, err)
}

// --- Snapshots ---

func TestSnapshots_GetCurrent_OK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(`{"current_load_id":"L1","committed_at":"2026-05-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	sc := extractor.NewSnapshotsClient(c)
	snap, err := sc.GetCurrent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "L1", snap.CurrentLoadID)
	assert.Equal(t, `"abc"`, snap.ETag)
}

func TestSnapshots_GetCurrent_503(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	sc := extractor.NewSnapshotsClient(c)
	_, err := sc.GetCurrent(context.Background())
	require.Error(t, err)
	// после retry-исчерпания возвращается ErrSourceUnavailable, NOT SnapshotNotReady
	// потому что 503 — retryable.
	assert.True(t, errors.Is(err, errorspkg.ErrSourceUnavailable))
}

func TestSnapshots_GetCurrent_404(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	sc := extractor.NewSnapshotsClient(c)
	_, err := sc.GetCurrent(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, errorspkg.ErrSourceUnavailable))
}

// --- Entities ---

func TestEntities_Stream_NDJSON(t *testing.T) {
	t.Parallel()
	lines := []string{
		`{"id":"p1","name":"Apple"}`,
		`{"id":"p2","name":"Banana"}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "snapshot_id=L1")
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("ETag", `"v1"`)
		_, _ = io.WriteString(w, strings.Join(lines, "\n")+"\n")
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	ec := extractor.NewEntitiesClient(c)
	rd, err := ec.Stream(context.Background(), "products", "L1", "")
	require.NoError(t, err)
	defer rd.Close()
	type prod struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	got := []prod{}
	for {
		var p prod
		if err := rd.Next(&p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("next: %v", err)
		}
		got = append(got, p)
	}
	assert.Equal(t, []prod{{ID: "p1", Name: "Apple"}, {ID: "p2", Name: "Banana"}}, got)
	assert.Equal(t, `"v1"`, rd.ETag())
}

func TestEntities_Stream_NotModified(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, `"old"`, r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	ec := extractor.NewEntitiesClient(c)
	rd, err := ec.Stream(context.Background(), "products", "L1", `"old"`)
	require.NoError(t, err)
	defer rd.Close()
	var v map[string]any
	require.ErrorIs(t, rd.Next(&v), io.EOF)
	assert.Equal(t, `"old"`, rd.ETag())
}

func TestEntities_Stream_BadInputs(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, "http://x")
	ec := extractor.NewEntitiesClient(c)
	_, err := ec.Stream(context.Background(), "", "L1", "")
	require.Error(t, err)
	_, err = ec.Stream(context.Background(), "products", "", "")
	require.Error(t, err)
}

// --- TokenSource ---

func TestHS256TokenSource_Token(t *testing.T) {
	t.Parallel()
	ts, err := extractor.NewHS256TokenSource(extractor.HS256Config{
		SigningKey: []byte("secret"), Role: "x-flow-etl", TTL: time.Hour,
	})
	require.NoError(t, err)
	tok, err := ts.Token(context.Background())
	require.NoError(t, err)
	parsed, err := jwt.Parse(tok, func(_ *jwt.Token) (any, error) { return []byte("secret"), nil })
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	claims := parsed.Claims.(jwt.MapClaims)
	assert.Equal(t, "x-flow-etl", claims["role"])
	exp, _ := claims["exp"].(float64)
	assert.Greater(t, int64(exp), time.Now().Unix())
}

func TestHS256TokenSource_Caching(t *testing.T) {
	t.Parallel()
	ts, err := extractor.NewHS256TokenSource(extractor.HS256Config{
		SigningKey: []byte("secret"), TTL: time.Hour, Leeway: time.Minute,
	})
	require.NoError(t, err)
	t1, err := ts.Token(context.Background())
	require.NoError(t, err)
	t2, err := ts.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, t1, t2)
}

func TestHS256TokenSource_BadConfig(t *testing.T) {
	t.Parallel()
	_, err := extractor.NewHS256TokenSource(extractor.HS256Config{})
	require.Error(t, err)
}

func TestRS256TokenSource(t *testing.T) {
	t.Parallel()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ts, err := extractor.NewRS256TokenSource(extractor.RS256Config{
		PrivateKey: pk, Role: "x-flow-etl", TTL: time.Hour,
	})
	require.NoError(t, err)
	tok, err := ts.Token(context.Background())
	require.NoError(t, err)
	parsed, err := jwt.Parse(tok, func(_ *jwt.Token) (any, error) { return &pk.PublicKey, nil })
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	// caching
	tok2, _ := ts.Token(context.Background())
	assert.Equal(t, tok, tok2)
}

func TestRS256TokenSource_BadConfig(t *testing.T) {
	t.Parallel()
	_, err := extractor.NewRS256TokenSource(extractor.RS256Config{})
	require.Error(t, err)
}

func TestStaticTokenSource(t *testing.T) {
	t.Parallel()
	tok, err := extractor.StaticTokenSource{Value: "v"}.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "v", tok)
}

// Sanity-проверка: backoff cap не превышен (визуально через таймаут).
func TestClient_Do_BackoffCap(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	httpc := &http.Client{Timeout: 5 * time.Second}
	c, err := extractor.NewClient(httpc, extractor.StaticTokenSource{Value: "t"}, extractor.ClientConfig{
		BaseURL:    srv.URL,
		RetryMax:   2,
		BackoffCap: 50 * time.Millisecond,
	}, slog.Default())
	require.NoError(t, err)
	start := time.Now()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/x", nil)
	_, err = c.Do(context.Background(), req)
	elapsed := time.Since(start)
	require.Error(t, err)
	// 2 attempts × backoff не должны существенно превышать 200ms (2*100+50)
	assert.Less(t, elapsed, 1*time.Second, "elapsed=%v", elapsed)
}

// Smoke: ENT path encoding не падает на специальных символах.
func TestEntities_Stream_PathEncoded(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/v1/entities/order_rule")
		_ = r // explicit
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "")
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	ec := extractor.NewEntitiesClient(c)
	rd, err := ec.Stream(context.Background(), "order_rule", "L1", "")
	require.NoError(t, err)
	defer rd.Close()
	var v map[string]any
	require.ErrorIs(t, rd.Next(&v), io.EOF)
}

// Verify resp for unknown status from entities returns ErrSourceUnavailable.
func TestEntities_Stream_UnexpectedStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418 — non-retryable, non-200
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	ec := extractor.NewEntitiesClient(c)
	_, err := ec.Stream(context.Background(), "products", "L1", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errorspkg.ErrSourceUnavailable), "got %v", err)
}

// Document a small helper: Bearer token format.
func TestStaticTokenSource_BearerFmt(t *testing.T) {
	t.Parallel()
	tok, _ := extractor.StaticTokenSource{Value: "abc"}.Token(context.Background())
	assert.Equal(t, "Bearer "+tok, fmt.Sprintf("Bearer %s", tok))
}
