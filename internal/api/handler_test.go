package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"fanfinity/internal/api"
	"fanfinity/internal/domain"
)

// MockProducer implements api.EventProducer for testing.
type MockProducer struct {
	ProduceFunc func(ctx context.Context, event *domain.Event) error
}

func (m *MockProducer) Produce(ctx context.Context, event *domain.Event) error {
	if m.ProduceFunc != nil {
		return m.ProduceFunc(ctx, event)
	}
	return nil
}

// MockRepository implements api.MetricsRepository for testing.
type MockRepository struct {
	GetMatchMetricsFunc    func(ctx context.Context, matchID string) (*domain.MatchMetrics, error)
	GetEventsPerMinuteFunc func(ctx context.Context, matchID string) ([]domain.EventsPerMinute, error)
	PingFunc               func(ctx context.Context) error
}

func (m *MockRepository) GetMatchMetrics(ctx context.Context, matchID string) (*domain.MatchMetrics, error) {
	if m.GetMatchMetricsFunc != nil {
		return m.GetMatchMetricsFunc(ctx, matchID)
	}
	return nil, nil
}

func (m *MockRepository) GetEventsPerMinute(ctx context.Context, matchID string) ([]domain.EventsPerMinute, error) {
	if m.GetEventsPerMinuteFunc != nil {
		return m.GetEventsPerMinuteFunc(ctx, matchID)
	}
	return nil, nil
}

func (m *MockRepository) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

// Helper to create a valid event request JSON
func validEventJSON() []byte {
	req := map[string]interface{}{
		"eventId":   uuid.New().String(),
		"matchId":   "match-123",
		"eventType": "goal",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"teamId":    1,
		"playerId":  "player-456",
		"metadata": map[string]interface{}{
			"minute": 45,
		},
	}
	data, _ := json.Marshal(req)
	return data
}

// Helper to create a chi router context with URL params
func withChiURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ====================
// IngestEvent Tests
// ====================

func TestIngestEvent_Success(t *testing.T) {
	var capturedEvent *domain.Event

	mockProducer := &MockProducer{
		ProduceFunc: func(ctx context.Context, event *domain.Event) error {
			capturedEvent = event
			return nil
		},
	}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	body := validEventJSON()
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 202 status
	if rr.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, rr.Code)
	}

	// Parse response
	var resp api.IngestEventResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Assert response contains eventId and status: accepted
	if resp.EventID == "" {
		t.Error("expected eventId in response, got empty string")
	}
	if resp.Status != "accepted" {
		t.Errorf("expected status 'accepted', got '%s'", resp.Status)
	}

	// Verify the producer was called with the event
	if capturedEvent == nil {
		t.Error("expected producer to receive event, got nil")
	}
}

func TestIngestEvent_InvalidJSON(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 400 status
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Verify error response
	var errResp api.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected error field in response")
	}
}

func TestIngestEvent_ValidationError_InvalidUUID(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	// Create event with invalid UUID
	eventReq := map[string]interface{}{
		"eventId":   "bad-uuid",
		"matchId":   "match-123",
		"eventType": "goal",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"teamId":    1,
	}
	body, _ := json.Marshal(eventReq)

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 400 status
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Verify error response mentions the field
	var errResp api.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Field != "eventId" {
		t.Errorf("expected field 'eventId' in error, got '%s'", errResp.Field)
	}
}

func TestIngestEvent_ValidationError_InvalidEventType(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	// Create event with invalid event type
	eventReq := map[string]interface{}{
		"eventId":   uuid.New().String(),
		"matchId":   "match-123",
		"eventType": "unknown",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"teamId":    1,
	}
	body, _ := json.Marshal(eventReq)

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 400 status
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Verify error response mentions the field
	var errResp api.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Field != "eventType" {
		t.Errorf("expected field 'eventType' in error, got '%s'", errResp.Field)
	}
}

