//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

func TestEtlRunsRepository_InsertGetUpdateList(t *testing.T) {
	skipIfDocker(t)
	ctx := context.Background()
	setupFixture(t)
	truncateAll(t, ctx)
	repo := repository.New(fix.pool)

	now := time.Now().UTC()
	run := &models.EtlRun{
		ID:           uuid.New(),
		StartedAt:    now,
		Status:       constants.StatusRunning,
		Kind:         constants.KindFull,
		Trigger:      constants.TriggerCron,
		MartsSummary: []byte(`{}`),
	}
	require.NoError(t, repo.InsertEtlRun(ctx, run))

	got, err := repo.GetEtlRunByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.Status, got.Status)
	assert.Equal(t, run.Kind, got.Kind)
	assert.Equal(t, run.Trigger, got.Trigger)

	finishedAt := time.Now().UTC()
	committedAt := finishedAt
	require.NoError(t, repo.UpdateEtlRunStatus(ctx, run.ID, repository.EtlRunStatusPatch{
		Status:       constants.StatusCommitted,
		FinishedAt:   &finishedAt,
		CommittedAt:  &committedAt,
		MartsSummary: []byte(`{"mart_demand_history":{"rows":10}}`),
	}))

	got, err = repo.GetEtlRunByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, constants.StatusCommitted, got.Status)
	require.NotNil(t, got.FinishedAt)
	require.NotNil(t, got.CommittedAt)

	list, err := repo.ListEtlRuns(ctx, repository.EtlRunListFilter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestEtlRunsRepository_GetByID_NotFound(t *testing.T) {
	skipIfDocker(t)
	ctx := context.Background()
	setupFixture(t)
	repo := repository.New(fix.pool)

	_, err := repo.GetEtlRunByID(ctx, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, errorspkg.ErrEtlRunNotFound),
		"expected ErrEtlRunNotFound, got %v", err)
}

func TestEtlRunsRepository_GetCurrentRunning_None(t *testing.T) {
	skipIfDocker(t)
	ctx := context.Background()
	setupFixture(t)
	truncateAll(t, ctx)
	repo := repository.New(fix.pool)

	_, err := repo.GetCurrentRunningEtlRun(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errorspkg.ErrEtlRunNotFound))
}
