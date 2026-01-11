.PHONY: build test run-server run-consumer clean lint tidy docker-build docker-up docker-down

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Build targets
build:
	@echo "Building server and consumer..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/server ./cmd/server
	go build $(LDFLAGS) -o bin/consumer ./cmd/consumer
	@echo "Build complete: bin/server, bin/consumer"

build-server:
	@echo "Building server..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/server ./cmd/server
	@echo "Build complete: bin/server"

build-consumer:
	@echo "Building consumer..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/consumer ./cmd/consumer
	@echo "Build complete: bin/consumer"

# Test targets
test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-short:
	go test -short ./...

# Run targets
run-server:
	go run ./cmd/server

run-consumer:
	go run ./cmd/consumer

# Development helpers
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

tidy:
	go mod tidy
	go mod verify

fmt:
	go fmt ./...

vet:
	go vet ./...

# Docker targets
docker-build:
	docker build -t fanfinity-server:$(VERSION) --target server .
	docker build -t fanfinity-consumer:$(VERSION) --target consumer .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Database migrations
migrate-up:
	@echo "Running migrations..."
	@if [ -f migrations/*.sql ]; then \
		for f in migrations/*.sql; do \
			echo "Applying $$f..."; \
			docker-compose exec -T clickhouse clickhouse-client --multiquery < "$$f"; \
		done \
	fi

# Generate targets
generate:
	go generate ./...

# All-in-one development setup
dev-setup: tidy lint test
	@echo "Development setup complete!"

# Quick check before committing
check: fmt vet lint test-short
	@echo "All checks passed!"

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build server and consumer binaries"
	@echo "  build-server   - Build only the server binary"
	@echo "  build-consumer - Build only the consumer binary"
	@echo "  test           - Run all tests with verbose output"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-short     - Run short tests only"
	@echo "  run-server     - Run the HTTP API server"
	@echo "  run-consumer   - Run the Kafka consumer"
	@echo "  clean          - Remove build artifacts"
	@echo "  lint           - Run golangci-lint"
	@echo "  lint-fix       - Run golangci-lint with auto-fix"
	@echo "  tidy           - Run go mod tidy and verify"
	@echo "  fmt            - Format code with go fmt"
	@echo "  vet            - Run go vet"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start Docker Compose services"
	@echo "  docker-down    - Stop Docker Compose services"
	@echo "  docker-logs    - Tail Docker Compose logs"
	@echo "  migrate-up     - Run database migrations"
	@echo "  dev-setup      - Run tidy, lint, and tests"
	@echo "  check          - Quick pre-commit checks"
	@echo "  help           - Show this help message"