func TestIngestEvent_ValidationError_InvalidTimestamp(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	eventReq := map[string]interface{}{
		"eventId":   uuid.New().String(),
		"matchId":   "match-123",
		"eventType": "goal",
		"timestamp": "invalid-timestamp",
		"teamId":    1,
	}
	body, _ := json.Marshal(eventReq)

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 400 status
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestIngestEvent_ValidationError_EmptyMatchID(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	eventReq := map[string]interface{}{
		"eventId":   uuid.New().String(),
		"matchId":   "",
		"eventType": "goal",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"teamId":    1,
	}
	body, _ := json.Marshal(eventReq)

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 400 status
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var errResp api.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Field != "matchId" {
		t.Errorf("expected field 'matchId' in error, got '%s'", errResp.Field)
	}
}

func TestIngestEvent_ValidationError_InvalidTeamID(t *testing.T) {
	testCases := []struct {
		name   string
		teamID int
	}{
		{"zero", 0},
		{"three", 3},
		{"negative", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockProducer := &MockProducer{}
			mockRepo := &MockRepository{}

			handler := api.NewHandler(mockProducer, mockRepo)

			eventReq := map[string]interface{}{
				"eventId":   uuid.New().String(),
				"matchId":   "match-123",
				"eventType": "goal",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"teamId":    tc.teamID,
			}
			body, _ := json.Marshal(eventReq)

			req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.IngestEvent(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status %d for teamId=%d, got %d", http.StatusBadRequest, tc.teamID, rr.Code)
			}
		})
	}
}

func TestIngestEvent_KafkaError(t *testing.T) {
	mockProducer := &MockProducer{
		ProduceFunc: func(ctx context.Context, event *domain.Event) error {
			return errors.New("kafka connection failed")
		},
	}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	body := validEventJSON()
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 503 status
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	// Verify error response
	var errResp api.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Message == "" {
		t.Error("expected error message in response")
	}
}

