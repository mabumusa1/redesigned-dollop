package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of match event.
type EventType string

// Event type constants for all supported match events.
const (
	EventTypePass         EventType = "pass"
	EventTypeShot         EventType = "shot"
	EventTypeGoal         EventType = "goal"
	EventTypeFoul         EventType = "foul"
	EventTypeYellowCard   EventType = "yellow_card"
	EventTypeRedCard      EventType = "red_card"
	EventTypeSubstitution EventType = "substitution"
	EventTypeOffside      EventType = "offside"
	EventTypeCorner       EventType = "corner"
	EventTypeFreeKick     EventType = "free_kick"
	EventTypeInterception EventType = "interception"
)

// ValidEventTypes is a map of all valid event types for validation.
var ValidEventTypes = map[EventType]bool{
	EventTypePass:         true,
	EventTypeShot:         true,
	EventTypeGoal:         true,
	EventTypeFoul:         true,
	EventTypeYellowCard:   true,
	EventTypeRedCard:      true,
	EventTypeSubstitution: true,
	EventTypeOffside:      true,
	EventTypeCorner:       true,
	EventTypeFreeKick:     true,
	EventTypeInterception: true,
}

// Event represents a validated match event in the domain layer.
type Event struct {
	EventID   uuid.UUID
	MatchID   string
	EventType EventType
	Timestamp time.Time
	TeamID    int
	PlayerID  string
	Metadata  map[string]interface{}
}

// EventRequest represents the incoming JSON request for an event.
type EventRequest struct {
	EventID   string                 `json:"eventId"`
	MatchID   string                 `json:"matchId"`
	EventType string                 `json:"eventType"`
	Timestamp string                 `json:"timestamp"`
	TeamID    int                    `json:"teamId"`
	PlayerID  string                 `json:"playerId"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToEvent validates and converts an EventRequest to a domain Event.
// Returns a ValidationError if any validation fails.
func (r *EventRequest) ToEvent() (*Event, error) {
	// Parse and validate UUID
	eventUUID, err := uuid.Parse(r.EventID)
	if err != nil {
		return nil, NewValidationError("eventId", "must be a valid UUID")
	}

	// Validate matchId is not empty
	if r.MatchID == "" {
		return nil, NewValidationError("matchId", "is required")
	}

	// Validate event type
	eventType := EventType(r.EventType)
	if !ValidEventTypes[eventType] {
		return nil, NewValidationError("eventType", "must be a valid event type")
	}

	// Parse and validate timestamp
	timestamp, err := time.Parse(time.RFC3339, r.Timestamp)
	if err != nil {
		return nil, NewValidationError("timestamp", "must be a valid RFC3339 timestamp")
	}

	// Validate teamId is 1 or 2
	if r.TeamID != 1 && r.TeamID != 2 {
		return nil, NewValidationError("teamId", "must be 1 or 2")
	}

	return &Event{
		EventID:   eventUUID,
		MatchID:   r.MatchID,
		EventType: eventType,
		Timestamp: timestamp,
		TeamID:    r.TeamID,
		PlayerID:  r.PlayerID,
		Metadata:  r.Metadata,
	}, nil
}

// MetadataJSON serializes the event metadata to a JSON string.
// Returns an empty JSON object "{}" if metadata is nil or serialization fails.
func (e *Event) MetadataJSON() string {
	if e.Metadata == nil {
		return "{}"
	}
	data, err := json.Marshal(e.Metadata)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// KafkaMessage represents the serialized form of an Event for Kafka.
type KafkaMessage struct {
	EventID   string                 `json:"eventId"`
	MatchID   string                 `json:"matchId"`
	EventType string                 `json:"eventType"`
	Timestamp string                 `json:"timestamp"`
	TeamID    int                    `json:"teamId"`
	PlayerID  string                 `json:"playerId"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToKafkaMessage converts an Event to a JSON byte slice for Kafka.
func (e *Event) ToKafkaMessage() ([]byte, error) {
	msg := KafkaMessage{
		EventID:   e.EventID.String(),
		MatchID:   e.MatchID,
		EventType: string(e.EventType),
		Timestamp: e.Timestamp.Format(time.RFC3339Nano),
		TeamID:    e.TeamID,
		PlayerID:  e.PlayerID,
		Metadata:  e.Metadata,
	}
	return json.Marshal(msg)
}

// EventFromKafkaMessage deserializes a Kafka message into an Event.
func EventFromKafkaMessage(data []byte) (*Event, error) {
	var msg KafkaMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	eventUUID, err := uuid.Parse(msg.EventID)
	if err != nil {
		return nil, err
	}

	timestamp, err := time.Parse(time.RFC3339Nano, msg.Timestamp)
	if err != nil {
		// Try RFC3339 as fallback
		timestamp, err = time.Parse(time.RFC3339, msg.Timestamp)
		if err != nil {
			return nil, err
		}
	}

	return &Event{
		EventID:   eventUUID,
		MatchID:   msg.MatchID,
		EventType: EventType(msg.EventType),
		Timestamp: timestamp,
		TeamID:    msg.TeamID,
		PlayerID:  msg.PlayerID,
		Metadata:  msg.Metadata,
	}, nil
}
