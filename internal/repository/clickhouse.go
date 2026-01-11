package repository

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"fanfinity/internal/domain"
)

var (
	// Prometheus metrics for ClickHouse repository
	clickhouseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "fanfinity",
			Subsystem: "clickhouse",
			Name:      "query_duration_seconds",
			Help:      "Histogram of ClickHouse query latency in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"operation"},
	)

	clickhouseQueryErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "fanfinity",
			Subsystem: "clickhouse",
			Name:      "query_errors_total",
			Help:      "Total number of ClickHouse query errors",
		},
		[]string{"operation"},
	)

	clickhouseBatchSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "fanfinity",
			Subsystem: "clickhouse",
			Name:      "batch_size",
			Help:      "Histogram of batch insert sizes",
			Buckets:   []float64{1, 10, 50, 100, 500, 1000, 5000, 10000},
		},
		[]string{},
	)

	clickhouseEventsInserted = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "fanfinity",
			Subsystem: "clickhouse",
			Name:      "events_inserted_total",
			Help:      "Total number of events inserted into ClickHouse",
		},
	)
)

// ClickHouseRepository handles ClickHouse database operations.
type ClickHouseRepository struct {
	conn   driver.Conn
	logger *slog.Logger
}

// NewClickHouseRepository creates a new ClickHouseRepository instance.
func NewClickHouseRepository(conn driver.Conn, logger *slog.Logger) *ClickHouseRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &ClickHouseRepository{
		conn:   conn,
		logger: logger,
	}
}

