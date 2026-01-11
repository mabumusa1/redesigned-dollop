package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kafkalib "github.com/segmentio/kafka-go"

	"fanfinity/internal/app"
	"fanfinity/internal/kafka"
	"fanfinity/internal/repository"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	// Initialize JSON logger for structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting Fanfinity event consumer",
		slog.String("version", Version),
		slog.String("component", "consumer"),
	)

	// Load configuration from environment
	cfg := app.LoadConfig()

	// Initialize ClickHouse connection directly
	chAddr := fmt.Sprintf("%s:%d", cfg.ClickHouse.Host, cfg.ClickHouse.Port)
	chConn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chAddr},
		Auth: clickhouse.Auth{
			Database: cfg.ClickHouse.Database,
			Username: cfg.ClickHouse.User,
			Password: cfg.ClickHouse.Password,
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
		logger.Error("failed to connect to ClickHouse",
			slog.String("address", chAddr),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// Verify ClickHouse connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := chConn.Ping(ctx); err != nil {
		cancel()
		logger.Error("failed to ping ClickHouse",
			slog.String("address", chAddr),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
	cancel()

	logger.Info("ClickHouse connection established",
		slog.String("address", chAddr),
		slog.String("database", cfg.ClickHouse.Database),
	)

	// Create Kafka reader for events topic
	reader := kafkalib.NewReader(kafkalib.ReaderConfig{
		Brokers:        []string{cfg.Kafka.BootstrapServers},
		Topic:          cfg.Kafka.TopicEvents,
		GroupID:        cfg.Consumer.ConsumerGroup,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		MaxWait:        cfg.Consumer.FlushInterval,
		CommitInterval: time.Second,
		StartOffset:    kafkalib.FirstOffset,
		Dialer: &kafkalib.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		},
	})
	logger.Info("Kafka reader created",
		slog.String("brokers", cfg.Kafka.BootstrapServers),
		slog.String("topic", cfg.Kafka.TopicEvents),
		slog.String("group_id", cfg.Consumer.ConsumerGroup),
	)

	// Create Kafka writer for retry topic
	retryWriter := &kafkalib.Writer{
		Addr:         kafkalib.TCP(cfg.Kafka.BootstrapServers),
		Topic:        cfg.Kafka.TopicRetry,
		Balancer:     &kafkalib.Hash{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 10 * time.Second,
		RequiredAcks: kafkalib.RequireAll,
		Async:        false,
	}
	logger.Info("Kafka retry writer created",
		slog.String("topic", cfg.Kafka.TopicRetry),
	)

	// Create Kafka writer for dead letter topic
	deadWriter := &kafkalib.Writer{
		Addr:         kafkalib.TCP(cfg.Kafka.BootstrapServers),
		Topic:        cfg.Kafka.TopicDead,
		Balancer:     &kafkalib.Hash{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 10 * time.Second,
		RequiredAcks: kafkalib.RequireAll,
		Async:        false,
	}
	logger.Info("Kafka dead letter writer created",
		slog.String("topic", cfg.Kafka.TopicDead),
	)

	// Create ClickHouse repository
	repo := repository.NewClickHouseRepository(chConn, logger)
	logger.Info("ClickHouse repository created")

	// Create batch consumer
	consumer := kafka.NewBatchConsumer(kafka.BatchConsumerConfig{
		Reader:        reader,
		Repository:    repo,
		RetryWriter:   retryWriter,
		DeadWriter:    deadWriter,
		BatchSize:     cfg.Consumer.BatchSize,
		FlushInterval: cfg.Consumer.FlushInterval,
		MaxRetries:    cfg.Consumer.MaxRetries,
		Logger:        logger,
	})
	logger.Info("batch consumer created",
		slog.Int("batch_size", cfg.Consumer.BatchSize),
		slog.Duration("flush_interval", cfg.Consumer.FlushInterval),
		slog.Int("max_retries", cfg.Consumer.MaxRetries),
	)

	// Start Prometheus metrics server in a goroutine
	metricsServer := &http.Server{
		Addr:         ":9091",
		Handler:      promhttp.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		logger.Info("metrics server starting",
			slog.String("address", ":9091"),
		)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Create a context that will be cancelled on shutdown signal
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Start consumer in a goroutine
	go func() {
		consumer.Start(ctx)
	}()

	logger.Info("Fanfinity event consumer is running",
		slog.String("events_topic", cfg.Kafka.TopicEvents),
		slog.String("retry_topic", cfg.Kafka.TopicRetry),
		slog.String("dead_topic", cfg.Kafka.TopicDead),
	)

	// Wait for shutdown signal (SIGINT, SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info("shutdown signal received",
		slog.String("signal", sig.String()),
	)

	// Create shutdown timeout context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Cancel the consumer context to trigger graceful shutdown
	cancel()

	// Stop the consumer (will flush remaining batch)
	consumer.Stop()
	logger.Info("consumer stopped")

	// Shutdown metrics server
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown error",
			slog.String("error", err.Error()),
		)
	}
	logger.Info("metrics server stopped")

	// Close Kafka reader
	if err := reader.Close(); err != nil {
		logger.Error("failed to close Kafka reader",
			slog.String("error", err.Error()),
		)
	}
	logger.Info("Kafka reader closed")

	// Close Kafka writers
	if err := retryWriter.Close(); err != nil {
		logger.Error("failed to close retry writer",
			slog.String("error", err.Error()),
		)
	}
	logger.Info("Kafka retry writer closed")

	if err := deadWriter.Close(); err != nil {
		logger.Error("failed to close dead writer",
			slog.String("error", err.Error()),
		)
	}
	logger.Info("Kafka dead letter writer closed")

	// Close ClickHouse connection
	if err := chConn.Close(); err != nil {
		logger.Error("failed to close ClickHouse connection",
			slog.String("error", err.Error()),
		)
	}
	logger.Info("ClickHouse connection closed")

	logger.Info("Fanfinity event consumer shutdown complete")
}
