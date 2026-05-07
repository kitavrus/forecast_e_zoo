package scheduler_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/exports_storage"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/scheduler"
)

func newStorage(t *testing.T) *exports_storage.LocalFSStorage {
	t.Helper()
	s, err := exports_storage.NewLocalFS(t.TempDir())
	require.NoError(t, err)
	return s
}

func TestCleanup_RemovesExpired(t *testing.T) {
	t.Parallel()
	s := newStorage(t)
	old := uuid.New()
	_, err := s.Put(context.Background(), old, "ndjson",
		strings.NewReader("x"),
		exports_storage.Meta{Entity: "products", Format: "ndjson", CreatedAt: time.Now().Add(-48 * time.Hour)})
	require.NoError(t, err)

	deleted, err := scheduler.CleanupOnce(context.Background(), s, 24*time.Hour, nil)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	exp, err := s.ListExpired(context.Background(), time.Now().Add(-1*time.Hour))
	require.NoError(t, err)
	require.NotContains(t, exp, old)
}

func TestCleanup_KeepsFresh(t *testing.T) {
	t.Parallel()
	s := newStorage(t)
	fresh := uuid.New()
	_, err := s.Put(context.Background(), fresh, "ndjson",
		strings.NewReader("x"),
		exports_storage.Meta{Entity: "products", Format: "ndjson", CreatedAt: time.Now()})
	require.NoError(t, err)

	deleted, err := scheduler.CleanupOnce(context.Background(), s, 24*time.Hour, nil)
	require.NoError(t, err)
	require.Equal(t, 0, deleted)
}
