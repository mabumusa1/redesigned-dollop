# /plan-architecture - System Architecture Planning

## Role

You are a **System Architect Agent** specializing in high-performance microservices for real-time event processing.

## Task

Design the high-level architecture for the Fanfinity real-time fan engagement analytics microservice.

## Context

Read `context.md` for full requirements. Key points:
- 50,000+ concurrent users during major matches
- 1,000+ requests/second during goal spikes
- Sub-200ms API response times
- Zero data loss during traffic surges

## Constraints

- **Storage:** ClickHouse ONLY (no Redis, no PostgreSQL)
- **Message Queue:** Kafka for durable event buffering
- **Focus:** Bulk HTTP ingestion with Kafka-backed durability
- **Pattern:** Follow [jitsucom/bulker](https://github.com/jitsucom/bulker) ingestion patterns

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
   (immediate)       │ events  │  <- main topic
                     │ retry   │  <- failed events
                     │ dead    │  <- permanently failed
                     └─────────┘
```

## Deep Reasoning Questions

Think through each question systematically:

1. **Architecture Pattern:** What is the optimal high-level architecture for handling 1000 req/s event ingestion with sub-200ms latency?

2. **Kafka Topics:** How should we structure Kafka topics for events, retries, and dead-letter handling?

3. **Batch Consumer:** What batch size and flush interval for optimal ClickHouse writes?

4. **Scalability:** How should the system scale horizontally when traffic exceeds 1000 req/s?

5. **Data Flow:** What is the optimal flow from event ingestion to metrics query?

6. **Backpressure:** How to handle backpressure when ClickHouse is slow or unavailable?

## Bulker Patterns to Follow

```go
// Context pattern - central dependency container
type Context struct {
    config         *Config
    kafkaProducer  *kafka.Producer
    kafkaConsumer  *kafka.Consumer
    clickhouse     clickhouse.Conn
    topicManager   *TopicManager
    server         *http.Server
}

// Async ingestion - write to Kafka immediately, return 202
func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
    event := parseAndValidate(r)

    // Write to Kafka (durable) - this is the key to zero data loss
    err := h.producer.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{Topic: &topic},
        Key:            []byte(event.MatchID),
        Value:          eventBytes,
    }, nil)

    if err != nil {
        // Kafka unavailable - fail fast, let client retry
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{"eventId": event.ID})
}

// Batch consumer - reads from Kafka, writes to ClickHouse in batches
type BatchConsumer struct {
    consumer     *kafka.Consumer
    clickhouse   clickhouse.Conn
    batchSize    int           // e.g., 1000 events
    flushTimeout time.Duration // e.g., 5 seconds
}

func (c *BatchConsumer) Run() {
    batch := make([]Event, 0, c.batchSize)
    ticker := time.NewTicker(c.flushTimeout)

    for {
        select {
        case <-ticker.C:
            c.flush(batch)
            batch = batch[:0]
        default:
            msg, err := c.consumer.ReadMessage(100 * time.Millisecond)
            if err == nil {
                batch = append(batch, parseEvent(msg))
                if len(batch) >= c.batchSize {
                    c.flush(batch)
                    batch = batch[:0]
                }
            }
        }
    }
}

// Graceful lifecycle
func (c *Context) InitContext() error { ... }
func (c *Context) Shutdown() error {
    // 1. Stop accepting new requests
    // 2. Wait for in-flight Kafka produces
    // 3. Flush batch consumer
    // 4. Close connections
}
```

## Kafka Topic Strategy

```go
// Topic naming convention (following Bulker)
const (
    TopicEvents      = "fanfinity.events"           // Main events
    TopicEventsRetry = "fanfinity.events.retry"     // Failed events for retry
    TopicEventsDead  = "fanfinity.events.dead"      // Permanently failed
)

// Topic configuration
var TopicConfig = kafka.TopicConfig{
    NumPartitions:     3,              // Parallelism
    ReplicationFactor: 1,              // Dev: 1, Prod: 3
    RetentionMs:       48 * 60 * 60 * 1000, // 48 hours
}
```

## Output Format

For each decision, provide:

```markdown
## Decision: [Title]

### Problem
[What we're solving]

### Options Considered
1. [Option A] - Pros/Cons
2. [Option B] - Pros/Cons

### Selected Approach
[Choice and rationale]

### Trade-offs
- [Trade-off 1]

### Confidence: [X]%
```

## Artifacts to Generate

After approval, create:
- `output/docs/architecture/SYSTEM_DESIGN.md`
- `output/docs/architecture/ADR-001-kafka-durability.md`
- `output/internal/app/context.go`
- `output/internal/app/config.go`
- `output/internal/kafka/producer.go`
- `output/internal/kafka/consumer.go`
- `output/internal/kafka/topics.go`
