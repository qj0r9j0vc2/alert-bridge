# Alert Bridge - Development Guidelines

This document provides development context and guidelines for working on the Alert Bridge codebase.

## Project Overview

Alert Bridge is a unified alert management system built with Clean Architecture principles, bridging Alertmanager with Slack and PagerDuty through bidirectional acknowledgment synchronization.

## Technology Stack

- **Language**: Go 1.24.2
- **Architecture**: Clean Architecture (Domain → Use Case → Infrastructure → Adapter)
- **Storage**: In-memory, SQLite, MySQL
- **Integrations**: Slack (Bot API), PagerDuty (Events API v2 + REST API v2), Alertmanager
- **Dependencies**:
  - `github.com/PagerDuty/go-pagerduty` v1.8.0
  - `github.com/slack-go/slack` (Slack SDK)
  - `github.com/spf13/viper` (Configuration management)

## Architecture Layers

### Domain Layer (`internal/domain`)
- **Entities**: Pure business objects (Alert, AckEvent, Silence)
- **Repositories**: Interface definitions (no implementation)
- **Rules**: Business logic and validation

### Use Case Layer (`internal/usecase`)
- **Alert Processing**: Deduplication, silence checking, notification
- **Ack Synchronization**: Bidirectional sync between Slack and PagerDuty
- **PagerDuty Webhooks**: Event handling and state synchronization

### Infrastructure Layer (`internal/infrastructure`)
- **PagerDuty Client**: Events API v2 + REST API v2 with retry logic
- **Slack Client**: Message posting and interaction handling
- **Persistence**: SQLite and MySQL repositories
- **Configuration**: Viper-based config with hot reload

### Adapter Layer (`internal/adapter`)
- **HTTP Handlers**: Webhook receivers (Alertmanager, Slack, PagerDuty)
- **DTOs**: Request/response mapping
- **Middleware**: Authentication, signature verification

## PagerDuty Integration Architecture

### Dual API Strategy

Alert Bridge uses both PagerDuty APIs strategically:

**Events API v2** (`routing_key`):
- Primary mechanism for incident lifecycle
- Used for: trigger, acknowledge, resolve operations
- Lighter weight, no authentication required
- Integrated at infrastructure layer via `ManageEventWithContext`

**REST API v2** (`api_token`):
- Optional advanced features
- Used for: incident notes, health checks, service validation
- Requires authentication token
- Integrated via custom `RESTClient` wrapper

### Retry Logic Implementation

**File**: `internal/infrastructure/pagerduty/retry.go`

All PagerDuty API calls use exponential backoff retry:

```go
type RetryPolicy struct {
    InitialDelay time.Duration // 100ms
    Multiplier   float64       // 2.0x
    MaxDelay     time.Duration // 5s
    MaxRetries   int           // 3
}
```

**Error Classification**:
- **Retryable**: 5xx errors, 429 rate limits, network timeouts
- **Non-Retryable**: 4xx errors (401, 403, 404), context cancellation

**Implementation Pattern**:
```go
err := retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
    resp, err := pagerduty.ManageEventWithContext(ctx, event)
    return err
})
```

### Health Check Strategy

**File**: `internal/infrastructure/pagerduty/health_check.go`

On-demand health checks with caching:
- **Triggers**: Application startup, post-REST API failure
- **Cache TTL**: 5 minutes
- **Timeout**: 5 seconds per check
- **Thread Safety**: `sync.RWMutex` for concurrent access

**Usage Pattern**:
```go
if client.healthChecker != nil {
    result, err := client.RunHealthCheck(ctx)
    // Check passes: continue normally
    // Check fails: log warning, degrade gracefully
}
```

### Incident Notes Integration

**File**: `internal/infrastructure/pagerduty/helpers.go`

Automatically creates PagerDuty incident notes when Slack acknowledgments include comments:

```go
func formatIncidentNote(alert *entity.Alert, ackEvent *entity.AckEvent, fromEmail string) string {
    // Combines:
    // 1. User attribution (UserName or fromEmail)
    // 2. Ack comment content
    // 3. Alert context (name, instance, fingerprint)
}
```

**Non-Blocking Design**:
- Note creation failures do not fail the acknowledgment operation
- Errors logged with structured context
- Post-failure health check triggered in background

### Configuration Validation

**File**: `internal/infrastructure/config/config.go`

Enhanced validation with clear guidance:

```go
if c.PagerDuty.Enabled {
    // routing_key: REQUIRED (fail startup if missing)
    if c.PagerDuty.RoutingKey == "" {
        return fmt.Errorf("pagerduty.routing_key is required")
    }

    // api_token: OPTIONAL (warn if missing)
    if c.PagerDuty.APIToken == "" {
        slog.Warn("PagerDuty REST API features disabled: api_token not configured")
    }
}
```

## Code Patterns

### Structured Logging

Use `log/slog` with structured key-value pairs:

```go
slog.Info("PagerDuty event sent successfully",
    "alert_id", alert.ID,
    "alert_name", alert.Name,
    "action", "trigger",
    "dedup_key", resp.DedupKey,
    "response_time", responseTime)
```

### Context Propagation

Always propagate context for cancellation and timeouts:

