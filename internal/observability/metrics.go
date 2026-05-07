// Package observability содержит Prometheus-метрики и slog-middleware
// для source-adapter.
package observability

import (
	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// Метрики бизнес-процесса и HTTP. Все начинаются с source_adapter_.
var (
	LoadSuccessTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "source_adapter_load_success_total",
		Help: "Successful loads count.",
	}, []string{"source"})

	LoadFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "source_adapter_load_failed_total",
		Help: "Failed loads count, partitioned by reason.",
	}, []string{"source", "reason"})

	SnapshotNotReadyTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "source_adapter_snapshot_not_ready_total",
		Help: "Times /v1/snapshots/current returned 503.",
	})

	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "source_adapter_http_requests_total",
		Help: "HTTP requests count.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "source_adapter_http_request_duration_seconds",
		Help:    "HTTP request duration.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	LoadDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "source_adapter_load_duration_seconds",
		Help:    "End-to-end load duration.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"source"})

	LinesProcessedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "source_adapter_lines_processed_total",
		Help: "Lines processed by entity.",
	}, []string{"entity"})

	LinesFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "source_adapter_lines_failed_total",
		Help: "Lines that failed validation by entity.",
	}, []string{"entity", "severity"})

	AdvisoryLockBusyTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "source_adapter_advisory_lock_busy_total",
		Help: "Times scheduler tick skipped because advisory lock was busy.",
	})

	SchedulerTickTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "source_adapter_scheduler_tick_total",
		Help: "Scheduler tick outcomes.",
	}, []string{"result"}) // result = ok|skipped|error

	// --- KPI Engine (Module 4 kpi-calibration) ---

	KpiEngineRunTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kpi_engine_run_total",
		Help: "KPI engine run outcomes.",
	}, []string{"result"}) // result = ok|partial|error|skipped

	KpiEngineRunDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "kpi_engine_run_duration_seconds",
		Help:    "KPI engine run end-to-end duration.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
	})

	KpiSnapshotCountTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kpi_snapshot_count_total",
		Help: "Snapshots written by KPI engine, by kpi_name.",
	}, []string{"kpi_name"})

	KpiEngineErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kpi_engine_errors_total",
		Help: "KPI engine errors by kpi_name and reason.",
	}, []string{"kpi_name", "reason"})

	// --- Forecast Engine (Module 5 forecast-engine) ---

	ForecastEngineRunTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forecast_engine_run_total",
		Help: "Forecast engine run outcomes.",
	}, []string{"result"}) // result = ok|error|skipped

	ForecastEngineRunDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "forecast_engine_run_duration_seconds",
		Help:    "Forecast engine run end-to-end duration.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
	})

	ForecastForecastsCountTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "forecast_forecasts_count_total",
		Help: "Forecast rows written by forecast engine.",
	})

	ForecastLinesCountTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "forecast_lines_count_total",
		Help: "Calculation lines written by forecast engine.",
	})

	ForecastPlansCountTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "forecast_plans_count_total",
		Help: "Replenishment plans written by forecast engine.",
	})

	ForecastEngineErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forecast_engine_errors_total",
		Help: "Forecast engine errors by reason.",
	}, []string{"reason"})

	// --- Order Builder (Module 6 order-builder) ---

	OrderBuilderRunTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "order_builder_run_total",
		Help: "Order builder run outcomes.",
	}, []string{"result"}) // result = ok|error|skipped

	OrderBuilderRunDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "order_builder_run_duration_seconds",
		Help:    "Order builder run end-to-end duration.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
	})

	OrderBuilderPOsCreatedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "order_builder_pos_created_total",
		Help: "Purchase orders created by order builder.",
	})

	OrderBuilderPlansProcessedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "order_builder_plans_processed_total",
		Help: "Approved plans processed by order builder.",
	})

	OrderBuilderErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "order_builder_errors_total",
		Help: "Order builder errors by reason.",
	}, []string{"reason"})

	// --- Channel Routing (Module 7 channel-routing) ---

	ChannelSendTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "channel_send_total",
		Help: "Channel send-attempt outcomes by channel and status.",
	}, []string{"channel", "status"}) // status = pending|success|failed|skipped

	ChannelSendDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "channel_send_duration_seconds",
		Help:    "Channel send-attempt end-to-end duration.",
		Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120, 300},
	}, []string{"channel"})

	ChannelRetryCountTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "channel_retry_count_total",
		Help: "Total retries inside Send (per channel).",
	}, []string{"channel"})

	ChannelIdempotentHitTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "channel_idempotent_hit_total",
		Help: "Times retry resolved to existing successful attempt without new send.",
	}, []string{"channel"})
)

// allMetrics — единый список для регистрации/тестов.
func allMetrics() []prometheus.Collector {
	return []prometheus.Collector{
		LoadSuccessTotal, LoadFailedTotal, SnapshotNotReadyTotal,
		HTTPRequestsTotal, HTTPRequestDuration, LoadDuration,
		LinesProcessedTotal, LinesFailedTotal, AdvisoryLockBusyTotal, SchedulerTickTotal,
		KpiEngineRunTotal, KpiEngineRunDuration, KpiSnapshotCountTotal, KpiEngineErrorsTotal,
		ForecastEngineRunTotal, ForecastEngineRunDuration,
		ForecastForecastsCountTotal, ForecastLinesCountTotal, ForecastPlansCountTotal,
		ForecastEngineErrorsTotal,
		OrderBuilderRunTotal, OrderBuilderRunDuration,
		OrderBuilderPOsCreatedTotal, OrderBuilderPlansProcessedTotal,
		OrderBuilderErrorsTotal,
		ChannelSendTotal, ChannelSendDuration,
		ChannelRetryCountTotal, ChannelIdempotentHitTotal,
	}
}

// Init создаёт новый Registry и регистрирует все метрики.
func Init() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	for _, c := range allMetrics() {
		reg.MustRegister(c)
	}
	return reg
}

// Handler возвращает Fiber-handler /metrics поверх Prometheus registry.
func Handler(reg *prometheus.Registry) fiber.Handler {
	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
	netH := fasthttpadaptor.NewFastHTTPHandler(h)
	return func(c fiber.Ctx) error {
		netH(c.RequestCtx())
		return nil
	}
}
