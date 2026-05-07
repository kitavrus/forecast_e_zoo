package engine

import (
	"github.com/Kitavrus/e_zoo/internal/observability"
)

// PrometheusMetrics — реализация Metrics поверх observability.* counters.
//
// Связывает engine.Metrics interface (узкий, без зависимости от Prometheus)
// с конкретными counter'ами в internal/observability.
type PrometheusMetrics struct{}

// NewPrometheusMetrics — конструктор.
func NewPrometheusMetrics() *PrometheusMetrics { return &PrometheusMetrics{} }

// IncRun — kpi_engine_run_total{result}.
func (PrometheusMetrics) IncRun(result string) {
	observability.KpiEngineRunTotal.WithLabelValues(result).Inc()
}

// ObserveDuration — kpi_engine_run_duration_seconds.
func (PrometheusMetrics) ObserveDuration(seconds float64) {
	observability.KpiEngineRunDuration.Observe(seconds)
}

// IncSnapshot — kpi_snapshot_count_total{kpi_name} += count.
func (PrometheusMetrics) IncSnapshot(kpiName string, count int) {
	observability.KpiSnapshotCountTotal.WithLabelValues(kpiName).Add(float64(count))
}

// IncError — kpi_engine_errors_total{kpi_name,reason}.
func (PrometheusMetrics) IncError(kpiName, reason string) {
	observability.KpiEngineErrorsTotal.WithLabelValues(kpiName, reason).Inc()
}

// Compile-time guard: PrometheusMetrics реализует engine.Metrics.
var _ Metrics = PrometheusMetrics{}
