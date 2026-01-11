package domain_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"fanfinity/internal/domain"
)

// TestEventRequest_ToEvent_Valid tests successful conversion of a valid EventRequest.
func TestEventRequest_ToEvent_Valid(t *testing.T) {
	validUUID := uuid.New().String()
	validTimestamp := time.Now().UTC().Format(time.RFC3339)

	req := &domain.EventRequest{
		EventID:   validUUID,
		MatchID:   "match-123",
		EventType: "goal",
		Timestamp: validTimestamp,
		TeamID:    1,
		PlayerID:  "player-456",
		Metadata: map[string]interface{}{
			"minute":   45,
			"position": "penalty",
		},
	}

	event, err := req.ToEvent()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Assert EventID is a valid UUID
	if event.EventID.String() != validUUID {
		t.Errorf("expected EventID %s, got %s", validUUID, event.EventID.String())
	}

	// Assert all fields are correctly converted
	if event.MatchID != "match-123" {
		t.Errorf("expected MatchID 'match-123', got '%s'", event.MatchID)
	}

	if event.EventType != domain.EventTypeGoal {
		t.Errorf("expected EventType 'goal', got '%s'", event.EventType)
	}

	if event.TeamID != 1 {
		t.Errorf("expected TeamID 1, got %d", event.TeamID)
	}

	if event.PlayerID != "player-456" {
		t.Errorf("expected PlayerID 'player-456', got '%s'", event.PlayerID)
	}

	// Assert Timestamp is correctly parsed
	expectedTime, _ := time.Parse(time.RFC3339, validTimestamp)
	if !event.Timestamp.Equal(expectedTime) {
		t.Errorf("expected Timestamp %v, got %v", expectedTime, event.Timestamp)
	}

	// Assert metadata is preserved
	if event.Metadata == nil {
		t.Fatal("expected Metadata to be non-nil")
	}
	// When metadata is passed directly (not from JSON), integers stay as int
	if event.Metadata["minute"] != 45 {
		t.Errorf("expected Metadata['minute'] = 45, got %v (type: %T)", event.Metadata["minute"], event.Metadata["minute"])
	}
	if event.Metadata["position"] != "penalty" {
		t.Errorf("expected Metadata['position'] = 'penalty', got %v", event.Metadata["position"])
	}
}

// TestEventRequest_ToEvent_AllEventTypes tests all valid event types.
func TestEventRequest_ToEvent_AllEventTypes(t *testing.T) {
	validEventTypes := []struct {
		input    string
		expected domain.EventType
	}{
		{"pass", domain.EventTypePass},
		{"shot", domain.EventTypeShot},
		{"goal", domain.EventTypeGoal},
		{"foul", domain.EventTypeFoul},
		{"yellow_card", domain.EventTypeYellowCard},
		{"red_card", domain.EventTypeRedCard},
		{"substitution", domain.EventTypeSubstitution},
		{"offside", domain.EventTypeOffside},
		{"corner", domain.EventTypeCorner},
		{"free_kick", domain.EventTypeFreeKick},
		{"interception", domain.EventTypeInterception},
	}

	for _, tc := range validEventTypes {
		t.Run(tc.input, func(t *testing.T) {
			req := &domain.EventRequest{
				EventID:   uuid.New().String(),
				MatchID:   "match-123",
				EventType: tc.input,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				TeamID:    1,
			}

			event, err := req.ToEvent()
			if err != nil {
				t.Fatalf("expected no error for event type '%s', got: %v", tc.input, err)
			}
			if event.EventType != tc.expected {
				t.Errorf("expected EventType '%s', got '%s'", tc.expected, event.EventType)
			}
		})
	}
}