```go
func (c *Client) Acknowledge(ctx context.Context, alert *entity.Alert, ackEvent *entity.AckEvent) error {
    // Apply timeout to nested operations
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Pass context to all downstream calls
    err := c.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
        _, retryErr := pagerduty.ManageEventWithContext(ctx, *event)
        return retryErr
    })
}
```

### Graceful Degradation

Optional features fail gracefully without blocking core functionality:

```go
// Add incident note if REST API is available and note exists
if c.restClient != nil && ackEvent.Note != "" {
    c.createIncidentNote(ctx, dedupKey, alert, ackEvent)
    // Note: Non-blocking, errors logged but not returned
}
```

### Error Wrapping

Use `fmt.Errorf` with `%w` for error chain preservation:

```go
if err != nil {
    return fmt.Errorf("acknowledging pagerduty event: %w", err)
}
```

## Testing Strategy

### Manual Testing Checklist

Before deployment, verify:
- ✅ Events API works with `routing_key` only (backward compatibility)
- ✅ Health check executes on startup when `api_token` + `service_id` configured
- ✅ Incident notes created when Slack ack includes comment
- ✅ Retry logic triggers on 5xx/429 errors with correct delays
- ✅ New webhook events processed correctly
- ✅ Configuration validation fails on missing `routing_key`
- ✅ Configuration warnings logged for missing optional fields

### Unit Testing

Not currently implemented (test coverage to be added in future iterations).

## Development Workflow

### Making Changes

1. Create feature branch from `main`
2. Implement changes following Clean Architecture boundaries
3. Use atomic commits with conventional commit prefixes
4. Update documentation (README.md, CLAUDE.md)
5. Manually test backward compatibility
6. Submit pull request

### Commit Message Format

Follow conventional commits:
```
feat: add incident notes integration
fix: resolve retry timeout issue
refactor: extract health check logic
docs: update PagerDuty configuration guide
```

### Code Organization

**File Naming Conventions**:
- Implementation: `client.go`, `health_check.go`, `retry.go`
- Interfaces: `rest_client.go` (contains interface + implementation)
- Helpers: `helpers.go` (utility functions)
- Tests: `*_test.go` (when added)

**Package Structure**:
```
internal/infrastructure/pagerduty/
├── client.go           # Main client with Events API integration
├── rest_client.go      # REST API wrapper
├── health_check.go     # On-demand health checker
├── retry.go            # Exponential backoff retry logic
└── helpers.go          # Incident note formatting
```

## Performance Considerations

### Retry Performance

Total retry duration with exponential backoff:
- **Best Case**: Single successful call (~50-100ms network latency)
- **Worst Case**: 3 retries = 100ms + 200ms + 400ms = ~700ms + network latency
- **Typical 5xx Recovery**: 1-2 retries = ~300-500ms total

### Health Check Caching

Cache prevents excessive API calls:
- **Cache Hit**: Instant return (no API call)
- **Cache Miss**: 5-second timeout API call
- **Cache Duration**: 5 minutes
- **Concurrent Safety**: Read-heavy pattern optimized with `RWMutex`

### Database Query Performance

SQLite query patterns:
- Alert lookup by PagerDuty incident ID: Index-based, sub-millisecond
- Alert lookup by fingerprint: Index-based, sub-millisecond
- Ack event creation: Single INSERT, sub-millisecond

## Debugging

### Enable Debug Logging

```yaml
logging:
  level: debug
  format: json
```

### Key Log Messages

**Startup**:
- `PagerDuty REST API client initialized`
- `PagerDuty health checker initialized`
- `PagerDuty health check completed`

**Operations**:
- `PagerDuty event sent successfully`
- `Incident note created successfully`
- `Failed to create incident note (ack still succeeded)`

**Failures**:
- `Failed to send PagerDuty event after retries`
- `PagerDuty health check failed`

## Common Issues

### Issue: Health Check Fails on Startup

**Symptoms**: Warning logged: "PagerDuty health check failed"

**Possible Causes**:
1. Invalid `api_token`
2. Invalid `service_id`
3. Network connectivity issues
4. PagerDuty API outage

**Resolution**:
- Verify token has correct permissions
- Verify service ID exists in PagerDuty
- Application continues with degraded features (graceful degradation)

### Issue: Incident Notes Not Created

**Symptoms**: Warning logged: "Failed to create incident note"

**Possible Causes**:
1. `api_token` not configured
2. Token lacks note creation permissions
3. Invalid incident ID

**Resolution**:
- Verify `api_token` configured
- Verify token has `write` permissions
- Note: Acknowledgment still succeeds (non-blocking design)

## Future Enhancements

Potential areas for improvement:
- Add unit and integration tests
- Implement circuit breaker pattern
- Add metrics/observability (Prometheus)
- Support additional webhook event types
- Add responder request functionality (when go-pagerduty library supports it)

## References

- [PagerDuty Events API v2 Documentation](https://developer.pagerduty.com/docs/ZG9jOjExMDI5NTgw-events-api-v2-overview)
- [PagerDuty REST API v2 Documentation](https://developer.pagerduty.com/api-reference/)
- [PagerDuty Webhooks V3 Documentation](https://developer.pagerduty.com/docs/webhooks/v3-overview/)
- [go-pagerduty Library](https://github.com/PagerDuty/go-pagerduty)
