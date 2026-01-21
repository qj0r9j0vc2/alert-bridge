# Development Guide

## Project Structure

```
alert-bridge/
├── cmd/alert-bridge/          # Application entry point (main.go)
├── config/                    # Configuration files (config.example.yaml)
├── docs/                      # Documentation
├── k8s/                       # Kubernetes manifests
├── scripts/                   # Build and deployment scripts
├── test/                      # Test fixtures and integration tests
├── internal/
│   ├── app/                  # Application bootstrap and dependency injection
│   │   ├── app.go           # Application struct and lifecycle
│   │   ├── bootstrap.go     # Initialization logic
│   │   ├── config.go        # Config manager setup
│   │   ├── storage.go       # Storage factory
│   │   ├── clients.go       # External client setup
│   │   ├── handlers.go      # Handler initialization
│   │   └── usecases.go      # Use case wiring
│   ├── adapter/              # HTTP handlers, DTOs, presenters
│   │   ├── dto/             # Data transfer objects (Alertmanager, Slack, PagerDuty)
│   │   ├── handler/         # HTTP request handlers
│   │   │   └── middleware/  # Authentication and observability middleware
│   │   └── presenter/       # Response formatting (Slack message blocks)
│   ├── domain/               # Business entities and interfaces
│   │   ├── entity/          # Core domain entities (Alert, AckEvent, SilenceMark, etc.)
│   │   ├── repository/      # Repository interfaces
│   │   ├── logger/          # Logger interface
│   │   └── errors/          # Domain errors
│   ├── infrastructure/       # External integrations
│   │   ├── config/          # Configuration loading and hot-reload
│   │   ├── persistence/     # Storage implementations
│   │   │   ├── memory/      # In-memory storage
│   │   │   ├── sqlite/      # SQLite storage with migrations
│   │   │   └── mysql/       # MySQL storage with migrations
│   │   ├── slack/           # Slack client (Socket Mode and HTTP)
│   │   ├── pagerduty/       # PagerDuty client
│   │   ├── server/          # HTTP server and router
│   │   ├── observability/   # Metrics and telemetry
│   │   └── resilience/      # Circuit breaker patterns
│   └── usecase/             # Business logic
│       ├── alert/           # Alert processing use case
│       ├── ack/             # Acknowledgment sync use case
│       ├── slack/           # Slack command and interaction use cases
│       └── pagerduty/       # PagerDuty webhook handling
├── Dockerfile               # Container build
├── Makefile                 # Build automation
└── go.mod                   # Go module definition

```

## Running Tests

### All Tests

```bash
go test ./...
```

### Verbose Output

```bash
go test -v ./...
```

### Specific Package

```bash
# SQLite tests
go test -v ./internal/infrastructure/persistence/sqlite/...

# MySQL tests
go test -v ./internal/infrastructure/persistence/mysql/...
```

### Integration Tests

```bash
# SQLite integration tests
go test -v ./internal/infrastructure/persistence/sqlite/... -run Integration

# MySQL integration tests (requires MySQL running)
go test -v ./internal/infrastructure/persistence/mysql/... -run Integration
```

### Benchmarks

```bash
# SQLite benchmarks
go test -bench=. ./internal/infrastructure/persistence/sqlite/...

# MySQL benchmarks (requires MySQL running)
go test -bench=. ./internal/infrastructure/persistence/mysql/...
```

### Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out
```

## Code Organization

### Clean Architecture Layers

Alert Bridge follows Clean Architecture principles:

1. **Domain Layer** (`internal/domain/`)
   - Core business entities (`entity/`): Alert, AckEvent, SilenceMark, SlackCommand
   - Repository interfaces (`repository/`): AlertRepository, AckEventRepository, SilenceRepository
   - No external dependencies

2. **Use Case Layer** (`internal/usecase/`)
   - Business logic orchestrating domain entities
   - `alert/` - ProcessAlertUseCase for handling incoming alerts
   - `ack/` - SyncAckUseCase for acknowledgment synchronization
   - `slack/` - QueryAlertStatusUseCase, SummarizeAlertsUseCase, HandleInteractionUseCase
   - `pagerduty/` - HandleWebhookUseCase for PagerDuty events

3. **Infrastructure Layer** (`internal/infrastructure/`)
   - External integrations (Slack, PagerDuty clients)
   - Persistence implementations (memory, SQLite, MySQL)
   - Configuration loading with hot-reload support
   - Observability (metrics, telemetry)
   - Resilience patterns (circuit breaker)

4. **Adapter Layer** (`internal/adapter/`)
   - HTTP handlers for webhooks and API endpoints
   - DTOs for Alertmanager, Slack, and PagerDuty payloads
   - Response formatters (Slack Block Kit)
   - Middleware (authentication, logging, recovery)

5. **Application Layer** (`internal/app/`)
   - Dependency injection and wiring
   - Application lifecycle management
   - Storage factory pattern
   - Client initialization

### Adding a New Feature

1. **Define domain entity** (`internal/domain/entity/`)
2. **Create repository interface** (`internal/domain/repository/`)
3. **Implement use case** (`internal/usecase/`)
4. **Add persistence** (`internal/infrastructure/persistence/`)
5. **Create HTTP handler** (`internal/adapter/handler/`)
6. **Write tests** for each layer

## Building

### Development Build

```bash
go build -o alert-bridge ./cmd/alert-bridge
```

### Production Build

```bash
go build -ldflags="-s -w" -o alert-bridge ./cmd/alert-bridge
```

### Cross-Platform Build

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o alert-bridge-linux ./cmd/alert-bridge

# macOS
GOOS=darwin GOARCH=amd64 go build -o alert-bridge-macos ./cmd/alert-bridge

# Windows
GOOS=windows GOARCH=amd64 go build -o alert-bridge.exe ./cmd/alert-bridge
```

## Code Quality

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

### Formatting

```bash
# Format all Go files
go fmt ./...

# Organize imports
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

## Debugging

### Enable Debug Logging

Set log level to `debug` in config:

```yaml
logging:
  level: debug
  format: json
```

### View Logs

```bash
# Follow logs
tail -f /var/log/alert-bridge/app.log

# With jq for JSON logs
tail -f /var/log/alert-bridge/app.log | jq .
```

### Using Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug application
dlv debug ./cmd/alert-bridge
```

## Contributing

### Code Style

- Follow Go standard formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions small and focused

### Testing

- Write unit tests for new functions
- Add integration tests for persistence layer
- Maintain test coverage above 70%
- Use table-driven tests where appropriate

### Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`go test ./...`)
5. Format code (`go fmt ./...`)
6. Commit your changes
7. Push to your fork
8. Create a Pull Request

### Commit Messages

Follow conventional commits format:

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Adding or updating tests
- `chore:` - Build process or auxiliary tool changes

Examples:
```
feat: add MySQL persistence support
fix: resolve SQLite database locking issue
docs: update deployment guide
```

## Next Steps

- [Architecture](architecture.md) - Understand the system design
- [API Reference](api.md) - Learn about endpoints
- [Storage](storage.md) - Configure persistence