// TestEventRequest_ToEvent_InvalidUUID tests that invalid UUIDs are rejected.
func TestEventRequest_ToEvent_InvalidUUID(t *testing.T) {
	testCases := []struct {
		name    string
		eventID string
	}{
		{"not a uuid", "not-a-uuid"},
		{"empty string", ""},
		{"partial uuid", "550e8400-e29b-41d4"},
		{"invalid characters", "550e8400-e29b-41d4-a716-zzzzzzzzzzzz"},
		{"too short", "550e8400"},
		{"spaces", "550e8400 e29b 41d4 a716 446655440000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &domain.EventRequest{
				EventID:   tc.eventID,
				MatchID:   "match-123",
				EventType: "goal",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				TeamID:    1,
			}

			_, err := req.ToEvent()
			if err == nil {
				t.Fatal("expected error for invalid UUID, got nil")
			}

			// Assert returns ValidationError
			if !domain.IsValidationError(err) {
				t.Errorf("expected ValidationError, got: %T", err)
			}

			// Assert error mentions "eventId"
			ve := domain.AsValidationError(err)
			if ve == nil {
				t.Fatal("expected ValidationError")
			}
			if ve.Field != "eventId" {
				t.Errorf("expected field 'eventId', got '%s'", ve.Field)
			}
		})
	}
}

// TestEventRequest_ToEvent_InvalidTimestamp tests that invalid timestamps are rejected.
func TestEventRequest_ToEvent_InvalidTimestamp(t *testing.T) {
	testCases := []struct {
		name      string
		timestamp string
	}{
		{"invalid-date", "invalid-date"},
		{"empty string", ""},
		{"unix timestamp", "1609459200"},
		{"date only", "2021-01-01"},
		{"wrong format", "01/01/2021 12:00:00"},
		{"missing timezone", "2021-01-01T12:00:00"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &domain.EventRequest{
				EventID:   uuid.New().String(),
				MatchID:   "match-123",
				EventType: "goal",
				Timestamp: tc.timestamp,
				TeamID:    1,
			}

			_, err := req.ToEvent()
			if err == nil {
				t.Fatal("expected error for invalid timestamp, got nil")
			}

			// Assert returns ValidationError
			if !domain.IsValidationError(err) {
				t.Errorf("expected ValidationError, got: %T", err)
			}

			// Assert error mentions "timestamp"
			ve := domain.AsValidationError(err)
			if ve == nil {
				t.Fatal("expected ValidationError")
			}
			if ve.Field != "timestamp" {
				t.Errorf("expected field 'timestamp', got '%s'", ve.Field)
			}
		})
	}
}

// TestEventRequest_ToEvent_InvalidEventType tests that invalid event types are rejected.
func TestEventRequest_ToEvent_InvalidEventType(t *testing.T) {
	testCases := []struct {
		name      string
		eventType string
	}{
		{"invalid_type", "invalid_type"},
		{"empty string", ""},
		{"uppercase", "GOAL"},
		{"mixed case", "Goal"},
		{"unknown type", "unknown"},
		{"with spaces", "free kick"},
		{"typo", "goall"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &domain.EventRequest{
				EventID:   uuid.New().String(),
				MatchID:   "match-123",
				EventType: tc.eventType,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				TeamID:    1,
			}

			_, err := req.ToEvent()
			if err == nil {
				t.Fatalf("expected error for invalid event type '%s', got nil", tc.eventType)
			}

			// Assert returns ValidationError
			if !domain.IsValidationError(err) {
				t.Errorf("expected ValidationError, got: %T", err)
			}

			// Assert error mentions eventType field
			ve := domain.AsValidationError(err)
			if ve == nil {
				t.Fatal("expected ValidationError")
			}
			if ve.Field != "eventType" {
				t.Errorf("expected field 'eventType', got '%s'", ve.Field)
			}
		})
	}
}

// TestEventRequest_ToEvent_EmptyMatchID tests that empty matchId is rejected.
func TestEventRequest_ToEvent_EmptyMatchID(t *testing.T) {
	req := &domain.EventRequest{
		EventID:   uuid.New().String(),
		MatchID:   "",
		EventType: "goal",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TeamID:    1,
	}

	_, err := req.ToEvent()
	if err == nil {
		t.Fatal("expected error for empty matchId, got nil")
	}

	// Assert returns ValidationError
	if !domain.IsValidationError(err) {
		t.Errorf("expected ValidationError, got: %T", err)
	}

	// Assert error mentions "matchId"
	ve := domain.AsValidationError(err)
	if ve == nil {
		t.Fatal("expected ValidationError")
	}
	if ve.Field != "matchId" {
		t.Errorf("expected field 'matchId', got '%s'", ve.Field)
	}
}

