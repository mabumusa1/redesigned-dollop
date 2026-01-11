package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"fanfinity/internal/domain"
)

// mockRepository is a mock implementation of Repository for testing.
type mockRepository struct {
	insertErr     error
	insertedBatch []*domain.Event
	insertCalled  bool
	mu            sync.Mutex
}

func (m *mockRepository) InsertBatch(ctx context.Context, events []*domain.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertCalled = true
	if m.insertErr != nil {
		return m.insertErr
	}
	m.insertedBatch = append(m.insertedBatch, events...)
	return nil
}

func (m *mockRepository) getInsertedEvents() []*domain.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.insertedBatch
}

func TestNewBatchConsumer_DefaultValues(t *testing.T) {
	cfg := BatchConsumerConfig{
		// Leave defaults
	}

	consumer := NewBatchConsumer(cfg)

	if consumer.batchSize != 1000 {
		t.Errorf("expected default batch size 1000, got %d", consumer.batchSize)
	}
	if consumer.flushInterval != 5*time.Second {
		t.Errorf("expected default flush interval 5s, got %v", consumer.flushInterval)
	}
	if consumer.maxRetries != 3 {
		t.Errorf("expected default max retries 3, got %d", consumer.maxRetries)
	}
	if consumer.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestNewBatchConsumer_CustomValues(t *testing.T) {
	cfg := BatchConsumerConfig{
		BatchSize:     500,
		FlushInterval: 10 * time.Second,
		MaxRetries:    5,
	}

	consumer := NewBatchConsumer(cfg)

	if consumer.batchSize != 500 {
		t.Errorf("expected batch size 500, got %d", consumer.batchSize)
	}
	if consumer.flushInterval != 10*time.Second {
		t.Errorf("expected flush interval 10s, got %v", consumer.flushInterval)
	}
	if consumer.maxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", consumer.maxRetries)
	}
}

func TestNewBatchConsumer_InitializesBatchSlices(t *testing.T) {
	cfg := BatchConsumerConfig{
		BatchSize: 100,
	}

	consumer := NewBatchConsumer(cfg)

	if cap(consumer.batch) != 100 {
		t.Errorf("expected batch capacity 100, got %d", cap(consumer.batch))
	}
	if len(consumer.batch) != 0 {
		t.Errorf("expected batch length 0, got %d", len(consumer.batch))
	}
	if cap(consumer.messages) != 100 {
		t.Errorf("expected messages capacity 100, got %d", cap(consumer.messages))
	}
}

func TestNewReader(t *testing.T) {
	cfg := ReaderConfig{
		Brokers:        []string{"localhost:9092"},
		Topic:          "events",
		GroupID:        "test-group",
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        5 * time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.FirstOffset,
	}

	reader := NewReader(cfg)

	if reader == nil {
		t.Fatal("expected non-nil reader")
	}

	stats := reader.Stats()
	if stats.Topic != cfg.Topic {
		t.Errorf("expected topic %s, got %s", cfg.Topic, stats.Topic)
	}
}

func TestDefaultReaderConfig(t *testing.T) {
	brokers := []string{"localhost:9092"}
	topic := "events"
	groupID := "test-group"

	cfg := DefaultReaderConfig(brokers, topic, groupID)

	if len(cfg.Brokers) != 1 || cfg.Brokers[0] != "localhost:9092" {
		t.Error("unexpected brokers")
	}
	if cfg.Topic != topic {
		t.Errorf("expected topic %s, got %s", topic, cfg.Topic)
	}
	if cfg.GroupID != groupID {
		t.Errorf("expected group ID %s, got %s", groupID, cfg.GroupID)
	}
	if cfg.MinBytes != 1 {
		t.Errorf("expected min bytes 1, got %d", cfg.MinBytes)
	}
	if cfg.MaxBytes != 10e6 {
		t.Errorf("expected max bytes 10MB, got %d", cfg.MaxBytes)
	}
	if cfg.MaxWait != 5*time.Second {
		t.Errorf("expected max wait 5s, got %v", cfg.MaxWait)
	}
	if cfg.CommitInterval != time.Second {
		t.Errorf("expected commit interval 1s, got %v", cfg.CommitInterval)
	}
	if cfg.StartOffset != kafka.FirstOffset {
		t.Errorf("expected first offset, got %d", cfg.StartOffset)
	}
}

