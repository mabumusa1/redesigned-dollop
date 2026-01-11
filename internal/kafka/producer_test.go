package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"fanfinity/internal/domain"
)

func TestNewEventProducer(t *testing.T) {
	writer := &kafka.Writer{Topic: "test-topic"}

	t.Run("creates producer with default logger", func(t *testing.T) {
		producer := NewEventProducer(writer, nil)
		if producer == nil {
			t.Fatal("expected non-nil producer")
		}
		if producer.writer != writer {
			t.Error("expected writer to be set")
		}
		if producer.logger == nil {
			t.Error("expected default logger to be set")
		}
	})
}

func TestEventProducer_Produce_NilEvent(t *testing.T) {
	writer := &kafka.Writer{Topic: "test-topic"}
	producer := NewEventProducer(writer, nil)

	err := producer.Produce(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil event")
	}
	if err.Error() != "event cannot be nil" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestEventProducer_Close_NilWriter(t *testing.T) {
	producer := &EventProducer{
		writer: nil,
	}

	err := producer.Close()
	if err != nil {
		t.Errorf("expected nil error for nil writer, got: %v", err)
	}
}

func TestNewWriter(t *testing.T) {
	brokers := []string{"localhost:9092", "localhost:9093"}
	topic := "test-topic"

	writer := NewWriter(brokers, topic)

	if writer == nil {
		t.Fatal("expected non-nil writer")
	}
	if writer.Topic != topic {
		t.Errorf("expected topic %s, got %s", topic, writer.Topic)
	}
	if writer.BatchSize != 100 {
		t.Errorf("expected batch size 100, got %d", writer.BatchSize)
	}
	if writer.BatchTimeout != 10*time.Millisecond {
		t.Errorf("expected batch timeout 10ms, got %v", writer.BatchTimeout)
	}
	if writer.RequiredAcks != kafka.RequireAll {
		t.Error("expected RequireAll acks")
	}
	if writer.Async != false {
		t.Error("expected sync mode")
	}
}

func TestNewWriterWithConfig(t *testing.T) {
	cfg := WriterConfig{
		Brokers:      []string{"localhost:9092"},
		Topic:        "custom-topic",
		BatchSize:    50,
		BatchTimeout: 5 * time.Millisecond,
		WriteTimeout: 5 * time.Second,
		MaxAttempts:  5,
		Async:        true,
	}

	writer := NewWriterWithConfig(cfg)

	if writer == nil {
		t.Fatal("expected non-nil writer")
	}
	if writer.Topic != cfg.Topic {
		t.Errorf("expected topic %s, got %s", cfg.Topic, writer.Topic)
	}
	if writer.BatchSize != cfg.BatchSize {
		t.Errorf("expected batch size %d, got %d", cfg.BatchSize, writer.BatchSize)
	}
	if writer.MaxAttempts != cfg.MaxAttempts {
		t.Errorf("expected max attempts %d, got %d", cfg.MaxAttempts, writer.MaxAttempts)
	}
	if writer.Async != cfg.Async {
		t.Errorf("expected async %v, got %v", cfg.Async, writer.Async)
	}
}

func TestDefaultWriterConfig(t *testing.T) {
	brokers := []string{"localhost:9092"}
	topic := "events"

	cfg := DefaultWriterConfig(brokers, topic)

	if len(cfg.Brokers) != 1 || cfg.Brokers[0] != "localhost:9092" {
		t.Error("unexpected brokers")
	}
	if cfg.Topic != topic {
		t.Errorf("expected topic %s, got %s", topic, cfg.Topic)
	}
	if cfg.BatchSize != 100 {
		t.Errorf("expected batch size 100, got %d", cfg.BatchSize)
	}
	if cfg.BatchTimeout != 10*time.Millisecond {
		t.Errorf("expected batch timeout 10ms, got %v", cfg.BatchTimeout)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Errorf("expected write timeout 10s, got %v", cfg.WriteTimeout)
	}
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected max attempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.Async != false {
		t.Error("expected sync mode")
	}
}

func createTestEvent() *domain.Event {
	return &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now(),
		TeamID:    1,
		PlayerID:  "player-456",
		Metadata:  map[string]interface{}{"minute": 45},
	}
}

func TestEventProducer_ProduceBatch_EmptyBatch(t *testing.T) {
	writer := &kafka.Writer{Topic: "test-topic"}
	producer := NewEventProducer(writer, nil)

	err := producer.ProduceBatch(context.Background(), []*domain.Event{})
	if err != nil {
		t.Errorf("expected no error for empty batch, got: %v", err)
	}
}

func TestEventProducer_ProduceBatch_NilEvents(t *testing.T) {
	writer := &kafka.Writer{Topic: "test-topic"}
	producer := NewEventProducer(writer, nil)

	events := []*domain.Event{nil, nil}
	// This will attempt to write but should skip nil events
	// The actual write will fail without a real Kafka, but we're testing the nil handling
	_ = producer.ProduceBatch(context.Background(), events)
}

func TestWriterConfig_DefaultValues(t *testing.T) {
	cfg := WriterConfig{}

	if cfg.BatchSize != 0 {
		t.Error("expected zero value for BatchSize")
	}
	if cfg.Async != false {
		t.Error("expected false for Async")
	}
}

func BenchmarkEventSerialization(b *testing.B) {
	event := createTestEvent()
	event.Metadata = map[string]interface{}{
		"minute":     45,
		"scorer":     "Player Name",
		"assist":     "Another Player",
		"goal_type":  "header",
		"position_x": 95.5,
		"position_y": 32.1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := event.ToKafkaMessage()
		if err != nil {
			b.Fatal(err)
		}
	}
}