// TestEventRequest_ToEvent_InvalidTeamID tests that invalid teamIds are rejected.
func TestEventRequest_ToEvent_InvalidTeamID(t *testing.T) {
	testCases := []struct {
		name   string
		teamID int
	}{
		{"zero", 0},
		{"three", 3},
		{"negative", -1},
		{"large number", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &domain.EventRequest{
				EventID:   uuid.New().String(),
				MatchID:   "match-123",
				EventType: "goal",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				TeamID:    tc.teamID,
			}

			_, err := req.ToEvent()
			if err == nil {
				t.Fatalf("expected error for teamId=%d, got nil", tc.teamID)
			}

			// Assert returns ValidationError
			if !domain.IsValidationError(err) {
				t.Errorf("expected ValidationError, got: %T", err)
			}

			// Assert error mentions "teamId"
			ve := domain.AsValidationError(err)
			if ve == nil {
				t.Fatal("expected ValidationError")
			}
			if ve.Field != "teamId" {
				t.Errorf("expected field 'teamId', got '%s'", ve.Field)
			}
		})
	}
}

// TestEventRequest_ToEvent_ValidTeamIDs tests that valid teamIds (1 and 2) succeed.
func TestEventRequest_ToEvent_ValidTeamIDs(t *testing.T) {
	validTeamIDs := []int{1, 2}

	for _, teamID := range validTeamIDs {
		t.Run("team_"+string(rune('0'+teamID)), func(t *testing.T) {
			req := &domain.EventRequest{
				EventID:   uuid.New().String(),
				MatchID:   "match-123",
				EventType: "goal",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				TeamID:    teamID,
			}

			event, err := req.ToEvent()
			if err != nil {
				t.Fatalf("expected no error for teamId=%d, got: %v", teamID, err)
			}
			if event.TeamID != teamID {
				t.Errorf("expected TeamID %d, got %d", teamID, event.TeamID)
			}
		})
	}
}

// TestEvent_MetadataJSON_Empty tests that nil metadata returns "{}".
func TestEvent_MetadataJSON_Empty(t *testing.T) {
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		Metadata:  nil,
	}

	jsonStr := event.MetadataJSON()
	if jsonStr != "{}" {
		t.Errorf("expected '{}' for nil metadata, got '%s'", jsonStr)
	}
}

// TestEvent_MetadataJSON_EmptyMap tests that empty map returns "{}".
func TestEvent_MetadataJSON_EmptyMap(t *testing.T) {
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		Metadata:  map[string]interface{}{},
	}

	jsonStr := event.MetadataJSON()
	if jsonStr != "{}" {
		t.Errorf("expected '{}' for empty metadata map, got '%s'", jsonStr)
	}
}

// TestEvent_MetadataJSON_WithData tests that metadata with keys returns valid JSON.
func TestEvent_MetadataJSON_WithData(t *testing.T) {
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		Metadata: map[string]interface{}{
			"minute":   45,
			"position": "penalty",
			"distance": 12.5,
		},
	}

	jsonStr := event.MetadataJSON()

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("MetadataJSON returned invalid JSON: %v", err)
	}

	// Verify contents
	if parsed["minute"] != float64(45) {
		t.Errorf("expected minute=45, got %v", parsed["minute"])
	}
	if parsed["position"] != "penalty" {
		t.Errorf("expected position='penalty', got %v", parsed["position"])
	}
	if parsed["distance"] != 12.5 {
		t.Errorf("expected distance=12.5, got %v", parsed["distance"])
	}
}

// TestEvent_MetadataJSON_ComplexData tests serialization of complex nested metadata.
func TestEvent_MetadataJSON_ComplexData(t *testing.T) {
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		Metadata: map[string]interface{}{
			"nested": map[string]interface{}{
				"key": "value",
			},
			"array": []interface{}{"a", "b", "c"},
		},
	}

	jsonStr := event.MetadataJSON()

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("MetadataJSON returned invalid JSON for complex data: %v", err)
	}

	// Verify nested object exists
	nested, ok := parsed["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'nested' to be a map")
	}
	if nested["key"] != "value" {
		t.Errorf("expected nested.key='value', got %v", nested["key"])
	}
}

