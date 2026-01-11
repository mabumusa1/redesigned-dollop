# Fanfinity Analytics Service

Real-time fan engagement analytics microservice for processing live match events during high-traffic scenarios.

## Architecture Overview

```
┌─────────────┐    ┌─────────────┐    ┌─────────────────┐    ┌────────────┐
│   HTTP      │───▶│   Kafka     │───▶│  Batch Consumer │───▶│ ClickHouse │
│   Ingest    │    │   (durable) │    │                 │    │            │
└─────────────┘    └─────────────┘    └─────────────────┘    └────────────┘
      │                   │
      │              ┌────┴────┐
      ▼              │  Topics │
   202 Accepted      ├─────────┤
   (immediate)       │ events  │
                     │ retry   │
                     │ dead    │
                     └─────────┘
```

### Key Design Decisions

**1. Language: Go 1.22**
- High-performance, low-latency HTTP handling
- Excellent concurrency primitives for async processing
- Small memory footprint ideal for containerization
- Strong standard library reduces external dependencies

**2. Message Queue: Apache Kafka**
- Durable event buffering ensures zero data loss during traffic spikes
- Decouples ingestion from processing for better scalability
- Enables replay and recovery scenarios
- Handles 1000+ req/s with sub-10ms produce latency

**3. Database: ClickHouse**
- Purpose-built for real-time analytics on time-series data
- Column-oriented storage with 10x compression
- Sub-second queries on millions of events
- Materialized views for pre-aggregated metrics

**4. Async Processing Pattern**
- HTTP endpoint immediately returns 202 Accepted
- Events are durably stored in Kafka before acknowledgment
- Batch consumer writes to ClickHouse in optimized batches
- Retry and dead-letter topics handle failures gracefully

## API Documentation

### POST /api/events
Ingest a match event for processing.

**Request:**
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440000",
  "matchId": "match-123",
  "eventType": "goal",
  "timestamp": "2024-01-15T14:30:00Z",
  "teamId": 1,
  "playerId": "player-456",
  "metadata": {
    "minute": 45,
    "scorer": "Player Name"
  }
}
```

**Response (202 Accepted):**
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "accepted",
  "timestamp": "2024-01-15T14:30:00.123Z"
}
```

**Event Types:**
- `goal`, `shot`, `pass`, `foul`
- `yellow_card`, `red_card`
- `substitution`, `offside`
- `corner`, `free_kick`, `interception`

**Validation:**
- `eventId`: Valid UUID (required)
- `matchId`: Non-empty string (required)
- `eventType`: One of the valid types (required)
- `timestamp`: RFC3339 format (required)
- `teamId`: 1 or 2 (required)
- `playerId`: Optional string
- `metadata`: Optional JSON object

### GET /api/matches/{matchId}/metrics
Retrieve real-time engagement metrics for a match.

**Response (200 OK):**
```json
{
  "matchId": "match-123",
  "totalEvents": 1523,
  "eventsByType": {
    "pass": 892,
    "shot": 45,
    "goal": 3,
    "foul": 28
  },
  "goals": 3,
  "yellowCards": 4,
  "redCards": 1,
  "firstEventAt": "2024-01-15T14:00:00Z",
  "lastEventAt": "2024-01-15T15:45:00Z",
  "peakMinute": {
    "minute": "2024-01-15T14:45:00Z",
    "eventCount": 89
  },
  "responseTimePercentiles": {
    "p50": 12.5,
    "p95": 45.2,
    "p99": 98.7
  }
}
```

### GET /health
Liveness probe - always returns healthy if the service is running.