func TestBatchConsumer_FlushEmptyBatch(t *testing.T) {
	repo := &mockRepository{}
	consumer := NewBatchConsumer(BatchConsumerConfig{
		Repository: repo,
		BatchSize:  10,
	})

	// Flush with empty batch should not call repository
	consumer.flushWithContext(context.Background())

	if repo.insertCalled {
		t.Error("expected insert not to be called for empty batch")
	}
}

func TestBatchConsumer_FlushWithEvents(t *testing.T) {
	repo := &mockRepository{}
	consumer := NewBatchConsumer(BatchConsumerConfig{
		Repository: repo,
		BatchSize:  10,
	})

	// Add events to batch
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now(),
		TeamID:    1,
	}
	consumer.batch = append(consumer.batch, event)

	// Flush should call repository
	consumer.flushWithContext(context.Background())

	if !repo.insertCalled {
		t.Error("expected insert to be called")
	}
	if len(repo.getInsertedEvents()) != 1 {
		t.Errorf("expected 1 event inserted, got %d", len(repo.getInsertedEvents()))
	}
}

func TestBatchConsumer_FlushError(t *testing.T) {
	repo := &mockRepository{
		insertErr: errors.New("insert failed"),
	}
	consumer := NewBatchConsumer(BatchConsumerConfig{
		Repository: repo,
		BatchSize:  10,
	})

	// Add event to batch
	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now(),
		TeamID:    1,
	}
	consumer.batch = append(consumer.batch, event)

	// Flush should handle error gracefully
	consumer.flushWithContext(context.Background())

	if !repo.insertCalled {
		t.Error("expected insert to be called")
	}
}

func TestBatchConsumer_Stop(t *testing.T) {
	consumer := NewBatchConsumer(BatchConsumerConfig{
		BatchSize: 10,
	})

	// Start a goroutine that waits for done signal
	done := make(chan struct{})
	consumer.wg.Add(1)
	go func() {
		defer consumer.wg.Done()
		<-consumer.done
		close(done)
	}()

	// Stop should close the done channel
	consumer.Stop()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("expected done channel to be closed")
	}
}

func TestBatchConsumer_SendToRetry_NoRetryWriter(t *testing.T) {
	consumer := NewBatchConsumer(BatchConsumerConfig{
		RetryWriter: nil,
		DeadWriter:  nil,
		BatchSize:   10,
	})

	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now(),
		TeamID:    1,
	}

	// Should not panic when retry writer is nil
	consumer.sendToRetry(context.Background(), []*domain.Event{event}, nil)
}

func TestBatchConsumer_SendToDead_NoDeadWriter(t *testing.T) {
	consumer := NewBatchConsumer(BatchConsumerConfig{
		DeadWriter: nil,
		BatchSize:  10,
	})

	event := &domain.Event{
		EventID:   uuid.New(),
		MatchID:   "match-123",
		EventType: domain.EventTypeGoal,
		Timestamp: time.Now(),
		TeamID:    1,
	}

	// Should not panic when dead writer is nil
	consumer.sendSingleToDead(context.Background(), event)
}

func BenchmarkBatchConsumer_FlushBatch(b *testing.B) {
	repo := &mockRepository{}
	consumer := NewBatchConsumer(BatchConsumerConfig{
		Repository: repo,
		BatchSize:  1000,
	})

	// Create test events
	events := make([]*domain.Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = &domain.Event{
			EventID:   uuid.New(),
			MatchID:   "match-123",
			EventType: domain.EventTypePass,
			Timestamp: time.Now(),
			TeamID:    1,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consumer.batch = make([]*domain.Event, len(events))
		copy(consumer.batch, events)
		consumer.flushWithContext(context.Background())
	}
}
