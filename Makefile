.PHONY: build run test clean docker-build docker-run lint fmt help

# Variables
BINARY_NAME=alert-bridge
DOCKER_IMAGE=alert-bridge
DOCKER_TAG=latest
GO=go

# Build the binary
build:
	$(GO) build -o bin/$(BINARY_NAME) ./cmd/alert-bridge

# Run the application
run:
	$(GO) run ./cmd/alert-bridge

# Run tests
test:
	$(GO) test -v ./...

# Run tests with coverage
test-coverage:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Run Docker container
docker-run:
	docker run --rm -p 8080:8080 \
		-e SLACK_BOT_TOKEN \
		-e SLACK_SIGNING_SECRET \
		-e SLACK_CHANNEL_ID \
		-e PAGERDUTY_API_TOKEN \
		-e PAGERDUTY_ROUTING_KEY \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Run linter
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GO) fmt ./...
	goimports -w .

# Tidy dependencies
tidy:
	$(GO) mod tidy

# Download dependencies
deps:
	$(GO) mod download

# Generate mocks (requires mockery)
mocks:
	mockery --all --dir=internal/domain/repository --output=internal/mocks --outpkg=mocks

# Development mode with hot reload (requires air)
dev:
	air

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  run            - Run the application"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  tidy           - Tidy dependencies"
	@echo "  deps           - Download dependencies"
	@echo "  mocks          - Generate mocks"
	@echo "  dev            - Development mode with hot reload"
	@echo "  help           - Show this help"
