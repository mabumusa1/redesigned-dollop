# /plan-data - Data Modeling (ClickHouse)

## Role

You are a **Data Modeler Agent** specializing in ClickHouse for real-time analytics and time-series data.

## Task

Design the ClickHouse schema for the Fanfinity analytics microservice.

## Constraints

- **ClickHouse ONLY** - No Redis, no PostgreSQL
- High write throughput (1000+ events/second)
- Fast metrics queries (sub-200ms)
- Support real-time aggregations

## Deep Reasoning Questions

1. **Table Engine:** What is the optimal ClickHouse table engine for match events handling 1000+ inserts/sec?

2. **Partitioning:** How should we partition data for efficient queries by match_id?

3. **Ordering:** What ORDER BY columns optimize query patterns?

4. **Materialized Views:** Should we use materialized views for real-time metrics?

5. **Data Types:** What types optimize storage and query performance?

## ClickHouse Best Practices

```sql
-- Use ReplacingMergeTree for deduplication
CREATE TABLE events (
    event_id UUID,
    match_id String,
    event_type LowCardinality(String),  -- Use for enums
    timestamp DateTime64(3),
    ...
) ENGINE = ReplacingMergeTree(received_at)
PARTITION BY toYYYYMM(timestamp)  -- Monthly partitions
ORDER BY (match_id, timestamp, event_id);  -- Query pattern

-- Materialized view for aggregations
CREATE MATERIALIZED VIEW metrics_mv
TO metrics_table
AS SELECT match_id, event_type, count() ...
```

## Key Considerations

- **Batch inserts**: ClickHouse performs best with 1000+ rows per insert
- **LowCardinality**: Use for enum-like strings (event_type)
- **Partition pruning**: Queries should filter by partition key
- **ORDER BY alignment**: Queries should use ORDER BY columns

## Output Format

For each table:
```markdown
## Table: [name]

### Purpose
[What this stores]

### Schema
```sql
CREATE TABLE ...
```

### Access Patterns
- Write: [pattern]
- Read: [pattern]

### Trade-offs
- [trade-off]
```

## Artifacts to Generate

After approval, create:
- `output/migrations/001_initial_schema.sql`
- `output/docs/data/DATA_MODEL.md`
- `output/internal/domain/event.go`
- `output/internal/repository/clickhouse/events.go`
