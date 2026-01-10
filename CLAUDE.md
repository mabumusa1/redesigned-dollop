# Fanfinity Analytics - Claude CLI Project

## Project Overview

Real-time fan engagement analytics microservice for processing match events during high-traffic scenarios.

**Read `context.md` for full requirements.**

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
"Run /plan-architecture focusing on the buffering strategy"
"Run /plan-data for ClickHouse schema"

# Iterative refinement
"Review the API design and add batch support"
"Update the ClickHouse schema for better query performance"
```

## Key Constraints

- **Storage:** ClickHouse ONLY (no Redis, no PostgreSQL)
- **Focus:** Bulk HTTP ingestion (not source-to-destination sync)
- **Performance:** 1000 req/s, sub-200ms latency
- **Reliability:** Zero data loss

## Architecture Patterns (from Bulker)

The planning follows [jitsucom/bulker](https://github.com/jitsucom/bulker) ingestion patterns:

1. **Context Pattern** - Central dependency container
2. **Async Ingestion** - Never block on writes, return 202 immediately
3. **Batch Writes** - Buffer events, write to ClickHouse in batches
4. **Graceful Lifecycle** - InitContext, Shutdown with buffer flush
5. **Prometheus Metrics** - p50, p95, p99 latency histograms

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
- `.claude/` - Agent configurations and prompts
- `.claude/commands/` - Planning command definitions
- `.claude/prompts/` - Deep reasoning prompt templates
