# Alert-Bridge E2E Test Suite

Comprehensive end-to-end testing framework for the alert-bridge system using isolated git worktrees, Docker Compose orchestration, and custom mock services.

## Quick Start

### Run Complete Test Suite

```bash
# From project root
./scripts/e2e-setup.sh
```

This single command will:
1. ✅ Validate prerequisites (Docker, Git, Go, ports)
2. ✅ Create isolated git worktree
3. ✅ Build and start all services (Prometheus, Alertmanager, Alert-Bridge, mocks)
4. ✅ Wait for services to become healthy
5. ✅ Execute all E2E tests
6. ✅ Collect diagnostics on failure
7. ✅ Clean up all resources automatically

### Run Individual Scenario

```bash
# Run only Slack delivery test
./scripts/e2e-run-scenario.sh alert-creation-slack

# Run only deduplication test
./scripts/e2e-run-scenario.sh alert-deduplication
```

**Available scenarios:**
- `alert-creation-slack` - Alert delivery to Slack
- `alert-creation-pagerduty` - Alert delivery to PagerDuty
- `alert-deduplication` - Duplicate alert suppression
- `alert-resolution` - Resolution notifications
- `multiple-alerts-grouping` - Alert grouping behavior
- `different-severity-levels` - Severity handling

## Architecture

### Service Orchestration

```
┌─────────────────────────────────────────────────────────────┐
│                    Git Worktree (.worktrees/e2e-<timestamp>) │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐      ┌───────────┐ │
│  │ Prometheus   │─────▶│ Alertmanager │─────▶│Alert-     │ │
│  │ :9000        │      │ :9093        │      │Bridge     │ │
│  └──────────────┘      └──────────────┘      │:9080      │ │
│                                               └─────┬─────┘ │
│                                                     │        │
│                             ┌───────────────────────┴──┐    │
│                             ▼                          ▼    │
│                      ┌──────────────┐          ┌──────────┐│
│                      │ Mock Slack   │          │Mock PD   ││
│                      │ :9091        │          │:9092     ││
│                      └──────────────┘          └──────────┘│
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Mock Services

**Mock Slack** (`test/e2e/mocks/slack/`)
- Validates Slack Block Kit messages
- Stores messages for test verification
- Endpoints:
  - `POST /api/chat.postMessage` - Post message
  - `POST /api/chat.update` - Update message
  - `GET /api/test/messages` - Query messages (test helper)
  - `POST /api/test/reset` - Reset state (test helper)
  - `GET /health` - Health check

**Mock PagerDuty** (`test/e2e/mocks/pagerduty/`)
- Validates PagerDuty Events API v2 payloads
- Stores events for test verification
- Endpoints:
  - `POST /v2/enqueue` - Send event (trigger/acknowledge/resolve)
  - `GET /api/test/events` - Query events (test helper)
  - `POST /api/test/reset` - Reset state (test helper)
  - `GET /health` - Health check

## Test Helpers

### Docker Management (`helpers/docker.go`)

```go
// Wait for all services to become healthy
helpers.WaitForAllServices(t)

// Reset mock services to clean state
helpers.ResetMockServices(t)

// Check individual service health
err := helpers.ServiceHealthCheck("http://localhost:9091/health")
```

### Alert Creation (`helpers/alerts.go`)

```go
// Create alert from fixture
alert := helpers.CreateTestAlert("high_cpu_critical", nil)

// Override labels
alert := helpers.CreateTestAlert("high_cpu_critical", map[string]string{
    "severity": "warning",
})

// Send to Alertmanager
helpers.SendAlertToAlertmanager(t, alert)

// Send to Alert-Bridge directly
helpers.SendAlertToAlertBridge(t, []Alert{alert})

// Mark as resolved
resolvedAlert := helpers.ResolveAlert(t, alert)
```

### Assertions (`helpers/assertions.go`)

```go
// Assert Slack message received
msg := helpers.AssertSlackMessageReceived(t, alert.Fingerprint)
helpers.AssertSlackMessageContains(t, msg, "HighCPU")
helpers.AssertSlackMessageCount(t, 1, alert.Fingerprint)

