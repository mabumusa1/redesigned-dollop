# /plan-api - API Design Planning

## Role

You are an **API Designer Agent** specializing in high-performance REST APIs for real-time event processing.

## Task

Design the REST API for the Fanfinity analytics microservice.

## Required Endpoints

From `context.md`:

```
POST /api/events
- Accept match events (goal, card, substitution, match_start, match_end)
- Validate event data
- Process asynchronously
- Return 202 Accepted with event ID

GET /api/matches/{matchId}/metrics
- Total events count
- Events by type breakdown
- Peak engagement periods (events per minute)
- Response time percentiles (p50, p95, p99)

GET /metrics
- Prometheus-compatible metrics
- Request rates, error rates, latency
- Business metrics (events processed, matches active)
```

## Deep Reasoning Questions

1. **Event Schema:** What is the optimal request/response schema for POST /api/events handling 1000 req/s?

2. **Batch Support:** Should we support batch ingestion in a single request?

3. **Error Handling:** What error response format ensures clear feedback without exposing internals?

4. **Rate Limiting:** Should we implement rate limiting for 1000 req/s peaks? What strategy?

5. **Versioning:** How to version the API for future evolution?

## Design Principles

- Return 202 Accepted for async operations
- Use proper HTTP semantics
- Design for idempotency
- Include request IDs for tracing
- Follow RFC 7807 for error responses

## Event Data Model

```json
{
  "eventId": "uuid",
  "matchId": "string",
  "eventType": "goal|yellow_card|red_card|substitution|match_start|match_end",
  "timestamp": "ISO 8601",
  "teamId": "string",
  "playerId": "string (optional)",
  "metadata": {}
}
```

## Output Format

For each endpoint, provide:
- HTTP method and path
- Request schema
- Response schema
- Status codes
- Example requests/responses

## Artifacts to Generate

After approval, create:
- `output/docs/api/openapi.yaml`
- `output/docs/api/API_DESIGN.md`
- `output/internal/api/router.go`
- `output/internal/api/handlers/events.go`
- `output/internal/api/handlers/metrics.go`
