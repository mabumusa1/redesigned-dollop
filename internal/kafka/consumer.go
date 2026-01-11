package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"

	"fanfinity/internal/domain"
)

var (
	// Prometheus metrics for Kafka consumer
	kafkaConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "fanfinity",
			Subsystem: "kafka_consumer",
			Name:      "lag",
			Help:      "Current consumer lag (difference between latest offset and committed offset)",
		},
		[]string{"topic", "partition"},
	)

	kafkaBatchesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "fanfinity",
			Subsystem: "kafka_consumer",
			Name:      "batches_processed_total",
			Help:      "Total number of batches processed",
		},
		[]string{"status"},
	)

	kafkaEventsConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "fanfinity",
			Subsystem: "kafka_consumer",
			Name:      "events_consumed_total",
			Help:      "Total number of events consumed from Kafka",
		},
		[]string{"status"},
	)

	kafkaConsumeDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "fanfinity",
			Subsystem: "kafka_consumer",
			Name:      "consume_duration_seconds",
			Help:      "Histogram of batch processing duration in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"operation"},
	)

	kafkaRetryEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "fanfinity",
			Subsystem: "kafka_consumer",
			Name:      "retry_events_total",
			Help:      "Total number of events sent to retry topic",
		},
		[]string{"status"},
	)

	kafkaDeadLetterEvents = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "fanfinity",
			Subsystem: "kafka_consumer",
			Name:      "dead_letter_events_total",
			Help:      "Total number of events sent to dead letter queue",
		},
	)
)

// Repository defines the interface for batch event insertion.
type Repository interface {
	InsertBatch(ctx context.Context, events []*domain.Event) error
}

// BatchConsumer consumes events from Kafka and batch inserts them into ClickHouse.
type BatchConsumer struct {
	reader        *kafka.Reader
	repository    Repository
	retryWriter   *kafka.Writer
	deadWriter    *kafka.Writer
	batchSize     int
	flushInterval time.Duration
	maxRetries    int
	logger        *slog.Logger

	batch     []*domain.Event
	messages  []kafka.Message
	batchLock sync.Mutex
	ticker    *time.Ticker
	done      chan struct{}
	wg        sync.WaitGroup
}

// BatchConsumerConfig holds configuration for the batch consumer.
type BatchConsumerConfig struct {
	Reader        *kafka.Reader
	Repository    Repository
	RetryWriter   *kafka.Writer
	DeadWriter    *kafka.Writer
	BatchSize     int
	FlushInterval time.Duration
	MaxRetries    int
	Logger        *slog.Logger
}

// NewBatchConsumer creates a new BatchConsumer instance.
func NewBatchConsumer(cfg BatchConsumerConfig) *BatchConsumer {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &BatchConsumer{
		reader:        cfg.Reader,
		repository:    cfg.Repository,
		retryWriter:   cfg.RetryWriter,
		deadWriter:    cfg.DeadWriter,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		maxRetries:    cfg.MaxRetries,
		logger:        cfg.Logger,
		batch:         make([]*domain.Event, 0, cfg.BatchSize),
		messages:      make([]kafka.Message, 0, cfg.BatchSize),
		done:          make(chan struct{}),
	}
}

// Start begins consuming messages from Kafka.
// This method blocks until Stop() is called or the context is cancelled.
func (c *BatchConsumer) Start(ctx context.Context) {
	c.logger.Info("starting batch consumer",
		slog.Int("batch_size", c.batchSize),
		slog.Duration("flush_interval", c.flushInterval),
	)

	c.ticker = time.NewTicker(c.flushInterval)
	defer c.ticker.Stop()

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("context cancelled, flushing remaining batch")
			c.flushWithContext(context.Background())
			return

		case <-c.done:
			c.logger.Info("stop signal received, flushing remaining batch")
			c.flushWithContext(context.Background())
			return

		case <-c.ticker.C:
			c.flushWithContext(ctx)

		default:
			// Fetch message with a short timeout to allow checking for shutdown
			fetchCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			msg, err := c.reader.FetchMessage(fetchCtx)
			cancel()

			if err != nil {
				// Check if context was cancelled or timeout
				if ctx.Err() != nil {
					continue
				}
				// Timeout is expected when no messages are available
				if fetchCtx.Err() == context.DeadlineExceeded {
					continue
				}
				c.logger.Error("failed to fetch message",
					slog.String("error", err.Error()),
				)
				continue
			}

			// Update consumer lag metric
			c.updateLagMetric(msg)

			// Parse the message
			event, err := domain.EventFromKafkaMessage(msg.Value)
			if err != nil {
				c.logger.Error("failed to parse message",
					slog.String("error", err.Error()),
					slog.Int64("offset", msg.Offset),
					slog.Int("partition", msg.Partition),
				)
				kafkaEventsConsumed.WithLabelValues("parse_error").Inc()
				// Commit the message even if parsing failed to avoid reprocessing
				if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
					c.logger.Error("failed to commit message after parse error",
						slog.String("error", commitErr.Error()),
					)
				}
				continue
			}

			// Add to batch
			c.batchLock.Lock()
			c.batch = append(c.batch, event)
			c.messages = append(c.messages, msg)
			batchLen := len(c.batch)
			c.batchLock.Unlock()

			c.logger.Debug("message added to batch",
				slog.String("event_id", event.EventID.String()),
				slog.Int("batch_size", batchLen),
			)

			// Flush if batch is full
			if batchLen >= c.batchSize {
				c.flushWithContext(ctx)
			}
		}
	}
}