// Assert PagerDuty event received
event := helpers.AssertPagerDutyEventReceived(t, alert.Fingerprint, "trigger")
helpers.AssertPagerDutyEventAction(t, event, "trigger")
helpers.AssertPagerDutyEventSeverity(t, event, "critical")
helpers.AssertPagerDutyEventCount(t, 2, alert.Fingerprint)
```

### Diagnostics (`helpers/diagnostics.go`)

```go
// Log test phases for trace
helpers.LogTestPhase(t, "setup_environment")
helpers.LogTestPhase(t, "send_alert")
helpers.LogTestPhase(t, "verify_delivery")

// Capture diagnostics on failure
helpers.CaptureFailureDiagnostics(t, worktreeDir)

// Dump mock service state for debugging
helpers.DumpMockServiceState(t)

// Print service URLs
helpers.PrintServiceURLs(t)
```

## Writing Tests

### Basic Test Structure

```go
func TestMyScenario(t *testing.T) {
    // Track execution time
    startTime := time.Now()
    if reporter := helpers.GetReporter(); reporter != nil {
        reporter.StartTest(t.Name())
        defer func() {
            reporter.RecordPhase("total_execution", startTime)
            reporter.EndTest(t)
        }()
    }

    // Setup
    helpers.WaitForAllServices(t)
    helpers.ResetMockServices(t)

    // Create and send alert
    alert := helpers.CreateTestAlert("high_cpu_critical", nil)
    helpers.SendAlertToAlertmanager(t, alert)

    // Wait for processing
    time.Sleep(3 * time.Second)

    // Verify delivery
    msg := helpers.AssertSlackMessageReceived(t, alert.Fingerprint)
    helpers.AssertSlackMessageContains(t, msg, "HighCPU")

    t.Log("✓ Test passed")
}
```

### Using Test Fixtures

Fixtures are defined in `test/e2e/fixtures/alerts.json`:

```go
// Available fixtures
fixtures := []string{
    "high_cpu_critical",
    "memory_pressure_warning",
    "service_down_critical",
    "high_latency_warning",
    "duplicate_test_alert",
    "disk_space_critical",
    "database_connection_error",
    "api_rate_limit_warning",
    "ssl_certificate_expiring",
    "backup_failed_critical",
}

alert := helpers.CreateTestAlert("high_cpu_critical", nil)
```

## Configuration

### Environment Variables

```bash
# Port configuration
export E2E_BASE_PORT=9000              # Prometheus port
export E2E_BASE_PORT_ALERTMANAGER=9093 # Alertmanager port
export E2E_MOCK_SLACK_PORT=9091        # Mock Slack port
export E2E_MOCK_PAGERDUTY_PORT=9092    # Mock PagerDuty port
export E2E_ALERT_BRIDGE_PORT=9080      # Alert-Bridge port

# Timeout configuration
export E2E_SERVICE_TIMEOUT=60          # Service startup timeout (seconds)
export E2E_TEST_TIMEOUT=300            # Test execution timeout (seconds)

# Mock behavior
export SLACK_MOCK_FAILURE_RATE=0.0     # Simulate failures (0.0-1.0)
export SLACK_MOCK_LATENCY_MIN=10       # Min latency (ms)
export SLACK_MOCK_LATENCY_MAX=500      # Max latency (ms)
export PAGERDUTY_MOCK_FAILURE_RATE=0.0 # Simulate failures (0.0-1.0)

# Debugging
export E2E_PRESERVE_ON_SUCCESS=false   # Keep worktree on success
export E2E_VERBOSE=false               # Enable verbose logging
```

### Custom Port Range

```bash
# Use ports 10000-10013 instead of default
E2E_BASE_PORT=10000 ./scripts/e2e-setup.sh
```

## Debugging

### Failed Test Diagnostics

When a test fails, diagnostics are automatically collected:

```
.worktrees/e2e-<timestamp>/diagnostics/
├── services/
│   ├── e2e-prometheus.log
│   ├── e2e-alertmanager.log
│   ├── e2e-alert-bridge.log
│   ├── e2e-mock-slack.log
│   └── e2e-mock-pagerduty.log
├── containers.json
└── test-trace.log
```

**View diagnostics:**

```bash
# Service logs
cat .worktrees/e2e-<timestamp>/diagnostics/services/alert-bridge.log

# Container states
cat .worktrees/e2e-<timestamp>/diagnostics/containers.json | jq

# Test execution trace
cat .worktrees/e2e-<timestamp>/diagnostics/test-trace.log
```

### Query Mock Services

While services are running:

```bash
# List all Slack messages
curl http://localhost:9091/api/test/messages | jq

