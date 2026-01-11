package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/segmentio/kafka-go"
)

// AppContext is the central dependency injection container for the application.
// It holds all initialized connections, configuration, and provides lifecycle management.
type AppContext struct {
	Config     *Config
	Logger     *slog.Logger
	Producer   *kafka.Writer
	Consumer   *kafka.Reader
	ClickHouse driver.Conn
	Server     *http.Server

	shutdownCh chan struct{}
}

// ContextOptions configures which components to initialize.
type ContextOptions struct {
	InitProducer bool // Initialize Kafka producer (for API server)
	InitConsumer bool // Initialize Kafka consumer (for consumer service)
}

// NewContext creates and initializes a new AppContext with all dependencies.
// Use opts to control which Kafka components to initialize.
func NewContext(cfg *Config, logger *slog.Logger, opts ContextOptions) (*AppContext, error) {
	ctx := &AppContext{
		Config:     cfg,
		Logger:     logger,
		shutdownCh: make(chan struct{}),
	}

	// Initialize ClickHouse connection
	if err := ctx.initClickHouse(); err != nil {
		return nil, fmt.Errorf("failed to initialize ClickHouse: %w", err)
	}
	logger.Info("ClickHouse connection established",
		slog.String("host", cfg.ClickHouse.Host),
		slog.Int("port", cfg.ClickHouse.Port),
		slog.String("database", cfg.ClickHouse.Database),
	)

	// Initialize Kafka producer (only for API server)
	if opts.InitProducer {
		ctx.initKafkaProducer()
		logger.Info("Kafka producer initialized",
			slog.String("brokers", cfg.Kafka.BootstrapServers),
			slog.String("topic", cfg.Kafka.TopicEvents),
		)
	}

	// Initialize Kafka consumer (only for consumer service)
	if opts.InitConsumer {
		ctx.initKafkaConsumer()
		logger.Info("Kafka consumer initialized",
			slog.String("brokers", cfg.Kafka.BootstrapServers),
			slog.String("topic", cfg.Kafka.TopicEvents),
			slog.String("group", cfg.Consumer.ConsumerGroup),
		)
	}

	return ctx, nil
}

// NewServerContext creates an AppContext for the API server (producer only, no consumer).
func NewServerContext(cfg *Config, logger *slog.Logger) (*AppContext, error) {
	return NewContext(cfg, logger, ContextOptions{InitProducer: true, InitConsumer: false})
}

// NewConsumerContext creates an AppContext for the consumer service (consumer only, no producer).
func NewConsumerContext(cfg *Config, logger *slog.Logger) (*AppContext, error) {
	return NewContext(cfg, logger, ContextOptions{InitProducer: false, InitConsumer: true})
}

// initClickHouse establishes a connection to ClickHouse with appropriate settings.
func (c *AppContext) initClickHouse() error {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", c.Config.ClickHouse.Host, c.Config.ClickHouse.Port)},
		Auth: clickhouse.Auth{
			Database: c.Config.ClickHouse.Database,
			Username: c.Config.ClickHouse.User,
			Password: c.Config.ClickHouse.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:          10 * time.Second,
		MaxOpenConns:         10,
		MaxIdleConns:         5,
		ConnMaxLifetime:      time.Hour,
		ConnOpenStrategy:     clickhouse.ConnOpenInOrder,
		BlockBufferSize:      10,
		MaxCompressionBuffer: 10240,
	})
	if err != nil {
		return fmt.Errorf("failed to open ClickHouse connection: %w", err)
	}

	// Verify the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	c.ClickHouse = conn
	return nil
}

// initKafkaProducer creates and configures the Kafka writer for producing events.
func (c *AppContext) initKafkaProducer() {
	c.Producer = &kafka.Writer{
		Addr:         kafka.TCP(c.Config.Kafka.BootstrapServers),
		Topic:        c.Config.Kafka.TopicEvents,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: c.Config.Kafka.ProducerTimeout,
		RequiredAcks: kafka.RequireAll,
		Async:        false, // Synchronous for durability guarantees
		Compression:  kafka.Snappy,
		Transport: &kafka.Transport{
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				DualStack: true,
			}).DialContext,
		},
	}
}

// initKafkaConsumer creates and configures the Kafka reader for consuming events.
func (c *AppContext) initKafkaConsumer() {
	c.Consumer = kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{c.Config.Kafka.BootstrapServers},
		Topic:          c.Config.Kafka.TopicEvents,
		GroupID:        c.Config.Consumer.ConsumerGroup,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		MaxWait:        c.Config.Consumer.FlushInterval,
		CommitInterval: time.Second,
		StartOffset:    kafka.FirstOffset,
		Dialer: &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		},
	})
}

// ShutdownChan returns the channel that signals application shutdown.
func (c *AppContext) ShutdownChan() <-chan struct{} {
	return c.shutdownCh
}

// Shutdown gracefully closes all connections in the proper order.
// It ensures that:
// 1. HTTP server stops accepting new requests
// 2. Kafka producer flushes remaining messages
// 3. Kafka consumer commits offsets and closes
// 4. ClickHouse connection is closed
func (c *AppContext) Shutdown(ctx context.Context) error {
	c.Logger.Info("Starting graceful shutdown")

	var errs []error

	// Signal shutdown to any listeners
	close(c.shutdownCh)

	// 1. Shutdown HTTP server first to stop accepting new requests
	if c.Server != nil {
		c.Logger.Info("Shutting down HTTP server")
		if err := c.Server.Shutdown(ctx); err != nil {
			c.Logger.Error("HTTP server shutdown error", slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("HTTP server shutdown: %w", err))
		}
	}

	// 2. Close Kafka producer to flush remaining messages
	if c.Producer != nil {
		c.Logger.Info("Closing Kafka producer")
		if err := c.Producer.Close(); err != nil {
			c.Logger.Error("Kafka producer close error", slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("Kafka producer close: %w", err))
		}
	}

	// 3. Close Kafka consumer to commit offsets
	if c.Consumer != nil {
		c.Logger.Info("Closing Kafka consumer")
		if err := c.Consumer.Close(); err != nil {
			c.Logger.Error("Kafka consumer close error", slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("Kafka consumer close: %w", err))
		}
	}

	// 4. Close ClickHouse connection last
	if c.ClickHouse != nil {
		c.Logger.Info("Closing ClickHouse connection")
		if err := c.ClickHouse.Close(); err != nil {
			c.Logger.Error("ClickHouse close error", slog.String("error", err.Error()))
			errs = append(errs, fmt.Errorf("ClickHouse close: %w", err))
		}
	}

	if len(errs) > 0 {
		c.Logger.Error("Shutdown completed with errors", slog.Int("error_count", len(errs)))
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	c.Logger.Info("Graceful shutdown completed successfully")
	return nil
}
