package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for HTTP and event processing.
var (
	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Event ingestion metrics
	eventsIngestedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_ingested_total",
			Help: "Total number of events ingested",
		},
		[]string{"event_type"},
	)

	eventIngestDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "event_ingest_duration_seconds",
			Help:    "Event ingestion duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// Error metrics
	kafkaProduceErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_produce_errors_total",
			Help: "Total number of Kafka produce errors",
		},
	)

	clickhouseQueryErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "clickhouse_query_errors_total",
			Help: "Total number of ClickHouse query errors",
		},
	)
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// newResponseWriter creates a new responseWriter with a default 200 status.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code before writing.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write ensures WriteHeader is called before writing the body.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for middleware compatibility.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// PrometheusMiddleware records HTTP request metrics.
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := newResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapped.statusCode)

		// Use the URL path pattern for metrics to avoid high cardinality
		path := r.URL.Path

		httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// RequestLogger returns middleware that logs HTTP requests using structured logging.
func RequestLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newResponseWriter(w)

			// Process request
			next.ServeHTTP(wrapped, r)

			// Log after request completes
			duration := time.Since(start)

			logger.Info("request completed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.Int("status", wrapped.statusCode),
				slog.Duration("duration", duration),
				slog.String("user_agent", r.UserAgent()),
			)
		})
	}
}

// RecordEventIngested increments the event ingestion counter.
func RecordEventIngested(eventType string) {
	eventsIngestedTotal.WithLabelValues(eventType).Inc()
}

// RecordEventIngestDuration records the duration of event ingestion.
func RecordEventIngestDuration(duration time.Duration) {
	eventIngestDuration.Observe(duration.Seconds())
}

// RecordKafkaProduceError increments the Kafka produce error counter.
func RecordKafkaProduceError() {
	kafkaProduceErrorsTotal.Inc()
}

// RecordClickHouseQueryError increments the ClickHouse query error counter.
func RecordClickHouseQueryError() {
	clickhouseQueryErrorsTotal.Inc()
}
