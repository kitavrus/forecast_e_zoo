// Package metrics содержит Prometheus-метрики для ETL pipeline (Module 2).
//
// Все метрики имеют префикс etl_*. Реализуют service.Metrics и
// scheduler.SkipMetrics через единый Recorder.
package metrics

import (
	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// Recorder — единый объект для всех ETL-метрик.
type Recorder struct {
	RunDuration       *prometheus.HistogramVec
	RunSuccess        prometheus.Counter
	RunFailed         *prometheus.CounterVec
	LinesProcessed    *prometheus.CounterVec
	LinesFailed       *prometheus.CounterVec
	MartRows          *prometheus.GaugeVec
	LagSeconds        prometheus.Gauge
	Skipped           *prometheus.CounterVec
	ExtractorRequest  *prometheus.HistogramVec
	AdvisoryLockHeld  prometheus.Histogram
	registry          *prometheus.Registry
}

// New создаёт Recorder и регистрирует метрики в собственном Registry.
func New() *Recorder {
	r := &Recorder{
		RunDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "etl_run_duration_seconds",
			Help:    "ETL run end-to-end duration.",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		}, []string{}),
		RunSuccess: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "etl_run_success_total",
			Help: "Successful ETL run count.",
		}),
		RunFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "etl_run_failed_total",
			Help: "Failed ETL run count by reason.",
		}, []string{"reason"}),
		LinesProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "etl_lines_processed_total",
			Help: "Lines processed by entity.",
		}, []string{"entity"}),
		LinesFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "etl_lines_failed_total",
			Help: "Lines failed validation by entity and severity.",
		}, []string{"entity", "severity"}),
		MartRows: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "etl_mart_rows_total",
			Help: "Last upsert row count per mart.",
		}, []string{"mart"}),
		LagSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "etl_lag_seconds",
			Help: "Seconds since last committed run.",
		}),
		Skipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "etl_runs_skipped_total",
			Help: "Skipped scheduler ticks by reason.",
		}, []string{"reason"}),
		ExtractorRequest: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "etl_extractor_request_seconds",
			Help:    "Extractor request latency by entity.",
			Buckets: prometheus.DefBuckets,
		}, []string{"entity"}),
		AdvisoryLockHeld: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "etl_advisory_lock_held_seconds",
			Help:    "Time advisory lock was held by ETL run.",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		}),
	}

	reg := prometheus.NewRegistry()
	r.registry = reg
	for _, c := range r.collectors() {
		reg.MustRegister(c)
	}
	return r
}

// collectors — единый список для регистрации.
func (r *Recorder) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		r.RunDuration, r.RunSuccess, r.RunFailed,
		r.LinesProcessed, r.LinesFailed, r.MartRows,
		r.LagSeconds, r.Skipped, r.ExtractorRequest,
		r.AdvisoryLockHeld,
	}
}

// Registry возвращает Prometheus registry (для /metrics handler).
func (r *Recorder) Registry() *prometheus.Registry { return r.registry }

// --- service.Metrics ---

// RecordRunSuccess реализует service.Metrics.
func (r *Recorder) RecordRunSuccess(durationSeconds float64) {
	r.RunDuration.WithLabelValues().Observe(durationSeconds)
	r.RunSuccess.Inc()
}

// RecordRunFailure реализует service.Metrics.
func (r *Recorder) RecordRunFailure(durationSeconds float64, reason string) {
	r.RunDuration.WithLabelValues().Observe(durationSeconds)
	r.RunFailed.WithLabelValues(reason).Inc()
}

// RecordRowsProcessed реализует service.Metrics.
func (r *Recorder) RecordRowsProcessed(mart string, rows int64) {
	r.MartRows.WithLabelValues(mart).Set(float64(rows))
}

// --- scheduler.SkipMetrics ---

// IncSkipped реализует scheduler.SkipMetrics.
func (r *Recorder) IncSkipped(reason string) {
	r.Skipped.WithLabelValues(reason).Inc()
}

// --- HTTP handler ---

// Handler возвращает Fiber-handler /metrics поверх registry.
func (r *Recorder) Handler() fiber.Handler {
	h := promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{Registry: r.registry})
	netH := fasthttpadaptor.NewFastHTTPHandler(h)
	return func(c fiber.Ctx) error {
		netH(c.RequestCtx())
		return nil
	}
}