// Ping performs a health check on the ClickHouse connection.
func (r *ClickHouseRepository) Ping(ctx context.Context) error {
	startTime := time.Now()
	err := r.conn.Ping(ctx)
	duration := time.Since(startTime)

	clickhouseQueryDuration.WithLabelValues("ping").Observe(duration.Seconds())

	if err != nil {
		r.logger.Error("ClickHouse ping failed",
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("ping").Inc()
		return fmt.Errorf("ClickHouse ping failed: %w", err)
	}

	r.logger.Debug("ClickHouse ping successful",
		slog.Duration("duration", duration),
	)
	return nil
}

// InsertBatch inserts a batch of events into the fanfinity.match_events table.
// Uses ClickHouse batch insert for optimal performance.
func (r *ClickHouseRepository) InsertBatch(ctx context.Context, events []*domain.Event) error {
	if len(events) == 0 {
		return nil
	}

	startTime := time.Now()

	// Prepare batch insert
	batch, err := r.conn.PrepareBatch(ctx, `
		INSERT INTO fanfinity.match_events (
			event_id,
			match_id,
			event_type,
			team_id,
			player_id,
			metadata,
			timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		r.logger.Error("failed to prepare batch insert",
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("insert_batch_prepare").Inc()
		return fmt.Errorf("failed to prepare batch insert: %w", err)
	}

	// Append each event to the batch
	for _, event := range events {
		if event == nil {
			continue
		}

		// Convert TeamID to string for ClickHouse schema
		teamIDStr := strconv.Itoa(event.TeamID)

		// Handle nullable player_id - use pointer for nullable string
		var playerID *string
		if event.PlayerID != "" {
			playerID = &event.PlayerID
		}

		// Serialize metadata to JSON string
		metadataJSON := event.MetadataJSON()

		err = batch.Append(
			event.EventID,
			event.MatchID,
			string(event.EventType),
			teamIDStr,
			playerID,
			metadataJSON,
			event.Timestamp,
		)
		if err != nil {
			r.logger.Warn("failed to append event to batch",
				slog.String("event_id", event.EventID.String()),
				slog.String("error", err.Error()),
			)
			// Continue with other events rather than failing the entire batch
			continue
		}
	}

	// Send the batch
	err = batch.Send()
	duration := time.Since(startTime)

	// Record metrics
	clickhouseQueryDuration.WithLabelValues("insert_batch").Observe(duration.Seconds())
	clickhouseBatchSize.WithLabelValues().Observe(float64(len(events)))

	if err != nil {
		r.logger.Error("failed to send batch insert",
			slog.Int("batch_size", len(events)),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("insert_batch_send").Inc()
		return fmt.Errorf("failed to send batch insert: %w", err)
	}

	r.logger.Debug("successfully inserted batch",
		slog.Int("batch_size", len(events)),
		slog.Duration("duration", duration),
	)
	clickhouseEventsInserted.Add(float64(len(events)))

	return nil
}

// GetMatchMetrics retrieves aggregated metrics for a specific match.
// Queries the fanfinity.match_metrics materialized view and aggregates events by type.
func (r *ClickHouseRepository) GetMatchMetrics(ctx context.Context, matchID string) (*domain.MatchMetrics, error) {
	if matchID == "" {
		return nil, fmt.Errorf("matchID cannot be empty")
	}

	startTime := time.Now()

	// Query for basic metrics from aggregated view
	metrics := domain.NewMatchMetrics(matchID)

	// Query total events, goals, yellow cards, red cards, and time range
	row := r.conn.QueryRow(ctx, `
		SELECT
			count(*) as total_events,
			countIf(event_type = 'goal') as goals,
			countIf(event_type = 'yellow_card') as yellow_cards,
			countIf(event_type = 'red_card') as red_cards,
			min(timestamp) as first_event_at,
			max(timestamp) as last_event_at
		FROM fanfinity.match_events
		WHERE match_id = ?
	`, matchID)

	var totalEvents, goals, yellowCards, redCards int64
	var firstEventAt, lastEventAt time.Time

	err := row.Scan(&totalEvents, &goals, &yellowCards, &redCards, &firstEventAt, &lastEventAt)
	if err != nil {
		duration := time.Since(startTime)
		r.logger.Error("failed to query match metrics",
			slog.String("match_id", matchID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("get_match_metrics").Inc()
		clickhouseQueryDuration.WithLabelValues("get_match_metrics").Observe(duration.Seconds())
		return nil, fmt.Errorf("failed to query match metrics: %w", err)
	}

	// If no events found, return nil
	if totalEvents == 0 {
		duration := time.Since(startTime)
		clickhouseQueryDuration.WithLabelValues("get_match_metrics").Observe(duration.Seconds())
		return nil, nil
	}

	metrics.TotalEvents = totalEvents
	metrics.Goals = goals
	metrics.YellowCards = yellowCards
	metrics.RedCards = redCards

	// Set time pointers only if we have events
	if !firstEventAt.IsZero() {
		metrics.FirstEventAt = &firstEventAt
	}
	if !lastEventAt.IsZero() {
		metrics.LastEventAt = &lastEventAt
	}

	// Query events by type
	rows, err := r.conn.Query(ctx, `
		SELECT event_type, count(*) as event_count
		FROM fanfinity.match_events
		WHERE match_id = ?
		GROUP BY event_type
		ORDER BY event_count DESC
	`, matchID)
	if err != nil {
		duration := time.Since(startTime)
		r.logger.Error("failed to query events by type",
			slog.String("match_id", matchID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("get_match_metrics_by_type").Inc()
		clickhouseQueryDuration.WithLabelValues("get_match_metrics").Observe(duration.Seconds())
		return nil, fmt.Errorf("failed to query events by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var eventType string
		var eventCount int64
		if err := rows.Scan(&eventType, &eventCount); err != nil {
			r.logger.Warn("failed to scan event type row",
				slog.String("error", err.Error()),
			)
			continue
		}
		metrics.EventsByType[eventType] = eventCount
	}

	if err := rows.Err(); err != nil {
		duration := time.Since(startTime)
		r.logger.Error("error iterating events by type rows",
			slog.String("match_id", matchID),
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("get_match_metrics_by_type").Inc()
		clickhouseQueryDuration.WithLabelValues("get_match_metrics").Observe(duration.Seconds())
		return nil, fmt.Errorf("error iterating events by type: %w", err)
	}

	// Query for peak engagement minute
	peakRow := r.conn.QueryRow(ctx, `
		SELECT
			toStartOfMinute(timestamp) as minute,
			count(*) as event_count
		FROM fanfinity.match_events
		WHERE match_id = ?
		GROUP BY minute
		ORDER BY event_count DESC
		LIMIT 1
	`, matchID)

	var peakMinute time.Time
	var peakCount int64
	err = peakRow.Scan(&peakMinute, &peakCount)
	if err == nil && peakCount > 0 {
		metrics.PeakMinute = &domain.PeakEngagement{
			Minute:     peakMinute,
			EventCount: peakCount,
		}
	}

	duration := time.Since(startTime)
	clickhouseQueryDuration.WithLabelValues("get_match_metrics").Observe(duration.Seconds())

	r.logger.Debug("successfully retrieved match metrics",
		slog.String("match_id", matchID),
		slog.Int64("total_events", totalEvents),
		slog.Duration("duration", duration),
	)

	return metrics, nil
}

// GetEventsPerMinute retrieves events aggregated by minute for a specific match.
// Uses the fanfinity.events_per_minute materialized view if available.
func (r *ClickHouseRepository) GetEventsPerMinute(ctx context.Context, matchID string) ([]domain.EventsPerMinute, error) {
	if matchID == "" {
		return nil, fmt.Errorf("matchID cannot be empty")
	}

	startTime := time.Now()

	rows, err := r.conn.Query(ctx, `
		SELECT
			toStartOfMinute(timestamp) as minute,
			event_type,
			count(*) as event_count
		FROM fanfinity.match_events
		WHERE match_id = ?
		GROUP BY minute, event_type
		ORDER BY minute ASC, event_type ASC
	`, matchID)
	if err != nil {
		duration := time.Since(startTime)
		r.logger.Error("failed to query events per minute",
			slog.String("match_id", matchID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("get_events_per_minute").Inc()
		clickhouseQueryDuration.WithLabelValues("get_events_per_minute").Observe(duration.Seconds())
		return nil, fmt.Errorf("failed to query events per minute: %w", err)
	}
	defer rows.Close()

	var results []domain.EventsPerMinute
	for rows.Next() {
		var epm domain.EventsPerMinute
		if err := rows.Scan(&epm.Minute, &epm.EventType, &epm.EventCount); err != nil {
			r.logger.Warn("failed to scan events per minute row",
				slog.String("error", err.Error()),
			)
			continue
		}
		results = append(results, epm)
	}

	if err := rows.Err(); err != nil {
		duration := time.Since(startTime)
		r.logger.Error("error iterating events per minute rows",
			slog.String("match_id", matchID),
			slog.String("error", err.Error()),
		)
		clickhouseQueryErrors.WithLabelValues("get_events_per_minute").Inc()
		clickhouseQueryDuration.WithLabelValues("get_events_per_minute").Observe(duration.Seconds())
		return nil, fmt.Errorf("error iterating events per minute: %w", err)
	}

	duration := time.Since(startTime)
	clickhouseQueryDuration.WithLabelValues("get_events_per_minute").Observe(duration.Seconds())

	r.logger.Debug("successfully retrieved events per minute",
		slog.String("match_id", matchID),
		slog.Int("result_count", len(results)),
		slog.Duration("duration", duration),
	)

	return results, nil
}

// Close closes the ClickHouse connection.
func (r *ClickHouseRepository) Close() error {
	if r.conn == nil {
		return nil
	}
	return r.conn.Close()
}

// ConnectionConfig holds configuration for ClickHouse connection.
type ConnectionConfig struct {
	Hosts           []string
	Database        string
	Username        string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	Debug           bool
}

// DefaultConnectionConfig returns default ClickHouse connection configuration.
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		Hosts:           []string{"localhost:9000"},
		Database:        "fanfinity",
		Username:        "default",
		Password:        "",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		DialTimeout:     10 * time.Second,
		ReadTimeout:     30 * time.Second,
		Debug:           false,
	}
}

// NewConnection creates a new ClickHouse connection with the given configuration.
func NewConnection(cfg ConnectionConfig) (driver.Conn, error) {
	opts := &clickhouse.Options{
		Addr: cfg.Hosts,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     cfg.DialTimeout,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		Debug:           cfg.Debug,
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open ClickHouse connection: %w", err)
	}

	return conn, nil
}

// NewConnectionFromDSN creates a new ClickHouse connection from a DSN string.
// DSN format: clickhouse://user:password@host:port/database
func NewConnectionFromDSN(dsn string) (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open ClickHouse connection from DSN: %w", err)
	}
	return conn, nil
}
