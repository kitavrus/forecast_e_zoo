package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/metrics"
)

func TestNewAndRegistry(t *testing.T) {
	t.Parallel()
	r := metrics.New()
	require.NotNil(t, r)
	require.NotNil(t, r.Registry())
}

func TestRecordRunSuccess(t *testing.T) {
	t.Parallel()
	r := metrics.New()
	r.RecordRunSuccess(3.5)
	assert.Equal(t, float64(1), testutil.ToFloat64(r.RunSuccess))
}

func TestRecordRunFailure(t *testing.T) {
	t.Parallel()
	r := metrics.New()
	r.RecordRunFailure(2.0, "quality")
	r.RecordRunFailure(2.0, "loader")
	assert.Equal(t, float64(1), testutil.ToFloat64(r.RunFailed.WithLabelValues("quality")))
	assert.Equal(t, float64(1), testutil.ToFloat64(r.RunFailed.WithLabelValues("loader")))
}

func TestRecordRowsProcessed(t *testing.T) {
	t.Parallel()
	r := metrics.New()
	r.RecordRowsProcessed("mart_demand_history", 250)
	assert.Equal(t, float64(250), testutil.ToFloat64(r.MartRows.WithLabelValues("mart_demand_history")))
}

func TestIncSkipped(t *testing.T) {
	t.Parallel()
	r := metrics.New()
	r.IncSkipped("already_running")
	r.IncSkipped("already_running")
	r.IncSkipped("snapshot_not_ready")
	assert.Equal(t, float64(2), testutil.ToFloat64(r.Skipped.WithLabelValues("already_running")))
	assert.Equal(t, float64(1), testutil.ToFloat64(r.Skipped.WithLabelValues("snapshot_not_ready")))
}

func TestRegistry_AllMetricsRegistered(t *testing.T) {
	t.Parallel()
	r := metrics.New()
	mfs, err := r.Registry().Gather()
	require.NoError(t, err)
	names := make([]string, 0, len(mfs))
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	required := []string{
		"etl_run_duration_seconds",
		"etl_run_success_total",
		"etl_run_failed_total",
		"etl_lines_processed_total",
		"etl_lines_failed_total",
		"etl_mart_rows_total",
		"etl_lag_seconds",
		"etl_runs_skipped_total",
		"etl_extractor_request_seconds",
		"etl_advisory_lock_held_seconds",
	}
	// Counters / histograms register lazily — exercise each so they appear.
	r.RecordRunSuccess(1)
	r.RecordRunFailure(1, "x")
	r.RecordRowsProcessed("m", 1)
	r.IncSkipped("x")
	r.LinesProcessed.WithLabelValues("e").Inc()
	r.LinesFailed.WithLabelValues("e", "soft").Inc()
	r.LagSeconds.Set(10)
	r.ExtractorRequest.WithLabelValues("e").Observe(0.5)
	r.AdvisoryLockHeld.Observe(5)

	mfs, err = r.Registry().Gather()
	require.NoError(t, err)
	names = names[:0]
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	for _, want := range required {
		assert.True(t, contains(names, want), "missing metric %q in %v", want, strings.Join(names, ","))
	}
}

func contains(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}
