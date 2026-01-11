# Fanfinity Analytics - Comprehensive Testing Plan

## Overview

This document provides a comprehensive testing plan for the Fanfinity Analytics microservice. The plan covers all testing phases from unit tests to load testing, serving as a guideline to verify the system works as expected.

### System Architecture

```
HTTP Client --> POST /api/events --> API Server (port 8080)
                                          |
                                          v
                                     Kafka Producer --> Kafka (port 9092)
                                                            |
                                                            v (topics: events, retry, dead)
                                                    Batch Consumer --> ClickHouse (port 8123/9000)
                                                            |
                                                            v
HTTP Client <-- GET /api/matches/{id}/metrics <-- API Server <-- ClickHouse
```

### Key Files

| Component      | Location                                      |
| -------------- | --------------------------------------------- |
| API Server     | `/workspace/cmd/server/main.go`               |
| Consumer       | `/workspace/cmd/consumer/main.go`             |
| Handler Tests  | `/workspace/internal/api/handler_test.go`     |
| Domain Tests   | `/workspace/internal/domain/event_test.go`    |
| Simulation     | `/workspace/simulation/simulate_match.py`     |
| Docker Compose | `/workspace/.devcontainer/docker-compose.yml` |

---

## 1. Unit Testing

### 1.1 Test Execution

**Commands:**

```bash
# Run all unit tests
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html

# Run with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./internal/api/...
go test -bench=. ./internal/domain/...
```

**Pass/Fail Criteria:**
| Metric | Pass | Fail |
|--------|------|------|
| Test Pass Rate | 100% | Any failure |
| Coverage | >= 70% | < 70% |
| Race Conditions | None | Any detected |

### 1.2 Coverage Targets

| Package             | Target | Critical Functions                           |
| ------------------- | ------ | -------------------------------------------- |
| internal/api        | >= 80% | IngestEvent, GetMatchMetrics, HealthCheck    |
| internal/domain     | >= 85% | ToEvent, MetadataJSON, EventFromKafkaMessage |
| internal/kafka      | >= 60% | Produce, ProduceBatch                        |
| internal/repository | >= 50% | InsertBatch, GetMatchMetrics                 |

---

## 2. Integration Testing

### 2.1 Prerequisites Check

```bash
# Check Docker services status
cd /workspace/.devcontainer && docker-compose ps

# Verify Kafka
docker-compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:29092 --list

# Verify ClickHouse
docker-compose exec clickhouse clickhouse-client --query "SELECT 1"

# Check ports
nc -z localhost 9092 && echo "Kafka OK"
nc -z localhost 8123 && echo "ClickHouse HTTP OK"
```

### 2.2 Kafka Integration

```bash
# Create topics
docker-compose exec kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:29092 \
  --create --topic events --partitions 3 --replication-factor 1 --if-not-exists

# Test produce/consume
echo '{"test":"message"}' | docker-compose exec -T kafka \
  /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server localhost:29092 --topic events
```

### 2.3 ClickHouse Integration

```bash
# Verify database
curl -s 'http://localhost:8123' --data 'SHOW DATABASES' | grep fanfinity

# Verify table
curl -s 'http://localhost:8123' --data 'SHOW TABLES FROM fanfinity'

# Test insert/query
curl -s 'http://localhost:8123' --data "SELECT count(*) FROM fanfinity.match_events"
```

---

## 3. Component Testing

### 3.1 API Server

```bash
# Build and start
go build -o bin/server ./cmd/server && ./bin/server &

# Test endpoints
curl http://localhost:8080/health                    # 200 OK
curl http://localhost:8080/ready                     # 200 or 503
curl http://localhost:8080/metrics                   # Prometheus format
curl -X POST http://localhost:8080/api/events \
  -H "Content-Type: application/json" \
  -d '{"eventId":"uuid","matchId":"test","eventType":"goal","timestamp":"2024-01-01T12:00:00Z","teamId":1}'
```

### 3.2 Consumer

```bash
# Build and start
go build -o bin/consumer ./cmd/consumer && ./bin/consumer &

# Verify consumer group registration
docker-compose exec kafka /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:29092 --list
```

---

## 4. Functional Testing (End-to-End)

### 4.1 Complete Event Flow

```bash
# Start services
go run ./cmd/server &
go run ./cmd/consumer &

# Send events
MATCH_ID="functional-test-$(date +%s)"
for i in {1..10}; do
  curl -s -X POST http://localhost:8080/api/events \
    -H "Content-Type: application/json" \
    -d "{\"eventId\":\"$(uuidgen)\",\"matchId\":\"${MATCH_ID}\",\"eventType\":\"pass\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"teamId\":1}"
done

# Wait for batch flush
sleep 10

# Verify in ClickHouse
curl -s 'http://localhost:8123' --data "SELECT count(*) FROM fanfinity.match_events WHERE match_id = '${MATCH_ID}'"

# Query metrics API
curl -s "http://localhost:8080/api/matches/${MATCH_ID}/metrics" | jq .
```

**Pass/Fail Criteria:**
| Phase | Pass | Fail |
|-------|------|------|
| Event Ingestion | All return 202 | Any non-202 |
| ClickHouse Count | Matches sent | Discrepancy |
| Metrics Query | Valid JSON | 404 or wrong counts |

---

## 5. Load Testing

### 5.1 Tools

