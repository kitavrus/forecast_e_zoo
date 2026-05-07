package calculators_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/calculators"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

func TestOSA_HappyPath(t *testing.T) {
	t.Parallel()
	rows := []models.DemandHistoryAgg{
		{ProductID: "p1", LocationID: "l1", DaysObserved: 30, DaysOOS: 3},
	}
	params := calculators.ParseOSAParams(json.RawMessage(`{"lookback_days":30,"min_observations":7}`))
	snaps, errs := calculators.ComputeOSA(rows, params)
	require.Equal(t, 0, errs)
	require.Len(t, snaps, 1)
	require.InDelta(t, 0.9, snaps[0].Value, 1e-6) // 1 - 3/30
}

func TestOSA_BelowMinObservationsDropped(t *testing.T) {
	t.Parallel()
	rows := []models.DemandHistoryAgg{
		{ProductID: "p1", LocationID: "l1", DaysObserved: 3, DaysOOS: 1},
	}
	params := calculators.ParseOSAParams(json.RawMessage(`{"min_observations":7}`))
	snaps, _ := calculators.ComputeOSA(rows, params)
	require.Empty(t, snaps)
}

func TestOSA_AlwaysOOS_Returns0(t *testing.T) {
	t.Parallel()
	rows := []models.DemandHistoryAgg{
		{ProductID: "p1", LocationID: "l1", DaysObserved: 30, DaysOOS: 30},
	}
	params := calculators.ParseOSAParams(nil)
	snaps, _ := calculators.ComputeOSA(rows, params)
	require.Len(t, snaps, 1)
	require.InDelta(t, 0.0, snaps[0].Value, 1e-6)
}

func TestOSA_DefaultsWhenParamsEmpty(t *testing.T) {
	t.Parallel()
	p := calculators.ParseOSAParams(nil)
	require.Equal(t, 30, p.LookbackDays)
	require.Equal(t, 7, p.MinObservations)
}
