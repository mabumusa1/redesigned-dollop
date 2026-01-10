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
- **Focus:** Bulk HTTP ingestion only (not source-to-destination syncing)
- **Pattern:** Follow bulker ingestion patterns

## Deep Reasoning Questions

Think through each question systematically:

1. **Architecture Pattern:** What is the optimal high-level architecture for handling 1000 req/s event ingestion with sub-200ms latency?

2. **Component Structure:** How should we structure service components to achieve zero data loss during traffic spikes?

3. **Buffering Strategy:** What buffering approach between HTTP ingestion and ClickHouse writes?

4. **Scalability:** How should the system scale horizontally when traffic exceeds 1000 req/s?

5. **Data Flow:** What is the optimal flow from event ingestion to metrics query?

6. **Backpressure:** How to handle backpressure when ClickHouse is slow or unavailable?

## Bulker Patterns to Follow

```go
// Context pattern - central dependency container
type Context struct {
    config        *Config
    clickhouse    clickhouse.Conn
    eventBuffer   *EventBuffer
    server        *http.Server
}

// Async ingestion - never block on writes
func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
    // Validate, buffer, return 202 immediately
    buffer.Add(event)
    w.WriteHeader(http.StatusAccepted)
}

// Graceful lifecycle
func (c *Context) InitContext() error { ... }
func (c *Context) Shutdown() error { ... }  // Flush buffer first
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
- `output/docs/architecture/ADR-001-architecture-pattern.md`
- `output/internal/app/context.go`
- `output/internal/app/config.go`
