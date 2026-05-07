package engine

import (
	"github.com/Kitavrus/e_zoo/internal/observability"
)

// PrometheusMetrics — реализация Metrics поверх observability.* counters.
type PrometheusMetrics struct{}

// NewPrometheusMetrics — конструктор.
func NewPrometheusMetrics() *PrometheusMetrics { return &PrometheusMetrics{} }

// IncRun — forecast_engine_run_total{result}.
func (PrometheusMetrics) IncRun(result string) {
	observability.ForecastEngineRunTotal.WithLabelValues(result).Inc()
}

// ObserveDuration — forecast_engine_run_duration_seconds.
func (PrometheusMetrics) ObserveDuration(seconds float64) {
	observability.ForecastEngineRunDuration.Observe(seconds)
}

// IncForecasts — forecast_forecasts_count_total += count.
func (PrometheusMetrics) IncForecasts(count int) {
	observability.ForecastForecastsCountTotal.Add(float64(count))
}

// IncLines — forecast_lines_count_total += count.
func (PrometheusMetrics) IncLines(count int) {
	observability.ForecastLinesCountTotal.Add(float64(count))
}

// IncPlans — forecast_plans_count_total += count.
func (PrometheusMetrics) IncPlans(count int) {
	observability.ForecastPlansCountTotal.Add(float64(count))
}

// IncError — forecast_engine_errors_total{reason}.
func (PrometheusMetrics) IncError(reason string) {
	observability.ForecastEngineErrorsTotal.WithLabelValues(reason).Inc()
}

// Compile-time guard.
var _ Metrics = PrometheusMetrics{}
