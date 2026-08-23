package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics records HTTP metrics into a Prometheus registry.
type PrometheusMetrics struct {
	registry            prometheus.Registerer
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
}

// NewPrometheusMetrics builds a collector registered with the given registry.
//
// Pass nil to use Prometheus' default registry, which is what a single-process
// application wants. Pass an explicit registry — prometheus.NewRegistry() — when
// more than one instance may exist in the same process, most notably in tests:
// registering the same collector twice in one registry is an error.
func NewPrometheusMetrics(registry prometheus.Registerer) (MetricsService, error) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	for _, collector := range []prometheus.Collector{requests, duration} {
		if err := registry.Register(collector); err != nil {
			return nil, fmt.Errorf("register http metrics: %w", err)
		}
	}

	return &PrometheusMetrics{
		registry:            registry,
		httpRequestsTotal:   requests,
		httpRequestDuration: duration,
	}, nil
}

// RecordHTTPRequest counts one request.
func (p *PrometheusMetrics) RecordHTTPRequest(method, path, status string) {
	p.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
}

// RecordHTTPDuration observes how long one request took.
func (p *PrometheusMetrics) RecordHTTPDuration(method, path, status string, duration time.Duration) {
	p.httpRequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())
}
