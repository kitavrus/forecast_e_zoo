package calculators_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/calculators"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

func dd(v float64) *float64 { return &v }

func TestStockDays_HappyPath(t *testing.T) {
	t.Parallel()
	rows := []models.CalcInputRow{
		{ProductID: "p1", LocationID: "l1", OnHand: 100, InTransit: 20, DailyDemand: dd(10)},
	}
	p := calculators.ParseStockDaysParams(json.RawMessage(`{"include_in_transit":true,"min_daily_demand":0.001,"cap_days":365}`))
	snaps, _ := calculators.ComputeStockDays(rows, p)
	require.Len(t, snaps, 1)
	require.InDelta(t, 12.0, snaps[0].Value, 1e-6) // (100 + 20) / 10
}

func TestStockDays_NoInTransit(t *testing.T) {
	t.Parallel()
	rows := []models.CalcInputRow{
		{ProductID: "p1", LocationID: "l1", OnHand: 100, InTransit: 20, DailyDemand: dd(10)},
	}
	p := calculators.ParseStockDaysParams(json.RawMessage(`{"include_in_transit":false}`))
	snaps, _ := calculators.ComputeStockDays(rows, p)
	require.InDelta(t, 10.0, snaps[0].Value, 1e-6)
}

func TestStockDays_ZeroDailyDemand_CapApplied(t *testing.T) {
	t.Parallel()
	rows := []models.CalcInputRow{
		{ProductID: "p1", LocationID: "l1", OnHand: 100, DailyDemand: nil},
	}
	p := calculators.ParseStockDaysParams(json.RawMessage(`{"min_daily_demand":0.001,"cap_days":365}`))
	snaps, _ := calculators.ComputeStockDays(rows, p)
	require.InDelta(t, 365.0, snaps[0].Value, 1e-6) // capped
}

func TestStockDays_DefaultsWhenParamsEmpty(t *testing.T) {
	t.Parallel()
	p := calculators.ParseStockDaysParams(nil)
	require.True(t, p.IncludeInTransit)
	require.InDelta(t, 0.001, p.MinDailyDemand, 1e-9)
	require.InDelta(t, 365.0, p.CapDays, 1e-6)
}