# Filter by fingerprint
curl 'http://localhost:9091/api/test/messages?fingerprint=abc123' | jq

# List all PagerDuty events
curl http://localhost:9092/api/test/events | jq

# Filter by dedup_key and action
curl 'http://localhost:9092/api/test/events?dedup_key=abc123&action=trigger' | jq
```

### Manual Cleanup

If automatic cleanup fails:

```bash
# Clean up everything
./scripts/e2e-cleanup.sh

# Clean up only containers
./scripts/e2e-cleanup.sh --containers

# Clean up only worktrees
./scripts/e2e-cleanup.sh --worktrees
```

## Troubleshooting

### Port Already in Use

```bash
# Find process using port
lsof -i :9090

# Kill process
kill <PID>

# Or use different port range
E2E_BASE_PORT=10000 ./scripts/e2e-setup.sh
```

### Docker Containers Not Starting

```bash
# Check Docker is running
docker ps

# Check Docker Compose version (must be v2.x)
docker compose version

# View container logs
docker logs e2e-prometheus
docker logs e2e-alert-bridge
```

### Worktree Creation Fails

```bash
# Clean up stale worktrees
./scripts/e2e-cleanup.sh --worktrees

# Or manually remove
git worktree remove .worktrees/e2e-<timestamp> --force
```

### Tests Timeout

```bash
# Increase timeout
E2E_TEST_TIMEOUT=600 ./scripts/e2e-setup.sh

# Run individual scenario to isolate issue
./scripts/e2e-run-scenario.sh slow-scenario
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: E2E Tests

on: [push, pull_request]

jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24.2'

      - name: Run E2E Tests
        run: ./scripts/e2e-setup.sh
        timeout-minutes: 10

      - name: Upload Diagnostics on Failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: e2e-diagnostics
          path: .worktrees/*/diagnostics/
          retention-days: 7
```

## Directory Structure

```
test/e2e/
├── README.md                    ← This file
├── e2e_test.go                  ← Main test suite (7 scenarios)
├── fixtures/
│   └── alerts.json              ← Test alert fixtures
├── helpers/
│   ├── alerts.go                ← Alert creation & sending
│   ├── assertions.go            ← Custom test assertions
│   ├── diagnostics.go           ← Failure diagnostics
│   ├── docker.go                ← Service management
│   ├── fixtures.go              ← Fixture loader
│   └── report.go                ← Test reporting
└── mocks/
    ├── pagerduty/
    │   ├── Dockerfile
    │   ├── handler.go
    │   └── main.go
    └── slack/
        ├── Dockerfile
        ├── handler.go
        └── main.go

test/e2e-config/
├── alert-bridge.yaml            ← Alert-Bridge config
├── alert-rules.yml              ← Test alert rules
├── alertmanager.yml             ← Alertmanager config
└── prometheus.yml               ← Prometheus config

test/e2e-docker-compose.yml      ← Service orchestration

scripts/
├── e2e-setup.sh                 ← Main orchestration script
├── e2e-cleanup.sh               ← Manual cleanup script
└── e2e-run-scenario.sh          ← Individual scenario runner
```

## Performance Benchmarks

Expected performance on typical development machine (MacBook Pro M1, 16GB RAM):

| Metric                    | Target      | Typical   |
|---------------------------|-------------|-----------|
| Total test suite          | < 5 minutes | 4m30s     |
| Service startup           | N/A         | 45s       |
| Individual test scenario  | < 30s       | 12-20s    |
| Mock service startup      | N/A         | < 1s      |
| Cleanup time              | N/A         | 10s       |

## Test Coverage

The E2E test suite validates:

✅ Alert creation and delivery to Slack
✅ Alert creation and delivery to PagerDuty
✅ Alert deduplication logic
✅ Alert resolution notifications
✅ Multiple alerts grouping
✅ Different severity levels handling
✅ Service health and availability

## Contributing

### Adding New Test Scenarios

1. Create test fixture in `test/e2e/fixtures/alerts.json`
2. Add test function to `test/e2e/e2e_test.go`
3. Update scenario mapping in `scripts/e2e-run-scenario.sh`
4. Document in this README

### Modifying Mock Services

Mock services follow OpenAPI 3.0.3 specifications in `specs/feat-e2e-test-env/contracts/`:
- `mock-slack-api.yaml`
- `mock-pagerduty-api.yaml`

## License

Part of the alert-bridge project.
