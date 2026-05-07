package queries

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// expectedQueries — whitelist всех SQL-файлов фазы 6 (sync с code-plan).
var expectedQueries = []string{
	// master selects
	"select_products",
	"select_product_barcodes",
	"select_category",
	"select_location",
	"select_supplier",
	"select_supply_spec",
	"select_promo",
	"select_order_rule",
	"select_supply_plan",
	"select_store_assortment",
	"select_store_assortment_lifecycle_events",
	"select_master_change_log",
	// facts selects
	"select_receipt_line",
	"select_location_stock_snapshot",
	"select_stock_movement",
	"select_supplier_stock_snapshot",
	// loads
	"loads_insert_running",
	"loads_mark_committed",
	"loads_mark_failed",
	"loads_mark_aborted",
	"loads_get_by_id",
	"loads_select_running",
	// snapshot
	"snapshot_select_current",
	"snapshot_flip",
	"snapshot_seed",
	// advisory locks
	"advisory_lock_try",
	"advisory_unlock",
	// reject_log
	"reject_log_insert",
	"reject_log_select",
	// audit_access
	"audit_access_insert",
	"audit_access_select",
	// partitions
	"partitions_create_month",
}

func TestEmbed_AllExpectedFilesPresent(t *testing.T) {
	t.Parallel()
	for _, name := range expectedQueries {
		s, err := getOrError(name)
		require.NoErrorf(t, err, "query %s should be embedded", name)
		require.NotEmptyf(t, strings.TrimSpace(s), "query %s should be non-empty", name)
	}
}

func TestEmbed_GetReturnsContent(t *testing.T) {
	t.Parallel()
	s := Get("snapshot_select_current")
	require.NotEmpty(t, s)
	require.Contains(t, s, "snapshot_pointer")
}

func TestEmbed_GetUnknown_Behavior(t *testing.T) {
	t.Parallel()
	_, err := getOrError("totally_made_up_name")
	require.Error(t, err)

	// Public Get panics — проверим recover.
	defer func() {
		r := recover()
		require.NotNil(t, r)
	}()
	_ = Get("totally_made_up_name")
}

func TestEmbed_GetRejectsBadName(t *testing.T) {
	t.Parallel()
	_, err := getOrError("../etc/passwd")
	require.Error(t, err)
}

// TestEmbed_FactsHaveEventDateFilter — guard partitioning pruning:
// все select_-запросы фактов должны содержать "event_date" в WHERE.
func TestEmbed_FactsHaveEventDateFilter(t *testing.T) {
	t.Parallel()
	facts := []string{
		"select_receipt_line",
		"select_location_stock_snapshot",
		"select_stock_movement",
		"select_supplier_stock_snapshot",
	}
	for _, name := range facts {
		s := Get(name)
		require.Containsf(t, s, "event_date", "%s must filter by event_date for partition pruning", name)
	}
}
