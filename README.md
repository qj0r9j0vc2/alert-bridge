# Alert Bridge

A unified alert management system that bridges Alertmanager with Slack and PagerDuty, enabling bidirectional acknowledgment synchronization and persistent alert storage.

## Features

- **Alert Processing**: Receive and process alerts from Alertmanager webhooks
- **Bidirectional Sync**: Synchronize acknowledgments between Slack and PagerDuty
- **Persistent Storage**: SQLite and MySQL-based persistence for alerts, ack events, and silence rules
- **Silence Management**: Create and manage alert silences across platforms
- **Audit Trail**: Complete history of all acknowledgment events
- **High Performance**: Sub-millisecond read/write operations

## Quick Start

### Prerequisites

- Go 1.24 or later
- Slack workspace (optional)
- PagerDuty account (optional)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/qj0r9j0vc2/alert-bridge.git
cd alert-bridge
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o alert-bridge ./cmd/alert-bridge
```

### Configuration

Create and configure `config/config.yaml`:

```bash
cp config/config.example.yaml config/config.yaml
```

Edit the configuration file with your settings:

```yaml
server:
  port: 8080

storage:
  type: sqlite  # Options: memory, sqlite, mysql
  sqlite:
    path: ./data/alert-bridge.db

slack:
  enabled: true
  bot_token: ${SLACK_BOT_TOKEN}
  signing_secret: ${SLACK_SIGNING_SECRET}
  channel_id: ${SLACK_CHANNEL_ID}

pagerduty:
  enabled: true
  routing_key: ${PAGERDUTY_ROUTING_KEY}      # REQUIRED - Events API v2
  api_token: ${PAGERDUTY_API_TOKEN}          # OPTIONAL - REST API features
  service_id: ${PAGERDUTY_SERVICE_ID}        # OPTIONAL - Health checks
  from_email: ${PAGERDUTY_FROM_EMAIL}        # OPTIONAL - Note attribution
  webhook_secret: ${PAGERDUTY_WEBHOOK_SECRET}
```

See [PagerDuty Configuration](#pagerduty-configuration) for detailed setup.

### Running

Start the application:

```bash
./alert-bridge
```

Verify it's running:

```bash
curl http://localhost:8080/health
```

## Documentation

### Getting Started
- [Installation Guide](docs/installation.md) - Detailed installation and configuration
- [Storage Options](docs/storage.md) - Configure persistent storage (SQLite, MySQL)
- [API Reference](docs/api.md) - Available endpoints and webhooks

### Deployment
- [Deployment Guide](docs/deployment.md) - Docker and Kubernetes deployment
- [Troubleshooting](docs/troubleshooting.md) - Common issues and solutions

### Development
- [Development Guide](docs/development.md) - Project structure, testing, contributing
- [Architecture](docs/architecture.md) - System design and architecture decisions

## Architecture

Alert Bridge follows Clean Architecture principles with four distinct layers:

- **Domain Layer**: Core business entities and repository interfaces
- **Use Case Layer**: Business logic for alert processing, ack sync, silence management
- **Infrastructure Layer**: External integrations (Slack, PagerDuty, SQLite, MySQL)
- **Adapter Layer**: HTTP handlers, request/response mapping

See [Architecture Documentation](docs/architecture.md) for details.

## Storage Backends

| Feature | Memory | SQLite | MySQL |
|---------|--------|--------|-------|
| Persistence | No | Yes | Yes |
| Multi-instance | No | No | Yes |
| Performance | Fastest | Very Fast | Fast |
| Recommended For | Dev/Test | Single instance | Multi-instance/HA |

See [Storage Documentation](docs/storage.md) for configuration details.

## PagerDuty Configuration

Alert Bridge supports both PagerDuty Events API v2 (for incident lifecycle) and REST API v2 (for advanced features).

### Events API v2 (Required)

The **routing_key** enables core incident management:
- ✅ Create incidents (trigger)
- ✅ Acknowledge incidents
- ✅ Resolve incidents
- ✅ Exponential backoff retry (100ms → 5s, 3 retries)

**Setup:**
1. Generate an Events API v2 integration key in PagerDuty
2. Set `pagerduty.routing_key` in config
3. Incidents will be created/managed via Events API

### REST API v2 (Optional)

The **api_token** enables advanced features:
- 📝 **Incident Notes**: Automatically attach Slack ack comments to PagerDuty incidents
- 🏥 **Health Checks**: Validate PagerDuty connectivity on startup (cached for 5 minutes)
- 🔄 **Enhanced Logging**: Track response times and API operation success/failure

**Setup:**
1. Generate a REST API token in PagerDuty (requires appropriate permissions)
2. Set `pagerduty.api_token` in config
3. Optionally set `service_id` (for health checks) and `from_email` (for note attribution)

**Configuration Validation:**
```yaml
pagerduty:
  enabled: true
  routing_key: "xxx"    # ✅ REQUIRED - App fails to start if missing
  api_token: "yyy"      # ⚠️  OPTIONAL - Warning logged if missing
  service_id: "zzz"     # ⚠️  OPTIONAL - Warning logged if missing when api_token set
  from_email: "..."     # ⚠️  OPTIONAL - Warning logged if missing when api_token set
```

### Retry Logic

All PagerDuty API calls use exponential backoff:
- **Initial Delay**: 100ms
- **Multiplier**: 2x
- **Max Delay**: 5s
- **Max Retries**: 3 attempts
- **Total Max Duration**: ~700ms (100ms + 200ms + 400ms)

**Retryable Errors:**
- HTTP 5xx (server errors)
- HTTP 429 (rate limit)
- Network timeouts
- Connection errors

**Non-Retryable Errors:**
- HTTP 4xx (client errors like 401, 403, 404)
- Context cancellation

### Health Checks

When `api_token` and `service_id` are configured, Alert Bridge performs on-demand health checks:

**Startup Check:**
- Validates PagerDuty connectivity when application starts
- Logs warning and continues with degraded features if check fails
- Uses 5-second timeout

**Post-Failure Check:**
- Triggered automatically after REST API call failures
- Runs in background to avoid blocking operations
- Results cached for 5 minutes to prevent excessive API calls

**Cache Behavior:**
- Health check results cached with 5-minute TTL
- Subsequent checks within TTL return cached result
- Fresh check performed after cache expiration

### Webhook Event Coverage

Supported webhook event types:
- ✅ `incident.acknowledged` - Syncs ack to Slack
- ✅ `incident.resolved` - Updates alert state
- ✅ `incident.unacknowledged` - Logged (not synced)
- ✅ `incident.reassigned` - Logged
- ✅ `incident.escalated` - Logged
- ✅ `incident.priority_updated` - Logged
- ✅ `incident.responder_added` - Logged
- ✅ `incident.status_update_published` - Logged

### Backward Compatibility

The PagerDuty integration is fully backward compatible:
- Existing configurations with only `routing_key` continue to work
- Events API functionality unchanged
- REST API features gracefully disabled when `api_token` not configured
- No breaking changes to existing deployments

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Submit a pull request

See [Development Guide](docs/development.md) for detailed guidelines.

## License

MIT License

Copyright (c) 2025

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
