package calculators_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/calculators"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

func fr(v float64) *float64 { return &v }

func TestOTIF_HappyPath(t *testing.T) {
	t.Parallel()
	rows := []models.SupplierScorecardRow{
		{
			SupplierID: "sup-1", WeekStart: time.Now(),
			LinesDelivered: 100, LinesLate: 5,
			FillRateAvg: fr(0.99),
		},
	}
	params := calculators.ParseOTIFParams(json.RawMessage(`{"fill_rate_threshold":0.95}`))
	snaps, errs := calculators.ComputeOTIF(rows, params)
	require.Equal(t, 0, errs)
	require.Len(t, snaps, 1)
	require.InDelta(t, 0.95, snaps[0].Value, 1e-6) // 1 - 5/100
}

func TestOTIF_ZeroDeliveriesDropped(t *testing.T) {
	t.Parallel()
	rows := []models.SupplierScorecardRow{
		{SupplierID: "sup-1", WeekStart: time.Now(), LinesDelivered: 0},
	}
	snaps, _ := calculators.ComputeOTIF(rows, calculators.ParseOTIFParams(nil))
	require.Empty(t, snaps)
}

func TestOTIF_LowFillRateBelowThreshold_AllShort(t *testing.T) {
	t.Parallel()
	rows := []models.SupplierScorecardRow{
		{
			SupplierID: "sup-1", WeekStart: time.Now(),
			LinesDelivered: 100, LinesLate: 0,
			FillRateAvg: fr(0.80),
		},
	}
	snaps, _ := calculators.ComputeOTIF(rows, calculators.ParseOTIFParams(json.RawMessage(`{"fill_rate_threshold":0.95}`)))
	require.Len(t, snaps, 1)
	require.InDelta(t, 0.0, snaps[0].Value, 1e-6) // weekly считается полностью short
}

func TestOTIF_DefaultsWhenParamsEmpty(t *testing.T) {
	t.Parallel()
	p := calculators.ParseOTIFParams(nil)
	require.InDelta(t, 0.95, p.FillRateThreshold, 1e-6)
}