// TestEvent_KafkaSerialization tests round-trip serialization to/from Kafka.
func TestEvent_KafkaSerialization(t *testing.T) {
	original := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-456",
		EventType: domain.EventTypeShot,
		Timestamp: time.Now().UTC().Truncate(time.Nanosecond),
		TeamID:    2,
		PlayerID:  "player-789",
		Metadata: map[string]interface{}{
			"on_target": true,
			"distance":  18.5,
		},
	}

	// Serialize to Kafka message
	data, err := original.ToKafkaMessage()
	if err != nil {
		t.Fatalf("ToKafkaMessage failed: %v", err)
	}

	// Verify serialized data is valid JSON
	if !json.Valid(data) {
		t.Fatal("ToKafkaMessage did not produce valid JSON")
	}

	// Deserialize from Kafka message
	restored, err := domain.EventFromKafkaMessage(data)
	if err != nil {
		t.Fatalf("EventFromKafkaMessage failed: %v", err)
	}

	// Assert all fields match original
	if restored.EventID != original.EventID {
		t.Errorf("EventID mismatch: expected %s, got %s", original.EventID, restored.EventID)
	}
	if restored.MatchID != original.MatchID {
		t.Errorf("MatchID mismatch: expected %s, got %s", original.MatchID, restored.MatchID)
	}
	if restored.EventType != original.EventType {
		t.Errorf("EventType mismatch: expected %s, got %s", original.EventType, restored.EventType)
	}
	if restored.TeamID != original.TeamID {
		t.Errorf("TeamID mismatch: expected %d, got %d", original.TeamID, restored.TeamID)
	}
	if restored.PlayerID != original.PlayerID {
		t.Errorf("PlayerID mismatch: expected %s, got %s", original.PlayerID, restored.PlayerID)
	}

	// Check timestamp (allow for some precision loss in serialization)
	if !restored.Timestamp.Equal(original.Timestamp) {
		// Check if timestamps are within a millisecond
		diff := restored.Timestamp.Sub(original.Timestamp)
		if diff > time.Millisecond || diff < -time.Millisecond {
			t.Errorf("Timestamp mismatch: expected %v, got %v (diff: %v)",
				original.Timestamp, restored.Timestamp, diff)
		}
	}

	// Check metadata
	if restored.Metadata["on_target"] != original.Metadata["on_target"] {
		t.Errorf("Metadata['on_target'] mismatch: expected %v, got %v",
			original.Metadata["on_target"], restored.Metadata["on_target"])
	}
	if restored.Metadata["distance"] != original.Metadata["distance"] {
		t.Errorf("Metadata['distance'] mismatch: expected %v, got %v",
			original.Metadata["distance"], restored.Metadata["distance"])
	}
}

// TestEvent_KafkaSerialization_EmptyMetadata tests round-trip with nil metadata.
func TestEvent_KafkaSerialization_EmptyMetadata(t *testing.T) {
	original := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-456",
		EventType: domain.EventTypePass,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		PlayerID:  "player-123",
		Metadata:  nil,
	}

	// Serialize and deserialize
	data, err := original.ToKafkaMessage()
	if err != nil {
		t.Fatalf("ToKafkaMessage failed: %v", err)
	}

	restored, err := domain.EventFromKafkaMessage(data)
	if err != nil {
		t.Fatalf("EventFromKafkaMessage failed: %v", err)
	}

	if restored.EventID != original.EventID {
		t.Errorf("EventID mismatch after round-trip")
	}
}