**Response (200 OK):**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T14:30:00Z"
}
```

### GET /ready
Readiness probe - checks dependency connectivity.

**Response (200 OK):**
```json
{
  "status": "ready",
  "timestamp": "2024-01-15T14:30:00Z",
  "checks": {
    "clickhouse": "healthy"
  }
}
```

### GET /metrics
Prometheus-compatible metrics endpoint.

**Key Metrics:**
- `http_requests_total{method,path,status}` - Request counts
- `http_request_duration_seconds{method,path}` - Latency histogram
- `fanfinity_events_ingested_total{event_type}` - Events by type
- `fanfinity_kafka_producer_messages_produced_total` - Kafka throughput
- `fanfinity_clickhouse_events_inserted_total` - Database writes

## Setup Instructions

### Prerequisites
- Go 1.22+
- Docker and Docker Compose
- Make (optional)

### Quick Start with DevContainer

1. Open the project in VS Code
2. Press `F1` → "Dev Containers: Reopen in Container"
3. Wait for the container to build and start all services
4. Run the services:

```bash
# Build binaries
go build -o bin/server ./cmd/server
go build -o bin/consumer ./cmd/consumer

# Start API server
./bin/server

# In another terminal, start consumer
./bin/consumer
```

### Environment Variables

See `.env.example` for all configuration options:

```bash
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Kafka
KAFKA_BOOTSTRAP_SERVERS=kafka:29092

# ClickHouse
CLICKHOUSE_HOST=clickhouse
CLICKHOUSE_PORT=9000
CLICKHOUSE_DATABASE=fanfinity
```

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run with race detection
go test -race ./...
```

## Project Structure

```
.
├── cmd/
│   ├── server/          # HTTP API server
│   └── consumer/        # Kafka batch consumer
├── internal/
│   ├── api/             # HTTP handlers and routing
│   ├── app/             # Application context and lifecycle
│   ├── domain/          # Domain models and validation
│   ├── kafka/           # Kafka producer and consumer
│   └── repository/      # ClickHouse data access
├── simulation/          # Python match simulator
└── .devcontainer/       # Development environment
```

## Scalability Approach

### How does this design handle 10x traffic?

1. **Horizontal Scaling**: Both API server and consumer can run multiple instances
   - API servers are stateless - add more behind a load balancer
   - Consumers use Kafka consumer groups for parallel processing

2. **Kafka Buffering**: Absorbs traffic spikes without backpressure
   - Producers write at 1000+ msg/s with low latency
   - Consumers process at their own pace
   - No data loss during overload

3. **Batch Processing**: ClickHouse writes are batched (1000 events or 5s)
   - Reduces write amplification
   - Maintains high throughput under load

### What would break first?

1. **Kafka Partition Limits**: With 3 partitions, max 3 consumers per group
   - Solution: Increase partitions for higher parallelism

2. **ClickHouse Write Throughput**: Single-node bottleneck at ~100K events/s
   - Solution: ClickHouse cluster with sharding

3. **Network Bandwidth**: High event volume with large metadata
   - Solution: Compression (already enabled: Snappy for Kafka, LZ4 for ClickHouse)

## Production Readiness

### What's implemented:
- Structured JSON logging with slog
- Prometheus metrics for RED method
- Health and readiness probes
- Graceful shutdown with connection draining
- Request timeout middleware
- Input validation and error handling
- Retry topics for failed events
- Dead letter queue for investigation

### What's missing for production:
- **Authentication/Authorization**: API keys or JWT tokens
- **Rate Limiting**: Per-client request throttling
- **TLS Termination**: HTTPS support (typically handled by ingress)
- **Distributed Tracing**: OpenTelemetry integration
- **Alerting Rules**: Prometheus alerting configuration
- **Backup Strategy**: ClickHouse backup and restore procedures
- **Multi-region**: Cross-datacenter replication

## Trade-offs Made

| Decision | Trade-off | Rationale |
|----------|-----------|-----------|
| ClickHouse over PostgreSQL | Less flexible queries | 100x faster analytics on time-series |
| Kafka over Redis Streams | More operational complexity | Stronger durability guarantees |
| Batch inserts over streaming | Higher latency (5s max) | 10x better throughput |
| Sync Kafka produce | Slightly higher latency | Guaranteed durability before 202 |

## License

MIT
