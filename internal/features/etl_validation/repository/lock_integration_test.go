//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
)

func TestLock_TryAdvisoryXactLock_Contention(t *testing.T) {
	skipIfDocker(t)
	ctx := context.Background()
	setupFixture(t)
	repo := repository.New(fix.pool)

	const key int64 = 0xE7F123

	tx1, err := fix.pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)

	ok, err := repo.TryAdvisoryXactLock(ctx, tx1, key)
	require.NoError(t, err)
	require.True(t, ok)

	tx2, err := fix.pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)

	ok2, err := repo.TryAdvisoryXactLock(ctx, tx2, key)
	require.NoError(t, err)
	assert.False(t, ok2, "second tx must not acquire same lock")

	require.NoError(t, tx2.Rollback(ctx))
	require.NoError(t, tx1.Rollback(ctx))

	// После Rollback первой tx — следующая попытка должна получить lock.
	tx3, err := fix.pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx3.Rollback(ctx) }()
	ok3, err := repo.TryAdvisoryXactLock(ctx, tx3, key)
	require.NoError(t, err)
	assert.True(t, ok3, "after first tx rollback, lock must be available")
}