// TestEvent_KafkaSerialization_EmptyPlayerID tests round-trip with empty player ID.
func TestEvent_KafkaSerialization_EmptyPlayerID(t *testing.T) {
	original := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-456",
		EventType: domain.EventTypeFoul,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		PlayerID:  "", // Empty player ID
		Metadata:  nil,
	}

	data, err := original.ToKafkaMessage()
	if err != nil {
		t.Fatalf("ToKafkaMessage failed: %v", err)
	}

	restored, err := domain.EventFromKafkaMessage(data)
	if err != nil {
		t.Fatalf("EventFromKafkaMessage failed: %v", err)
	}

	if restored.PlayerID != "" {
		t.Errorf("expected empty PlayerID, got '%s'", restored.PlayerID)
	}
}

// TestEventFromKafkaMessage_InvalidJSON tests deserialization with invalid JSON.
func TestEventFromKafkaMessage_InvalidJSON(t *testing.T) {
	invalidJSON := []byte("not valid json")

	_, err := domain.EventFromKafkaMessage(invalidJSON)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestEventFromKafkaMessage_InvalidUUID tests deserialization with invalid UUID in message.
func TestEventFromKafkaMessage_InvalidUUID(t *testing.T) {
	msg := []byte(`{
		"eventId": "invalid-uuid",
		"matchId": "match-123",
		"eventType": "goal",
		"timestamp": "2024-01-01T12:00:00Z",
		"teamId": 1
	}`)

	_, err := domain.EventFromKafkaMessage(msg)
	if err == nil {
		t.Fatal("expected error for invalid UUID in Kafka message, got nil")
	}
}

// TestEventFromKafkaMessage_InvalidTimestamp tests deserialization with invalid timestamp.
func TestEventFromKafkaMessage_InvalidTimestamp(t *testing.T) {
	validUUID := uuid.New().String()
	msg := []byte(`{
		"eventId": "` + validUUID + `",
		"matchId": "match-123",
		"eventType": "goal",
		"timestamp": "invalid-timestamp",
		"teamId": 1
	}`)

	_, err := domain.EventFromKafkaMessage(msg)
	if err == nil {
		t.Fatal("expected error for invalid timestamp in Kafka message, got nil")
	}
}

// TestValidationError_Error tests the error message format.
func TestValidationError_Error(t *testing.T) {
	ve := domain.NewValidationError("fieldName", "error message")

	errMsg := ve.Error()
	expectedMsg := "validation error: field 'fieldName' error message"

	if errMsg != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, errMsg)
	}
}

// TestIsValidationError tests the IsValidationError helper.
func TestIsValidationError(t *testing.T) {
	ve := domain.NewValidationError("field", "message")

	if !domain.IsValidationError(ve) {
		t.Error("expected IsValidationError to return true for ValidationError")
	}

	regularErr := errors.New("regular error")
	if domain.IsValidationError(regularErr) {
		t.Error("expected IsValidationError to return false for regular error")
	}
}

// TestAsValidationError tests the AsValidationError helper.
func TestAsValidationError(t *testing.T) {
	ve := domain.NewValidationError("field", "message")

	extracted := domain.AsValidationError(ve)
	if extracted == nil {
		t.Fatal("expected AsValidationError to return non-nil for ValidationError")
	}
	if extracted.Field != "field" {
		t.Errorf("expected Field 'field', got '%s'", extracted.Field)
	}

	regularErr := errors.New("regular error")
	extracted = domain.AsValidationError(regularErr)
	if extracted != nil {
		t.Error("expected AsValidationError to return nil for regular error")
	}
}

// Benchmark tests for performance validation
func BenchmarkEventRequest_ToEvent(b *testing.B) {
	req := &domain.EventRequest{
		EventID:   uuid.New().String(),
		MatchID:   "match-123",
		EventType: "goal",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TeamID:    1,
		PlayerID:  "player-456",
		Metadata: map[string]interface{}{
			"minute": 45,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = req.ToEvent()
	}
}

func BenchmarkEvent_ToKafkaMessage(b *testing.B) {
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		PlayerID:  "player-456",
		Metadata: map[string]interface{}{
			"minute": 45,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = event.ToKafkaMessage()
	}
}

func BenchmarkEvent_MetadataJSON(b *testing.B) {
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now().UTC(),
		TeamID:    1,
		Metadata: map[string]interface{}{
			"minute":   45,
			"position": "penalty",
			"distance": 12.5,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.MetadataJSON()
	}
}
