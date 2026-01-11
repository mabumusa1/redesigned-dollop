package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter creates and configures a new chi router with all routes and middleware.
func NewRouter(producer EventProducer, repository MetricsRepository, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Apply middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(RequestLogger(logger))
	r.Use(PrometheusMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Create handler
	h := NewHandler(producer, repository)

	// Health check endpoints (outside /api prefix)
	r.Get("/health", h.HealthCheck)
	r.Get("/ready", h.ReadinessCheck)

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Event ingestion
		r.Post("/events", h.IngestEvent)

		// Match metrics
		r.Get("/matches/{matchId}/metrics", h.GetMatchMetrics)
	})

	return r
}

// NewServer creates a new HTTP server with the configured router.
func NewServer(addr string, producer EventProducer, repository MetricsRepository, logger *slog.Logger) *http.Server {
	router := NewRouter(producer, repository, logger)

	return &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
