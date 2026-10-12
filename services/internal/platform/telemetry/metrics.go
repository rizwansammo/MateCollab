package telemetry

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns the process Prometheus registry. Labels stay low-cardinality: route patterns,
// never raw paths, user IDs or organisation IDs.
type Metrics struct {
	Registry        *prometheus.Registry
	HTTPRequests    *prometheus.CounterVec
	HTTPDuration    *prometheus.HistogramVec
	WorkerHeartbeat prometheus.Gauge
}

// NewMetrics creates a registry with Go runtime and process collectors.
func NewMetrics(service string) *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	constLabels := prometheus.Labels{"service": service}
	m := &Metrics{
		Registry: reg,
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mc_http_requests_total", Help: "HTTP requests by route, method and status class.", ConstLabels: constLabels,
		}, []string{"route", "method", "status"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "mc_http_request_duration_seconds", Help: "HTTP request latency by route.", ConstLabels: constLabels,
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"route", "method"}),
		WorkerHeartbeat: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mc_worker_heartbeat_timestamp_seconds", Help: "Unix time of the last completed worker heartbeat job.", ConstLabels: constLabels,
		}),
	}
	reg.MustRegister(m.HTTPRequests, m.HTTPDuration, m.WorkerHeartbeat)
	return m
}

// Handler serves the registry in the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{Registry: m.Registry})
}
