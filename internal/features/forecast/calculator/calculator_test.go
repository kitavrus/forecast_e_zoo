package calculator_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/calculator"
)

func TestCalculator_HappyPath(t *testing.T) {
	t.Parallel()
	c := calculator.New()

	res := c.Compute(calculator.Input{
		ProductID:    "p1",
		LocationID:   "l1",
		CurrentStock: 50,
		InTransit:    10,
		DailyDemand:  10,
		LeadTimeDays: 7,
		StdDev:       2,
	})

	// lead_time_demand = 10 × 7 = 70
	// safety_stock     = 1.96 × 2 × sqrt(7) ≈ 10.37
	// reorder_point    = 10.37 + 70 ≈ 80.37
	// cycle_stock      = 10 × 7 = 70
	// target_stock     = 80.37 + 70 ≈ 150.37
	// reorder_qty      = 150.37 − 50 − 10 ≈ 90.37
	require.InDelta(t, 70.0, res.DailyDemand*float64(res.LeadTimeDays), 0.01)
	require.Greater(t, res.SafetyStock, 10.0)
	require.Less(t, res.SafetyStock, 11.0)
	require.InDelta(t, 70.0, res.ReorderPoint-res.SafetyStock, 0.01)
	require.Greater(t, res.ReorderQty, 89.0)
	require.Less(t, res.ReorderQty, 91.0)
}

func TestCalculator_ZeroDemand(t *testing.T) {
	t.Parallel()
	c := calculator.New()
	res := c.Compute(calculator.Input{
		CurrentStock: 100,
		InTransit:    0,
		DailyDemand:  0,
		LeadTimeDays: 5,
	})
	require.Equal(t, 0.0, res.ReorderQty,
		"no demand → no reorder")
	require.Equal(t, 0.0, res.SafetyStock)
}

func TestCalculator_StockExceedsTarget_NoReorder(t *testing.T) {
	t.Parallel()
	c := calculator.New()
	res := c.Compute(calculator.Input{
		CurrentStock: 1000,
		InTransit:    0,
		DailyDemand:  5,
		LeadTimeDays: 7,
	})
	require.Equal(t, 0.0, res.ReorderQty,
		"target much smaller than current_stock → 0")
}

func TestCalculator_FallbackLeadTime(t *testing.T) {
	t.Parallel()
	c := calculator.New()
	res := c.Compute(calculator.Input{
		DailyDemand:  10,
		LeadTimeDays: 0, // → fallback 7
	})
	require.Equal(t, 7, res.LeadTimeDays,
		"lead_time_days=0 → fallback 7")
}

func TestCalculator_BatchAggregates(t *testing.T) {
	t.Parallel()
	c := calculator.New()
	inputs := []calculator.Input{
		{ProductID: "a", LocationID: "l", DailyDemand: 5, LeadTimeDays: 7},
		{ProductID: "b", LocationID: "l", DailyDemand: 10, LeadTimeDays: 7},
	}
	out := c.ComputeBatch(inputs)
	require.Len(t, out, 2)
	require.Equal(t, "a", out[0].ProductID)
	require.Equal(t, "b", out[1].ProductID)
}
