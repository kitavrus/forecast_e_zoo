package repository_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/sqls/queries"
)

// TestQueriesEmbed_AllFilesExist — smoke: все expected SQL должны быть в embed.
func TestQueriesEmbed_AllFilesExist(t *testing.T) {
	t.Parallel()
	names := []string{
		"current_version",
		"list_marts_versions",
		"select_mart_demand_history",
		"select_mart_calculation_input",
		"select_mart_kpi_daily",
		"select_mart_master_current",
		"select_mart_supplier_scorecard",
	}
	for _, n := range names {
		t.Run(n, func(t *testing.T) {
			t.Parallel()
			s := queries.MustGet(n)
			require.NotEmpty(t, s)
		})
	}
}

// TestQueriesEmbed_BadName_Panics — guard: неизвестное имя → panic.
func TestQueriesEmbed_BadName_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		require.NotNil(t, recover(), "MustGet bad name must panic")
	}()
	_ = queries.MustGet("does_not_exist")
}
