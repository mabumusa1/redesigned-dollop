# /plan-all - Full Architecture Planning

Run all planning agents in sequence for the Fanfinity analytics microservice.

## Workflow

1. **System Architecture** - High-level design decisions
2. **API Design** - REST endpoints and OpenAPI spec
3. **Data Modeling** - ClickHouse schema
4. **DevOps Planning** - Docker, CI/CD

## Instructions

When this command is invoked:

1. Read the project context from `context.md`
2. Load the reasoning prompts from `.claude/prompts/`
3. Execute each agent in order, presenting decisions for approval
4. Generate all artifacts to `./output/`

## Execution

```
For each agent:
1. Present the deep reasoning analysis
2. Show key decisions with trade-offs
3. Ask for approval before proceeding
4. Generate artifacts if approved
```

## Constraints

- Storage: ClickHouse ONLY
- Focus: Bulk HTTP ingestion
- Pattern: Follow bulker ingestion patterns
- Performance: 1000 req/s, sub-200ms latency