```bash
go install github.com/rakyll/hey@latest
go install github.com/tsenart/vegeta@latest
```

### 5.2 Target Load (1000 req/s)

```bash
# Prepare payload
cat > /tmp/event.json << 'EOF'
{"eventId":"550e8400-e29b-41d4-a716-446655440001","matchId":"load-test","eventType":"pass","timestamp":"2024-01-01T12:00:00Z","teamId":1}
EOF

# 1000 req/s for 30 seconds
hey -n 30000 -c 200 -q 5 -m POST \
  -H "Content-Type: application/json" \
  -D /tmp/event.json \
  http://localhost:8080/api/events
```

**Pass/Fail Criteria:**
| Metric | Target | Pass | Fail |
|--------|--------|------|------|
| Throughput | >= 1000 req/s | >= 950 | < 800 |
| p50 Latency | < 100ms | < 100ms | >= 200ms |
| p99 Latency | < 200ms | < 200ms | >= 500ms |
| Error Rate | 0% | < 0.1% | >= 1% |

### 5.3 Spike Test

```bash
# Baseline (100 req/s)
hey -n 1000 -c 20 -q 5 ...

# Spike (2000 req/s)
hey -n 20000 -c 400 -q 5 ...

# Return to baseline
hey -n 1000 -c 20 -q 5 ...
```

---

## 6. Reliability Testing

### 6.1 Error Handling

```bash
# Invalid JSON -> 400
curl -X POST http://localhost:8080/api/events -d 'not valid json'

# Missing fields -> 400
curl -X POST http://localhost:8080/api/events -d '{"matchId":"test"}'

# Invalid UUID -> 400
curl -X POST http://localhost:8080/api/events -d '{"eventId":"not-uuid",...}'
```

### 6.2 Kafka Failure

```bash
# Stop Kafka
docker-compose stop kafka

# Event should return 503
curl -X POST http://localhost:8080/api/events ...

# Restart and verify recovery
docker-compose start kafka
```

### 6.3 ClickHouse Failure

```bash
# Stop ClickHouse
docker-compose stop clickhouse

# Events should go to retry topic
# Restart and verify retry processing
docker-compose start clickhouse
```

### 6.4 Graceful Shutdown

```bash
# Send SIGTERM during load
kill -SIGTERM $SERVER_PID

# Verify:
# - In-flight requests complete
# - No dropped events
# - Log shows "shutdown complete"
```

---

## 7. Health & Observability

### 7.1 Health Endpoints

| Endpoint | Healthy       | Unhealthy   |
| -------- | ------------- | ----------- |
| /health  | 200 always    | N/A         |
| /ready   | 200 with deps | 503 without |

### 7.2 Prometheus Metrics

```bash
curl -s http://localhost:8080/metrics | promtool check metrics
```

**Key Metrics:**

- `http_requests_total{method,path,status}`
- `http_request_duration_seconds{method,path}`
- `fanfinity_kafka_producer_messages_produced_total`
- `fanfinity_events_ingested_total{event_type}`

---

## 8. Test Execution Checklist

### Pre-Test Setup

- [ ] Docker Compose services running
- [ ] Go modules downloaded
- [ ] Load testing tools installed

### Unit Tests (Run Daily)

- [ ] `go test -v ./...` passes
- [ ] Coverage >= 70%
- [ ] No race conditions

### Integration Tests (After Changes)

- [ ] Kafka connectivity verified
- [ ] ClickHouse connectivity verified
- [ ] Topics exist (events, retry, dead)

### Functional Tests (Before Release)

- [ ] End-to-end event flow works
- [ ] Events appear in ClickHouse
- [ ] Metrics API returns correct data

### Load Tests (Before Release)

- [ ] 1000 req/s target achieved
- [ ] p99 < 200ms
- [ ] No memory leaks

### Reliability Tests (Before Release)

- [ ] Error handling correct
- [ ] Kafka failure handled
- [ ] ClickHouse retry works
- [ ] Graceful shutdown works

---

## 9. Automated Test Script

```bash
#!/bin/bash
# /workspace/scripts/run_all_tests.sh

set -e
echo "=== Fanfinity Analytics Test Suite ==="

# Phase 1: Unit Tests
go test -v ./... || exit 1

# Phase 2: Coverage
go test -coverprofile=coverage.out ./...
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
[ $(echo "$COVERAGE >= 70" | bc -l) -eq 1 ] || exit 1

# Phase 3: Integration
curl -s 'http://localhost:8123' --data 'SELECT 1' || exit 1

# Phase 4: Functional
curl -s http://localhost:8080/health | grep -q "healthy" || exit 1

echo "=== All Tests Passed! ==="
```

---

## Summary

| Phase         | Tests        | Focus                          |
| ------------- | ------------ | ------------------------------ |
| Unit Testing  | ~70 tests    | Domain logic, handlers         |
| Integration   | 4 checks     | Kafka, ClickHouse connectivity |
| Component     | 5 tests      | API server, consumer startup   |
| Functional    | 2 scenarios  | End-to-end flow                |
| Load          | 4 test types | Performance targets            |
| Reliability   | 4 scenarios  | Failure handling               |
| Observability | 3 categories | Health, metrics, logs          |

**Critical Success Metrics:**

- Unit test pass rate: 100%
- Coverage: >= 70%
- Performance: >= 1000 req/s, < 200ms p99
- Reliability: Zero data loss, graceful failure handling
