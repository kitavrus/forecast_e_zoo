//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
)

func TestRejectLog_BulkInsertAndList(t *testing.T) {
	skipIfDocker(t)
	ctx := context.Background()
	setupFixture(t)
	truncateAll(t, ctx)
	repo := repository.New(fix.pool)

	runID := uuid.New()
	require.NoError(t, repo.InsertEtlRun(ctx, &models.EtlRun{
		ID:           runID,
		StartedAt:    time.Now().UTC(),
		Status:       constants.StatusRunning,
		Kind:         constants.KindFull,
		Trigger:      constants.TriggerCron,
		MartsSummary: []byte(`{}`),
	}))

	const total = 250
	entries := make([]models.RejectLogEntry, 0, total)
	for i := 0; i < total; i++ {
		sev := constants.SeveritySoft
		if i%5 == 0 {
			sev = constants.SeverityCritical
		}
		entries = append(entries, models.RejectLogEntry{
			EtlRunID: runID,
			Entity:   "receipt_line",
			Severity: sev,
			RuleID:   "duplicate_pk",
			Message:  "duplicate row",
		})
	}

	n, err := repo.InsertRejectEntries(ctx, entries)
	require.NoError(t, err)
	assert.Equal(t, int64(total), n)

	list, err := repo.ListRejectEntries(ctx, repository.RejectLogListFilter{
		EtlRunID: &runID,
		Severity: constants.SeverityCritical,
		Limit:    1000,
	})
	require.NoError(t, err)
	assert.Equal(t, total/5, len(list)) // 50 critical
}
