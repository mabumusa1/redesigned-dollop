package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"fanfinity/internal/api"
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

	logger.Info("starting Fanfinity API server",
		slog.String("version", Version),
		slog.String("component", "server"),
	)

	// Load configuration from environment
	cfg := app.LoadConfig()

	// Initialize application context (ClickHouse, Kafka connections)
	appCtx, err := app.NewContext(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize application context",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// Create Kafka producer for event ingestion
	producer := kafka.NewEventProducer(appCtx.Producer, logger)
	logger.Info("Kafka producer created",
		slog.String("topic", cfg.Kafka.TopicEvents),
	)

	// Create ClickHouse repository for metrics queries
	repo := repository.NewClickHouseRepository(appCtx.ClickHouse, logger)
	logger.Info("ClickHouse repository created",
		slog.String("database", cfg.ClickHouse.Database),
	)

	// Create HTTP router with dependencies
	router := api.NewRouter(producer, repo, logger)
	logger.Info("HTTP router created")

	// Configure HTTP server with timeouts from config
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Store server reference in app context for graceful shutdown
	appCtx.Server = server

	// Start HTTP server in a goroutine
	go func() {
		logger.Info("HTTP server starting",
			slog.String("address", addr),
			slog.Duration("read_timeout", cfg.Server.ReadTimeout),
			slog.Duration("write_timeout", cfg.Server.WriteTimeout),
			slog.Duration("idle_timeout", cfg.Server.IdleTimeout),
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error",
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}()

	logger.Info("Fanfinity API server is running",
		slog.String("address", addr),
		slog.String("health_endpoint", "/health"),
		slog.String("ready_endpoint", "/ready"),
		slog.String("metrics_endpoint", "/metrics"),
	)

	// Wait for shutdown signal (SIGINT, SIGTERM)
	app.WaitForShutdown(appCtx, 30*time.Second)

	logger.Info("Fanfinity API server shutdown complete")
}
