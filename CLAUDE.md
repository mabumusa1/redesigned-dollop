# Fanfinity Analytics - Claude CLI Project

## Project Overview

Real-time fan engagement analytics microservice for processing match events during high-traffic scenarios.

**Read `context.md` for full requirements.**

## Architecture

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

## Tech Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Language | Go 1.22 | High-performance services |
| Message Queue | Kafka | Zero data loss, durability |
| Database | ClickHouse | Real-time analytics |
| Metrics | Prometheus | Observability |
| Dashboards | Grafana | Visualization |

## Development Environment

The project uses a **devcontainer** with all dependencies pre-configured:

```bash
# Open in VS Code
code .
# Press F1 → "Dev Containers: Reopen in Container"
```

### Services Available

| Service | Port | URL |
|---------|------|-----|
| API Server | 8080 | http://localhost:8080 |
| Kafka | 9092 | localhost:9092 |
| Kafka UI | 8081 | http://localhost:8081 |
| ClickHouse HTTP | 8123 | http://localhost:8123 |
| Prometheus | 9090 | http://localhost:9090 |
| Grafana | 3000 | http://localhost:3000 |

## Planning Agents

This project uses specialized Claude agents for architecture planning. All agent configurations are in `.claude/`.

### Available Commands

| Command | Description |
|---------|-------------|
| `/plan-all` | Run full planning workflow |
| `/plan-architecture` | System architecture design |
| `/plan-api` | REST API design + OpenAPI |
| `/plan-data` | ClickHouse schema design |
| `/plan-devops` | Docker, CI/CD, observability |

### Usage

```bash
# Full planning session
"Run /plan-all for the analytics service"

# Specific planning
"Run /plan-architecture focusing on the Kafka consumer strategy"
"Run /plan-data for ClickHouse schema"

# Iterative refinement
"Review the API design and add batch support"
"Update the ClickHouse schema for better query performance"
```

## Key Constraints

- **Storage:** ClickHouse ONLY (no Redis, no PostgreSQL)
- **Message Queue:** Kafka for durable event buffering
- **Focus:** Bulk HTTP ingestion with Kafka-backed durability
- **Performance:** 1000 req/s, sub-200ms latency
- **Reliability:** Zero data loss

## Architecture Patterns (from Bulker)

The planning follows [jitsucom/bulker](https://github.com/jitsucom/bulker) ingestion patterns:

1. **Context Pattern** - Central dependency container
2. **Kafka Durability** - Write to Kafka immediately, return 202
3. **Batch Consumer** - Read from Kafka, batch write to ClickHouse
4. **Retry Topics** - Failed events go to retry topic with exponential backoff
5. **Dead Letter Queue** - Permanently failed events for investigation
6. **Graceful Lifecycle** - InitContext, Shutdown with proper cleanup
7. **Prometheus Metrics** - RED method + Kafka consumer lag

## Output Structure

Planning agents generate to `./output/`:

```
output/
├── docs/
│   ├── architecture/
│   │   ├── SYSTEM_DESIGN.md
│   │   └── ADR-*.md
│   ├── api/
│   │   └── openapi.yaml
│   └── data/
│       └── DATA_MODEL.md
├── internal/
│   ├── app/
│   ├── api/
│   ├── kafka/
│   ├── domain/
│   └── repository/
├── migrations/
├── Dockerfile
├── docker-compose.yml
└── .github/workflows/
```

## Deep Reasoning Mode

All planning agents use deep reasoning:

1. **State the problem** - Clear problem definition
2. **Analyze options** - 2-3 alternatives with trade-offs
3. **Reason through consequences** - What happens under load? Failures?
4. **Make decisions** - Clear rationale with confidence score

## Files

- `context.md` - Project requirements
- `.devcontainer/` - Development environment configuration
- `.claude/` - Agent configurations and prompts
- `.claude/commands/` - Planning command definitions
