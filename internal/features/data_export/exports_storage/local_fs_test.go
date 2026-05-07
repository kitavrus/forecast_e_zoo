package exports_storage_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/exports_storage"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

func newStorage(t *testing.T) *exports_storage.LocalFSStorage {
	t.Helper()
	root := t.TempDir()
	s, err := exports_storage.NewLocalFS(root)
	require.NoError(t, err)
	return s
}

func TestLocalFS_PutGet_Roundtrip(t *testing.T) {
	t.Parallel()
	s := newStorage(t)
	id := uuid.New()
	body := strings.NewReader(`{"id":"P-1"}` + "\n" + `{"id":"P-2"}` + "\n")
	now := time.Now().UTC()
	meta := exports_storage.Meta{
		Entity: "products", Format: "ndjson",
		SnapshotID: uuid.New().String(),
		Requester:  "test", CreatedAt: now,
	}

	path, err := s.Put(context.Background(), id, "ndjson", body, meta)
	require.NoError(t, err)
	require.NotEmpty(t, path)

	gotPath, gotMeta, err := s.Get(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, path, gotPath)
	require.Equal(t, "products", gotMeta.Entity)
	require.Greater(t, gotMeta.SizeBytes, int64(0))
	require.Equal(t, "ready", gotMeta.Status)
}

func TestLocalFS_Delete_RemovesBoth(t *testing.T) {
	t.Parallel()
	s := newStorage(t)
	id := uuid.New()
	_, err := s.Put(context.Background(), id, "ndjson",
		strings.NewReader("x"),
		exports_storage.Meta{Entity: "products", Format: "ndjson", CreatedAt: time.Now()})
	require.NoError(t, err)

	require.NoError(t, s.Delete(context.Background(), id))
	_, _, err = s.Get(context.Background(), id)
	require.True(t, errors.Is(err, errorspkg.ErrExportNotFound))

	// Delete снова — идемпотентно.
	require.NoError(t, s.Delete(context.Background(), id))
}

func TestLocalFS_ListExpired_ByCreatedAt(t *testing.T) {
	t.Parallel()
	s := newStorage(t)
	now := time.Now().UTC()
	old := uuid.New()
	fresh := uuid.New()

	_, err := s.Put(context.Background(), old, "ndjson",
		strings.NewReader("x"),
		exports_storage.Meta{Entity: "products", Format: "ndjson", CreatedAt: now.Add(-25 * time.Hour)})
	require.NoError(t, err)
	_, err = s.Put(context.Background(), fresh, "ndjson",
		strings.NewReader("x"),
		exports_storage.Meta{Entity: "products", Format: "ndjson", CreatedAt: now})
	require.NoError(t, err)

	expired, err := s.ListExpired(context.Background(), now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Contains(t, expired, old)
	require.NotContains(t, expired, fresh)
}

func TestLocalFS_Get_NotFound_ReturnsErrExportNotFound(t *testing.T) {
	t.Parallel()
	s := newStorage(t)
	_, _, err := s.Get(context.Background(), uuid.New())
	require.True(t, errors.Is(err, errorspkg.ErrExportNotFound))
}
