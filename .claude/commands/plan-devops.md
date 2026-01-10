# /plan-devops - DevOps Planning

## Role

You are a **DevOps Planner Agent** specializing in containerized Go microservices with observability.

## Task

Design the Docker, CI/CD, and observability setup for the Fanfinity analytics microservice.

## Deep Reasoning Questions

1. **Dockerfile:** What is the optimal multi-stage build for minimal image size and security?

2. **docker-compose:** How to structure local development with ClickHouse?

3. **CI/CD:** What GitHub Actions workflow for lint, test, build, and deploy?

4. **Metrics:** What Prometheus metrics for 1000 req/s SLA monitoring?

5. **Health Checks:** What strategy for Kubernetes/Docker orchestration?

## Dockerfile Best Practices

```dockerfile
# Multi-stage build
FROM golang:1.22-alpine AS builder
# ... build static binary

FROM scratch  # Minimal runtime
COPY --from=builder /app /app
USER 65534:65534  # Non-root
```

## Prometheus Metrics (RED Method)

```go
// Rate
http_requests_total{method, endpoint, status}

// Errors
http_errors_total{method, endpoint, error_type}

// Duration
http_request_duration_seconds{method, endpoint}
// Buckets for p50, p95, p99: {0.01, 0.05, 0.1, 0.2, 0.5, 1}
```

## Business Metrics

```go
events_ingested_total{event_type, match_id}
events_buffer_size
clickhouse_batch_duration_seconds
clickhouse_batch_size
active_matches
```

## CI/CD Pipeline

```yaml
jobs:
  lint:     # golangci-lint
  test:     # go test -race -coverprofile
  build:    # CGO_ENABLED=0 go build
  docker:   # Build and push image
  deploy:   # (optional) Deploy to staging
```

## Output Format

Provide complete, ready-to-use files with explanations.

## Artifacts to Generate

After approval, create:
- `output/Dockerfile`
- `output/docker-compose.yml`
- `output/.github/workflows/ci.yml`
- `output/internal/observability/metrics.go`
- `output/docs/devops/OBSERVABILITY.md`
