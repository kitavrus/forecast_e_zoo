package audit_test

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/audit"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
)

type fakeRepo struct {
	mu    sync.Mutex
	calls []repository.AuditAccessInput
	err   error
}

func (f *fakeRepo) InsertAudit(_ context.Context, in repository.AuditAccessInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	return f.err
}

func newApp(w *audit.Writer) *fiber.App {
	app := fiber.New()
	app.Use(w.Middleware())
	app.Get("/admin/loads", func(c fiber.Ctx) error { return c.Status(200).SendString("ok") })
	app.Get("/v1/products", func(c fiber.Ctx) error { return c.Status(200).SendString("ok") })
	return app
}

func TestAuditMiddleware_AdminPath_WritesEntry(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	w := audit.New(repo, nil)
	app := newApp(w)

	req := httptest.NewRequest("GET", "/admin/loads", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	require.Len(t, repo.calls, 1)
	require.Equal(t, "/admin/loads", repo.calls[0].Path)
	require.Equal(t, "GET", repo.calls[0].Method)
	require.Equal(t, 200, repo.calls[0].Status)
}

func TestAuditMiddleware_PublicPath_DoesNotWrite(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	w := audit.New(repo, nil)
	app := newApp(w)

	req := httptest.NewRequest("GET", "/v1/products", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	require.Empty(t, repo.calls)
}

func TestAuditMiddleware_DBError_DoesNotFailRequest(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{err: errors.New("db down")}
	w := audit.New(repo, nil)
	app := newApp(w)

	req := httptest.NewRequest("GET", "/admin/loads", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode, "audit error must not fail request")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func TestAuditMiddleware_NoClaims_StillWrites_RoleEmpty(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	w := audit.New(repo, nil)
	app := newApp(w)

	req := httptest.NewRequest("GET", "/admin/loads", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	require.Len(t, repo.calls, 1)
	require.Equal(t, "", repo.calls[0].ActorRole)
	require.Equal(t, "", repo.calls[0].ActorSub)
}
