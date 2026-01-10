# /plan-devops - DevOps Planning

## Role

You are a **DevOps Planner Agent** specializing in containerized Go microservices with observability.

## Task

Design the Docker, CI/CD, and observability setup for the Fanfinity analytics microservice.

## Infrastructure Stack

```
┌─────────────────────────────────────────────────────────────────┐
│                        Docker Compose                            │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────┤
│    App      │   Kafka     │  ClickHouse │  Prometheus │ Grafana │
│   :8080     │   :9092     │    :8123    │    :9090    │  :3000  │
│             │   :9094     │    :9000    │             │         │
├─────────────┼─────────────┼─────────────┼─────────────┼─────────┤
│  Zookeeper  │  Kafka UI   │             │             │         │
│   :2181     │   :8081     │             │             │         │
└─────────────┴─────────────┴─────────────┴─────────────┴─────────┘
```

## Deep Reasoning Questions

1. **Dockerfile:** What is the optimal multi-stage build for minimal image size and security?

2. **docker-compose:** How to structure local development with Kafka, ClickHouse, and observability?

3. **CI/CD:** What GitHub Actions workflow for lint, test, build, and deploy?

4. **Metrics:** What Prometheus metrics for 1000 req/s SLA monitoring?

5. **Health Checks:** What strategy for Kubernetes/Docker orchestration?

6. **Kafka Monitoring:** How to monitor Kafka consumer lag and throughput?

## Dockerfile Best Practices

```dockerfile
# Multi-stage build
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/server

FROM scratch
COPY --from=builder /app /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/app"]
```

## docker-compose Services

```yaml
services:
  # Application
  app:
    build: .
    ports: ["8080:8080"]
    depends_on:
      kafka: { condition: service_healthy }
      clickhouse: { condition: service_healthy }
    environment:
      KAFKA_BOOTSTRAP_SERVERS: kafka:9094
      CLICKHOUSE_HOST: clickhouse

  # Message Queue (Zero Data Loss)
  zookeeper:
    image: bitnami/zookeeper:3.9
  kafka:
    image: bitnami/kafka:3.7
    ports: ["9092:9092"]

  # Analytics Database
  clickhouse:
    image: clickhouse/clickhouse-server:24.3-alpine
    ports: ["8123:8123", "9000:9000"]

  # Observability
  prometheus:
    image: prom/prometheus:v2.51.0
    ports: ["9090:9090"]
  grafana:
    image: grafana/grafana:10.4.1
    ports: ["3000:3000"]

  # Debugging Tools
  kafka-ui:
    image: provectuslabs/kafka-ui:v0.7.2
    ports: ["8081:8080"]
```

## Prometheus Metrics (RED Method)

```go
// Rate
http_requests_total{method, endpoint, status}

// Errors
http_errors_total{method, endpoint, error_type}

// Duration
http_request_duration_seconds{method, endpoint}
// Buckets for p50, p95, p99: {0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1}
```

## Business Metrics

```go
// Event ingestion
events_ingested_total{event_type, match_id}
events_produced_total{topic, status}  // Kafka produce success/failure

// Kafka consumer metrics
kafka_consumer_lag{topic, partition}
kafka_consumer_messages_total{topic}
kafka_consumer_batch_size

// ClickHouse batch metrics
clickhouse_batch_duration_seconds
clickhouse_batch_size
clickhouse_batch_errors_total

// Business metrics
active_matches_gauge
```

## Kafka Metrics to Monitor

```go
// Producer metrics
kafka_producer_queue_size
kafka_producer_messages_total{status}  // success, error
kafka_producer_latency_seconds

// Consumer metrics
kafka_consumer_lag{topic, partition}
kafka_consumer_commit_latency_seconds
kafka_consumer_rebalance_total
```

## Health Check Strategy

```go
// Liveness - is the process alive?
GET /health/live
// Returns 200 if process is running

// Readiness - can we accept traffic?
GET /health/ready
// Checks: Kafka producer connected, ClickHouse pingable

// Startup - has initialization completed?
GET /health/startup
// Checks: Kafka topics exist, ClickHouse tables created
```

## CI/CD Pipeline

```yaml
name: CI/CD

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: golangci/golangci-lint-action@v4

  test:
    runs-on: ubuntu-latest
    services:
      kafka:
        image: bitnami/kafka:3.7
      clickhouse:
        image: clickhouse/clickhouse-server:24.3-alpine
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test -race -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v4

  build:
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/build-push-action@v5
        with:
          context: .
          push: ${{ github.ref == 'refs/heads/main' }}
          tags: |
            ghcr.io/${{ github.repository }}:${{ github.sha }}
            ghcr.io/${{ github.repository }}:latest
```

## Output Format

Provide complete, ready-to-use files with explanations.

## Artifacts to Generate

After approval, create:
- `output/Dockerfile`
- `output/docker-compose.yml`
- `output/docker-compose.dev.yml` (devcontainer reference)
- `output/.github/workflows/ci.yml`
- `output/internal/observability/metrics.go`
- `output/internal/observability/health.go`
- `output/docs/devops/OBSERVABILITY.md`
- `output/docs/devops/RUNBOOK.md`