func TestIngestEvent_EmptyBody(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	// Assert 400 status for empty body
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// ====================
// GetMatchMetrics Tests
// ====================

func TestGetMatchMetrics_Success(t *testing.T) {
	now := time.Now().UTC()
	expectedMetrics := &domain.MatchMetrics{
		MatchID:     "match-123",
		TotalEvents: 100,
		EventsByType: map[string]int64{
			"goal":        2,
			"shot":        15,
			"pass":        70,
			"yellow_card": 3,
		},
		Goals:        2,
		YellowCards:  3,
		RedCards:     0,
		FirstEventAt: &now,
		LastEventAt:  &now,
	}

	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		GetMatchMetricsFunc: func(ctx context.Context, matchID string) (*domain.MatchMetrics, error) {
			if matchID == "match-123" {
				return expectedMetrics, nil
			}
			return nil, nil
		},
		GetEventsPerMinuteFunc: func(ctx context.Context, matchID string) ([]domain.EventsPerMinute, error) {
			return []domain.EventsPerMinute{
				{Minute: now, EventType: "goal", EventCount: 1},
				{Minute: now, EventType: "shot", EventCount: 5},
			}, nil
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/matches/match-123/metrics", nil)
	req = withChiURLParams(req, map[string]string{"matchId": "match-123"})

	rr := httptest.NewRecorder()
	handler.GetMatchMetrics(rr, req)

	// Assert 200 status
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var metrics domain.MatchMetrics
	if err := json.NewDecoder(rr.Body).Decode(&metrics); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Assert JSON contains expected fields
	if metrics.MatchID != "match-123" {
		t.Errorf("expected matchId 'match-123', got '%s'", metrics.MatchID)
	}
	if metrics.TotalEvents != 100 {
		t.Errorf("expected totalEvents 100, got %d", metrics.TotalEvents)
	}
	if metrics.Goals != 2 {
		t.Errorf("expected goals 2, got %d", metrics.Goals)
	}
}

func TestGetMatchMetrics_NotFound(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		GetMatchMetricsFunc: func(ctx context.Context, matchID string) (*domain.MatchMetrics, error) {
			return nil, nil // Return nil for not found
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/matches/nonexistent/metrics", nil)
	req = withChiURLParams(req, map[string]string{"matchId": "nonexistent"})

	rr := httptest.NewRecorder()
	handler.GetMatchMetrics(rr, req)

	// Assert 404 status
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGetMatchMetrics_EmptyMatchID(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/matches//metrics", nil)
	req = withChiURLParams(req, map[string]string{"matchId": ""})

	rr := httptest.NewRecorder()
	handler.GetMatchMetrics(rr, req)

	// Assert 400 status
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGetMatchMetrics_RepositoryError(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		GetMatchMetricsFunc: func(ctx context.Context, matchID string) (*domain.MatchMetrics, error) {
			return nil, errors.New("database error")
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/matches/match-123/metrics", nil)
	req = withChiURLParams(req, map[string]string{"matchId": "match-123"})

	rr := httptest.NewRecorder()
	handler.GetMatchMetrics(rr, req)

	// Assert 500 status
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestGetMatchMetrics_ZeroTotalEvents(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		GetMatchMetricsFunc: func(ctx context.Context, matchID string) (*domain.MatchMetrics, error) {
			return &domain.MatchMetrics{
				MatchID:      "match-123",
				TotalEvents:  0,
				EventsByType: make(map[string]int64),
			}, nil
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/matches/match-123/metrics", nil)
	req = withChiURLParams(req, map[string]string{"matchId": "match-123"})

	rr := httptest.NewRecorder()
	handler.GetMatchMetrics(rr, req)

	// Should return 404 when total events is 0
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d for zero events, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestGetMatchMetrics_WithPeakEngagement(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)

	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		GetMatchMetricsFunc: func(ctx context.Context, matchID string) (*domain.MatchMetrics, error) {
			return &domain.MatchMetrics{
				MatchID:      "match-123",
				TotalEvents:  50,
				EventsByType: map[string]int64{"goal": 2, "shot": 10},
				Goals:        2,
			}, nil
		},
		GetEventsPerMinuteFunc: func(ctx context.Context, matchID string) ([]domain.EventsPerMinute, error) {
			// Return events per minute data
			return []domain.EventsPerMinute{
				{Minute: now, EventType: "goal", EventCount: 2},
				{Minute: now, EventType: "shot", EventCount: 8},
				{Minute: now.Add(time.Minute), EventType: "shot", EventCount: 3},
			}, nil
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/matches/match-123/metrics", nil)
	req = withChiURLParams(req, map[string]string{"matchId": "match-123"})

	rr := httptest.NewRecorder()
	handler.GetMatchMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var metrics domain.MatchMetrics
	if err := json.NewDecoder(rr.Body).Decode(&metrics); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify peak engagement is calculated
	if metrics.PeakMinute == nil {
		t.Error("expected peakMinute to be set")
	} else {
		// First minute has 10 events (2 goal + 8 shot), second has 3
		if metrics.PeakMinute.EventCount != 10 {
			t.Errorf("expected peak event count 10, got %d", metrics.PeakMinute.EventCount)
		}
	}
}

// ====================
// HealthCheck Tests
// ====================

func TestHealthCheck_Success(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.HealthCheck(rr, req)

	// Assert 200 status
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var resp api.HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Assert response contains status: healthy
	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", resp.Status)
	}
}

func TestHealthCheck_HasTimestamp(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.HealthCheck(rr, req)

	var resp api.HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify timestamp is present and recent
	if resp.Timestamp.IsZero() {
		t.Error("expected timestamp in health response")
	}
}

// ====================
// ReadinessCheck Tests
// ====================

func TestReadinessCheck_Success(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		PingFunc: func(ctx context.Context) error {
			return nil // Healthy
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	handler.ReadinessCheck(rr, req)

	// Assert 200 status
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var resp api.ReadinessResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Assert status is ready
	if resp.Status != "ready" {
		t.Errorf("expected status 'ready', got '%s'", resp.Status)
	}

	// Assert checks contain clickhouse status
	if resp.Checks == nil {
		t.Error("expected checks in readiness response")
	} else if resp.Checks["clickhouse"] != "healthy" {
		t.Errorf("expected clickhouse check 'healthy', got '%s'", resp.Checks["clickhouse"])
	}
}

func TestReadinessCheck_Unhealthy(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		PingFunc: func(ctx context.Context) error {
			return errors.New("connection refused")
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	handler.ReadinessCheck(rr, req)

	// Assert 503 status
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	// Parse response
	var resp api.ReadinessResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Assert status is not ready
	if resp.Status != "not ready" {
		t.Errorf("expected status 'not ready', got '%s'", resp.Status)
	}

	// Assert checks contain unhealthy clickhouse status
	if resp.Checks == nil {
		t.Error("expected checks in readiness response")
	} else if resp.Checks["clickhouse"] == "healthy" {
		t.Error("expected clickhouse check to be unhealthy")
	}
}

func TestReadinessCheck_HasTimestamp(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		PingFunc: func(ctx context.Context) error {
			return nil
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	handler.ReadinessCheck(rr, req)

	var resp api.ReadinessResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Timestamp.IsZero() {
		t.Error("expected timestamp in readiness response")
	}
}

// ====================
// Table-Driven Tests for Validation
// ====================

func TestIngestEvent_ValidationErrors_TableDriven(t *testing.T) {
	testCases := []struct {
		name          string
		eventRequest  map[string]interface{}
		expectedField string
	}{
		{
			name: "invalid uuid",
			eventRequest: map[string]interface{}{
				"eventId":   "not-a-uuid",
				"matchId":   "match-123",
				"eventType": "goal",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"teamId":    1,
			},
			expectedField: "eventId",
		},
		{
			name: "empty matchId",
			eventRequest: map[string]interface{}{
				"eventId":   uuid.New().String(),
				"matchId":   "",
				"eventType": "goal",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"teamId":    1,
			},
			expectedField: "matchId",
		},
		{
			name: "invalid eventType",
			eventRequest: map[string]interface{}{
				"eventId":   uuid.New().String(),
				"matchId":   "match-123",
				"eventType": "invalid_type",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"teamId":    1,
			},
			expectedField: "eventType",
		},
		{
			name: "invalid timestamp",
			eventRequest: map[string]interface{}{
				"eventId":   uuid.New().String(),
				"matchId":   "match-123",
				"eventType": "goal",
				"timestamp": "not-a-timestamp",
				"teamId":    1,
			},
			expectedField: "timestamp",
		},
		{
			name: "teamId zero",
			eventRequest: map[string]interface{}{
				"eventId":   uuid.New().String(),
				"matchId":   "match-123",
				"eventType": "goal",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"teamId":    0,
			},
			expectedField: "teamId",
		},
		{
			name: "teamId three",
			eventRequest: map[string]interface{}{
				"eventId":   uuid.New().String(),
				"matchId":   "match-123",
				"eventType": "goal",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"teamId":    3,
			},
			expectedField: "teamId",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockProducer := &MockProducer{}
			mockRepo := &MockRepository{}
			handler := api.NewHandler(mockProducer, mockRepo)

			body, _ := json.Marshal(tc.eventRequest)
			req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.IngestEvent(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
			}

			var errResp api.ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Field != tc.expectedField {
				t.Errorf("expected field '%s', got '%s'", tc.expectedField, errResp.Field)
			}
		})
	}
}

// ====================
// Response Content-Type Tests
// ====================

func TestIngestEvent_ResponseContentType(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	body := validEventJSON()
	req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.IngestEvent(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestHealthCheck_ResponseContentType(t *testing.T) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.HealthCheck(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

// ====================
// Benchmark Tests
// ====================

func BenchmarkIngestEvent_Success(b *testing.B) {
	mockProducer := &MockProducer{
		ProduceFunc: func(ctx context.Context, event *domain.Event) error {
			return nil
		},
	}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)
	body := validEventJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.IngestEvent(rr, req)
	}
}

func BenchmarkHealthCheck(b *testing.B) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{}

	handler := api.NewHandler(mockProducer, mockRepo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		handler.HealthCheck(rr, req)
	}
}

func BenchmarkReadinessCheck(b *testing.B) {
	mockProducer := &MockProducer{}
	mockRepo := &MockRepository{
		PingFunc: func(ctx context.Context) error {
			return nil
		},
	}

	handler := api.NewHandler(mockProducer, mockRepo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()
		handler.ReadinessCheck(rr, req)
	}
}
