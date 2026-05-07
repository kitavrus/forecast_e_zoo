//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
)

func TestAuditAccess_Insert(t *testing.T) {
	skipIfDocker(t)
	ctx := context.Background()
	setupFixture(t)
	truncateAll(t, ctx)
	repo := repository.New(fix.pool)

	requester := "etl-admin"
	role := "admin"
	statusCode := 200
	reqID := "req-123"

	require.NoError(t, repo.InsertAuditAccess(ctx, models.AuditAccessEntry{
		Method:     "POST",
		Path:       "/admin/etl-runs",
		Requester:  &requester,
		Role:       &role,
		StatusCode: &statusCode,
		RequestID:  &reqID,
	}))

	var n int
	err := fix.pool.QueryRow(ctx, "SELECT count(*) FROM marts.audit_access").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}