// updateLagMetric updates the consumer lag Prometheus metric.
func (c *BatchConsumer) updateLagMetric(msg kafka.Message) {
	// Get the current lag stats from the reader
	stats := c.reader.Stats()
	kafkaConsumerLag.WithLabelValues(
		stats.Topic,
		fmt.Sprintf("%d", msg.Partition),
	).Set(float64(stats.Lag))
}

// flushWithContext flushes the current batch to the repository.
func (c *BatchConsumer) flushWithContext(ctx context.Context) {
	c.batchLock.Lock()
	if len(c.batch) == 0 {
		c.batchLock.Unlock()
		return
	}

	// Take ownership of the current batch
	events := c.batch
	messages := c.messages
	c.batch = make([]*domain.Event, 0, c.batchSize)
	c.messages = make([]kafka.Message, 0, c.batchSize)
	c.batchLock.Unlock()

	startTime := time.Now()
	c.logger.Debug("flushing batch",
		slog.Int("batch_size", len(events)),
	)

	// Insert batch into ClickHouse
	err := c.repository.InsertBatch(ctx, events)
	duration := time.Since(startTime)
	kafkaConsumeDuration.WithLabelValues("insert_batch").Observe(duration.Seconds())

	if err != nil {
		c.logger.Error("failed to insert batch",
			slog.Int("batch_size", len(events)),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		kafkaBatchesProcessed.WithLabelValues("error").Inc()

		// Send failed events to retry topic
		c.sendToRetry(ctx, events, messages)
		return
	}

	// Commit messages after successful insert
	if len(messages) > 0 {
		if err := c.reader.CommitMessages(ctx, messages...); err != nil {
			c.logger.Error("failed to commit messages",
				slog.Int("message_count", len(messages)),
				slog.String("error", err.Error()),
			)
			// Continue despite commit failure - events are already in ClickHouse
		}
	}

	c.logger.Info("batch flushed successfully",
		slog.Int("batch_size", len(events)),
		slog.Duration("duration", duration),
	)
	kafkaBatchesProcessed.WithLabelValues("success").Inc()
	kafkaEventsConsumed.WithLabelValues("success").Add(float64(len(events)))
}

// sendToRetry sends failed events to the retry topic.
func (c *BatchConsumer) sendToRetry(ctx context.Context, events []*domain.Event, originalMessages []kafka.Message) {
	if c.retryWriter == nil {
		c.logger.Warn("retry writer not configured, sending to dead letter",
			slog.Int("event_count", len(events)),
		)
		c.sendToDead(ctx, events)
		return
	}

	c.logger.Info("sending events to retry topic",
		slog.Int("event_count", len(events)),
	)

	retryMessages := make([]kafka.Message, 0, len(events))
	for i, event := range events {
		value, err := event.ToKafkaMessage()
		if err != nil {
			c.logger.Warn("failed to serialize event for retry",
				slog.String("event_id", event.EventID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Extract retry count from original message headers
		retryCount := 0
		if i < len(originalMessages) {
			for _, header := range originalMessages[i].Headers {
				if header.Key == "retry_count" && len(header.Value) > 0 {
					retryCount = int(header.Value[0])
				}
			}
		}
		retryCount++

		// Check if max retries exceeded
		if retryCount > c.maxRetries {
			c.logger.Warn("max retries exceeded, sending to dead letter",
				slog.String("event_id", event.EventID.String()),
				slog.Int("retry_count", retryCount),
			)
			c.sendSingleToDead(ctx, event)
			continue
		}

		msg := kafka.Message{
			Key:   []byte(event.MatchID),
			Value: value,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(string(event.EventType))},
				{Key: "event_id", Value: []byte(event.EventID.String())},
				{Key: "retry_count", Value: []byte{byte(retryCount)}},
				{Key: "original_timestamp", Value: []byte(event.Timestamp.Format(time.RFC3339Nano))},
			},
		}
		retryMessages = append(retryMessages, msg)
	}

	if len(retryMessages) == 0 {
		return
	}

	err := c.retryWriter.WriteMessages(ctx, retryMessages...)
	if err != nil {
		c.logger.Error("failed to write to retry topic, sending to dead letter",
			slog.Int("event_count", len(retryMessages)),
			slog.String("error", err.Error()),
		)
		kafkaRetryEvents.WithLabelValues("error").Add(float64(len(retryMessages)))
		c.sendToDead(ctx, events)
		return
	}

	c.logger.Info("events sent to retry topic",
		slog.Int("event_count", len(retryMessages)),
	)
	kafkaRetryEvents.WithLabelValues("success").Add(float64(len(retryMessages)))

	// Commit original messages since we've sent to retry
	if len(originalMessages) > 0 {
		if err := c.reader.CommitMessages(ctx, originalMessages...); err != nil {
			c.logger.Error("failed to commit messages after retry",
				slog.String("error", err.Error()),
			)
		}
	}
}

// sendToDead sends events to the dead letter queue.
func (c *BatchConsumer) sendToDead(ctx context.Context, events []*domain.Event) {
	for _, event := range events {
		c.sendSingleToDead(ctx, event)
	}
}

// sendSingleToDead sends a single event to the dead letter queue.
func (c *BatchConsumer) sendSingleToDead(ctx context.Context, event *domain.Event) {
	if c.deadWriter == nil {
		c.logger.Error("dead letter writer not configured, event lost",
			slog.String("event_id", event.EventID.String()),
		)
		return
	}

	value, err := event.ToKafkaMessage()
	if err != nil {
		c.logger.Error("failed to serialize event for dead letter",
			slog.String("event_id", event.EventID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	// Include failure metadata
	failureInfo := map[string]interface{}{
		"event":      json.RawMessage(value),
		"failed_at":  time.Now().Format(time.RFC3339Nano),
		"reason":     "max_retries_exceeded_or_permanent_failure",
		"event_id":   event.EventID.String(),
		"match_id":   event.MatchID,
		"event_type": string(event.EventType),
	}

	deadValue, err := json.Marshal(failureInfo)
	if err != nil {
		c.logger.Error("failed to marshal dead letter metadata",
			slog.String("event_id", event.EventID.String()),
			slog.String("error", err.Error()),
		)
		deadValue = value // Fall back to just the event
	}

	msg := kafka.Message{
		Key:   []byte(event.MatchID),
		Value: deadValue,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(string(event.EventType))},
			{Key: "event_id", Value: []byte(event.EventID.String())},
			{Key: "failed_at", Value: []byte(time.Now().Format(time.RFC3339Nano))},
		},
	}

	err = c.deadWriter.WriteMessages(ctx, msg)
	if err != nil {
		c.logger.Error("failed to write to dead letter queue",
			slog.String("event_id", event.EventID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	c.logger.Warn("event sent to dead letter queue",
		slog.String("event_id", event.EventID.String()),
		slog.String("match_id", event.MatchID),
	)
	kafkaDeadLetterEvents.Inc()
}

// Stop signals the consumer to stop and waits for it to finish.
func (c *BatchConsumer) Stop() {
	c.logger.Info("stopping batch consumer")
	close(c.done)
	c.wg.Wait()
	c.logger.Info("batch consumer stopped")
}

// NewReader creates a new Kafka reader with production-ready configuration.
func NewReader(cfg ReaderConfig) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       cfg.MinBytes,
		MaxBytes:       cfg.MaxBytes,
		MaxWait:        cfg.MaxWait,
		CommitInterval: cfg.CommitInterval,
		StartOffset:    cfg.StartOffset,
		Dialer: &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		},
	})
}

// ReaderConfig holds configuration for Kafka reader.
type ReaderConfig struct {
	Brokers        []string
	Topic          string
	GroupID        string
	MinBytes       int
	MaxBytes       int
	MaxWait        time.Duration
	CommitInterval time.Duration
	StartOffset    int64
}

// DefaultReaderConfig returns default configuration for the events topic consumer.
func DefaultReaderConfig(brokers []string, topic, groupID string) ReaderConfig {
	return ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		MaxWait:        5 * time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.FirstOffset,
	}
}
