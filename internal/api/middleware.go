package api

import (
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"fanfinity/internal/domain"

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

// ResponseTimeTracker tracks response times for percentile calculation.
// Uses a circular buffer to store recent response times.
type ResponseTimeTracker struct {
	mu       sync.RWMutex
	samples  []float64
	maxSize  int
	position int
}

// Global response time tracker for events API
var eventsResponseTimeTracker = NewResponseTimeTracker(10000)

// NewResponseTimeTracker creates a new tracker with the given maximum sample size.
func NewResponseTimeTracker(maxSize int) *ResponseTimeTracker {
	return &ResponseTimeTracker{
		samples: make([]float64, 0, maxSize),
		maxSize: maxSize,
	}
}

// Record adds a response time sample in milliseconds.
func (t *ResponseTimeTracker) Record(durationMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.samples) < t.maxSize {
		t.samples = append(t.samples, durationMs)
	} else {
		t.samples[t.position] = durationMs
		t.position = (t.position + 1) % t.maxSize
	}
}

// Percentiles calculates p50, p95, and p99 percentiles.
// Returns nil if there are no samples.
func (t *ResponseTimeTracker) Percentiles() *domain.ResponseTimePercentiles {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.samples) == 0 {
		return nil
	}

	// Create a sorted copy
	sorted := make([]float64, len(t.samples))
	copy(sorted, t.samples)
	sort.Float64s(sorted)

	return &domain.ResponseTimePercentiles{
		P50: percentile(sorted, 50),
		P95: percentile(sorted, 95),
		P99: percentile(sorted, 99),
	}
}

// percentile calculates the pth percentile of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	// Calculate the index
	index := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return math.Round(sorted[lower]*100) / 100
	}

	// Linear interpolation
	fraction := index - float64(lower)
	result := sorted[lower] + fraction*(sorted[upper]-sorted[lower])
	return math.Round(result*100) / 100
}

// RecordEventResponseTime records a response time for event ingestion.
func RecordEventResponseTime(duration time.Duration) {
	eventsResponseTimeTracker.Record(float64(duration.Milliseconds()))
}

// GetEventResponseTimePercentiles returns the current response time percentiles.
func GetEventResponseTimePercentiles() *domain.ResponseTimePercentiles {
	return eventsResponseTimeTracker.Percentiles()
}
