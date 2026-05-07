package snapshot_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/snapshot"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// fakeTx — минимальный pgx.Tx для тестов (см. mocks_test.go в loader-е).
type fakeTx struct{ committed, rolledBack bool }

func (t *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }
func (t *fakeTx) BeginFunc(_ context.Context, _ func(pgx.Tx) error) error {
	return nil
}
func (t *fakeTx) Commit(_ context.Context) error   { t.committed = true; return nil }
func (t *fakeTx) Rollback(_ context.Context) error { t.rolledBack = true; return nil }
func (t *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                              { return pgx.LargeObjects{} }
func (t *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (t *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }
func (t *fakeTx) Conn() *pgx.Conn                                                { return nil }

type fakeRepo struct {
	current    models.SnapshotPointer
	currentErr error
	flipReturn models.SnapshotPointer
	flipErr    error

	tx fakeTx
}

func (f *fakeRepo) GetCurrent(_ context.Context) (models.SnapshotPointer, error) {
	return f.current, f.currentErr
}
func (f *fakeRepo) Flip(_ context.Context, _ pgx.Tx, _ uuid.UUID) (models.SnapshotPointer, error) {
	return f.flipReturn, f.flipErr
}
func (f *fakeRepo) BeginTx(_ context.Context) (pgx.Tx, error) { return &f.tx, nil }

func TestSnapshotService_Current_HappyPath(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	repo := &fakeRepo{current: models.SnapshotPointer{CurrentLoadID: &id}}
	s := snapshot.New(repo, nil)
	cur, err := s.Current(context.Background())
	require.NoError(t, err)
	require.Equal(t, id, *cur.CurrentLoadID)
}

func TestSnapshotService_Current_NotReady_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{currentErr: errorspkg.ErrSnapshotNotReady}
	s := snapshot.New(repo, nil)
	_, err := s.Current(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrSnapshotNotReady))
}

func TestSnapshotService_Flip_DelegatesToRepo(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	repo := &fakeRepo{flipReturn: models.SnapshotPointer{CurrentLoadID: &id}}
	s := snapshot.New(repo, nil)
	got, err := s.Flip(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, *got.CurrentLoadID)
	require.True(t, repo.tx.committed)
	require.False(t, repo.tx.rolledBack)
}

func TestSnapshotService_Flip_RepoError_Propagates(t *testing.T) {
	t.Parallel()
	boom := errors.New("flip boom")
	repo := &fakeRepo{flipErr: boom}
	s := snapshot.New(repo, nil)
	_, err := s.Flip(context.Background(), uuid.New())
	require.Error(t, err)
	require.True(t, errors.Is(err, boom))
	require.True(t, repo.tx.rolledBack)
	require.False(t, repo.tx.committed)
}
