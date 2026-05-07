package constructor_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/constructor"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

func TestConstructor_GroupsBySupplierAndLocation(t *testing.T) {
	t.Parallel()
	planDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	supS1 := "s1"
	supS2 := "s2"
	lines := []models.CalculationLine{
		{ProductID: "p1", LocationID: "l1", SupplierID: &supS1, ReorderQty: 10},
		{ProductID: "p2", LocationID: "l1", SupplierID: &supS1, ReorderQty: 5},
		{ProductID: "p3", LocationID: "l1", SupplierID: &supS2, ReorderQty: 20},
		{ProductID: "p4", LocationID: "l2", SupplierID: &supS1, ReorderQty: 7},
	}

	c := constructor.New()
	plans := c.BuildPlans(lines, nil, planDate)
	require.Len(t, plans, 3, "s1/l1, s2/l1, s1/l2")

	for _, p := range plans {
		switch {
		case p.SupplierID == "s1" && p.LocationID == "l1":
			require.InDelta(t, 15.0, p.TotalQty, 0.001)
			require.Equal(t, 2, p.LinesCount)
		case p.SupplierID == "s2" && p.LocationID == "l1":
			require.InDelta(t, 20.0, p.TotalQty, 0.001)
			require.Equal(t, 1, p.LinesCount)
		case p.SupplierID == "s1" && p.LocationID == "l2":
			require.InDelta(t, 7.0, p.TotalQty, 0.001)
		}
		require.Equal(t, "draft", p.Status)
	}
}

func TestConstructor_MOQFiltersSmallPlans(t *testing.T) {
	t.Parallel()
	planDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	sup := "s1"
	lines := []models.CalculationLine{
		{ProductID: "p1", LocationID: "l1", SupplierID: &sup, ReorderQty: 5},
	}
	cfgs := map[string]constructor.SupplierConfig{
		"s1": {MOQ: 10, Multiplier: 1},
	}
	c := constructor.New()
	plans := c.BuildPlans(lines, cfgs, planDate)
	require.Empty(t, plans, "qty=5 < MOQ=10 → skip")
}

func TestConstructor_Multiplier_RoundsUp(t *testing.T) {
	t.Parallel()
	planDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	sup := "s1"
	lines := []models.CalculationLine{
		{ProductID: "p1", LocationID: "l1", SupplierID: &sup, ReorderQty: 13},
	}
	cfgs := map[string]constructor.SupplierConfig{
		"s1": {MOQ: 0, Multiplier: 5}, // round to nearest 5
	}
	c := constructor.New()
	plans := c.BuildPlans(lines, cfgs, planDate)
	require.Len(t, plans, 1)
	require.InDelta(t, 15.0, plans[0].TotalQty, 0.001, "ceil(13/5)*5 = 15")
}

func TestConstructor_SkipsZeroQty(t *testing.T) {
	t.Parallel()
	planDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	sup := "s1"
	lines := []models.CalculationLine{
		{ProductID: "p1", LocationID: "l1", SupplierID: &sup, ReorderQty: 0},
	}
	c := constructor.New()
	plans := c.BuildPlans(lines, nil, planDate)
	require.Empty(t, plans)
}

func TestConstructor_SkipsNullSupplier(t *testing.T) {
	t.Parallel()
	planDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	lines := []models.CalculationLine{
		{ProductID: "p1", LocationID: "l1", SupplierID: nil, ReorderQty: 10},
	}
	c := constructor.New()
	plans := c.BuildPlans(lines, nil, planDate)
	require.Empty(t, plans, "nil supplier_id → skip")
}

func TestConstructor_DeterministicOrder(t *testing.T) {
	t.Parallel()
	planDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	supA, supB := "a", "b"
	lines := []models.CalculationLine{
		{ProductID: "p", LocationID: "l2", SupplierID: &supB, ReorderQty: 1},
		{ProductID: "p", LocationID: "l1", SupplierID: &supA, ReorderQty: 1},
		{ProductID: "p", LocationID: "l2", SupplierID: &supA, ReorderQty: 1},
	}
	c := constructor.New()
	plans := c.BuildPlans(lines, nil, planDate)
	require.Len(t, plans, 3)
	// Ожидаем: a/l1, a/l2, b/l2.
	require.Equal(t, "a", plans[0].SupplierID)
	require.Equal(t, "l1", plans[0].LocationID)
	require.Equal(t, "a", plans[1].SupplierID)
	require.Equal(t, "l2", plans[1].LocationID)
	require.Equal(t, "b", plans[2].SupplierID)
}
