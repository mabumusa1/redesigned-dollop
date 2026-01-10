-- Fanfinity Analytics - ClickHouse Schema Initialization
-- This runs automatically when the container starts

CREATE DATABASE IF NOT EXISTS fanfinity;

-- Match events table - main event storage
CREATE TABLE IF NOT EXISTS fanfinity.match_events
(
    event_id UUID DEFAULT generateUUIDv4(),
    match_id String,
    event_type LowCardinality(String),  -- goal, yellow_card, red_card, substitution, match_start, match_end
    team_id String,
    player_id Nullable(String),
    metadata String DEFAULT '{}',  -- JSON string for extensibility
    timestamp DateTime64(3),
    ingested_at DateTime64(3) DEFAULT now64(3),

    -- Indexes for common queries
    INDEX idx_event_type event_type TYPE set(10) GRANULARITY 1,
    INDEX idx_team team_id TYPE bloom_filter() GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (match_id, timestamp, event_id)
TTL toDateTime(timestamp) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Materialized view for per-minute aggregations (engagement metrics)
CREATE TABLE IF NOT EXISTS fanfinity.events_per_minute
(
    match_id String,
    minute DateTime,
    event_type LowCardinality(String),
    event_count UInt64
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(minute)
ORDER BY (match_id, minute, event_type);

CREATE MATERIALIZED VIEW IF NOT EXISTS fanfinity.events_per_minute_mv
TO fanfinity.events_per_minute
AS SELECT
    match_id,
    toStartOfMinute(timestamp) AS minute,
    event_type,
    count() AS event_count
FROM fanfinity.match_events
GROUP BY match_id, minute, event_type;

-- Aggregated match metrics (for fast GET /matches/{matchId}/metrics)
CREATE TABLE IF NOT EXISTS fanfinity.match_metrics
(
    match_id String,
    total_events UInt64,
    goals UInt64,
    yellow_cards UInt64,
    red_cards UInt64,
    substitutions UInt64,
    first_event_at Nullable(DateTime64(3)),
    last_event_at Nullable(DateTime64(3))
)
ENGINE = SummingMergeTree()
ORDER BY match_id;

CREATE MATERIALIZED VIEW IF NOT EXISTS fanfinity.match_metrics_mv
TO fanfinity.match_metrics
AS SELECT
    match_id,
    count() AS total_events,
    countIf(event_type = 'goal') AS goals,
    countIf(event_type = 'yellow_card') AS yellow_cards,
    countIf(event_type = 'red_card') AS red_cards,
    countIf(event_type = 'substitution') AS substitutions,
    min(timestamp) AS first_event_at,
    max(timestamp) AS last_event_at
FROM fanfinity.match_events
GROUP BY match_id;
