package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"fanfinity/internal/domain"
)

// EventProducer defines the interface for producing events to Kafka.
type EventProducer interface {
	Produce(ctx context.Context, event *domain.Event) error
}

// MetricsRepository defines the interface for querying metrics from ClickHouse.
type MetricsRepository interface {
	GetMatchMetrics(ctx context.Context, matchID string) (*domain.MatchMetrics, error)
	GetEventsPerMinute(ctx context.Context, matchID string) ([]domain.EventsPerMinute, error)
	Ping(ctx context.Context) error
}

// Handler handles HTTP requests for the API.
type Handler struct {
	producer   EventProducer
	repository MetricsRepository
}

// NewHandler creates a new Handler with the given producer and repository.
func NewHandler(producer EventProducer, repository MetricsRepository) *Handler {
	return &Handler{
		producer:   producer,
		repository: repository,
	}
}

// IngestEventResponse represents the response for a successful event ingestion.
type IngestEventResponse struct {
	EventID   string    `json:"eventId"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// IngestEvent handles POST /api/events.
// It validates the incoming event, produces it to Kafka, and returns 202 Accepted.
func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse JSON body
	var req domain.EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body", err.Error())
		return
	}

	// Validate and convert to domain Event
	event, err := req.ToEvent()
	if err != nil {
		if ve := domain.AsValidationError(err); ve != nil {
			respondErrorWithField(w, http.StatusBadRequest, ve.Message, ve.Field)
			return
		}
		respondError(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Produce to Kafka
	ctx := r.Context()
	if err := h.producer.Produce(ctx, event); err != nil {
		RecordKafkaProduceError()
		respondError(w, http.StatusServiceUnavailable, "failed to queue event", "")
		return
	}

	// Record metrics
	RecordEventIngested(string(event.EventType))
	RecordEventIngestDuration(time.Since(start))

	// Return 202 Accepted
	response := IngestEventResponse{
		EventID:   event.EventID.String(),
		Status:    "accepted",
		Timestamp: time.Now().UTC(),
	}
	respondJSON(w, http.StatusAccepted, response)
}

// GetMatchMetrics handles GET /api/matches/{matchId}/metrics.
// It queries the repository for match metrics and returns them.
func (h *Handler) GetMatchMetrics(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	if matchID == "" {
		respondError(w, http.StatusBadRequest, "matchId is required", "")
		return
	}

	ctx := r.Context()

	// Get base metrics
	metrics, err := h.repository.GetMatchMetrics(ctx, matchID)
	if err != nil {
		RecordClickHouseQueryError()
		respondError(w, http.StatusInternalServerError, "failed to fetch metrics", "")
		return
	}

	if metrics == nil || metrics.TotalEvents == 0 {
		respondError(w, http.StatusNotFound, "match not found", "")
		return
	}

	// Get events per minute to calculate peak engagement
	eventsPerMinute, err := h.repository.GetEventsPerMinute(ctx, matchID)
	if err != nil {
		RecordClickHouseQueryError()
		// Continue without peak engagement data
		respondJSON(w, http.StatusOK, metrics)
		return
	}

	// Calculate peak engagement
	var peak *domain.PeakEngagement
	if len(eventsPerMinute) > 0 {
		// Aggregate events per minute across all event types
		minuteTotals := make(map[time.Time]int64)
		for _, epm := range eventsPerMinute {
			minuteTotals[epm.Minute] += epm.EventCount
		}

		// Find the peak minute
		var peakMinute time.Time
		var peakCount int64
		for minute, count := range minuteTotals {
			if count > peakCount {
				peakCount = count
				peakMinute = minute
			}
		}

		if peakCount > 0 {
			peak = &domain.PeakEngagement{
				Minute:     peakMinute,
				EventCount: peakCount,
			}
		}
	}

	// Update metrics with peak engagement
	metrics.PeakMinute = peak

	respondJSON(w, http.StatusOK, metrics)
}

// HealthResponse represents the response for health check endpoints.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthCheck handles GET /health.
// It returns a simple health status without checking dependencies.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
	}
	respondJSON(w, http.StatusOK, response)
}

// ReadinessResponse represents the response for readiness check.
type ReadinessResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// ReadinessCheck handles GET /ready.
// It verifies that dependencies (repository) are available.
func (h *Handler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := make(map[string]string)

	// Check repository connectivity
	if err := h.repository.Ping(ctx); err != nil {
		checks["clickhouse"] = "unhealthy: " + err.Error()
		response := ReadinessResponse{
			Status:    "not ready",
			Timestamp: time.Now().UTC(),
			Checks:    checks,
		}
		respondJSON(w, http.StatusServiceUnavailable, response)
		return
	}
	checks["clickhouse"] = "healthy"

	response := ReadinessResponse{
		Status:    "ready",
		Timestamp: time.Now().UTC(),
		Checks:    checks,
	}
	respondJSON(w, http.StatusOK, response)
}
