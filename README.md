# Alert Bridge

A unified alert management system that bridges Alertmanager with Slack and PagerDuty, enabling bidirectional acknowledgment synchronization and persistent alert storage.

## Features

- **Alert Processing**: Receive and process alerts from Alertmanager webhooks
- **Bidirectional Sync**: Synchronize acknowledgments between Slack and PagerDuty
- **Persistent Storage**: SQLite and MySQL-based persistence for alerts, ack events, and silence rules
- **Silence Management**: Create and manage alert silences across platforms
- **Audit Trail**: Complete history of all acknowledgment events
- **High Performance**: Sub-millisecond read/write operations
- **Webhook Security**: Optional HMAC-SHA256 signature verification for Alertmanager webhooks
- **Hot Reload**: Configuration hot reload without service restart
- **Clean Architecture**: Modular design with Application struct and factory pattern

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
  api_token: ${PAGERDUTY_API_TOKEN}
  routing_key: ${PAGERDUTY_ROUTING_KEY}
  service_id: ${PAGERDUTY_SERVICE_ID}
  webhook_secret: ${PAGERDUTY_WEBHOOK_SECRET}
  from_email: ${PAGERDUTY_FROM_EMAIL}

alertmanager:
  webhook_secret: ${ALERTMANAGER_WEBHOOK_SECRET}  # Optional: HMAC-SHA256 webhook authentication
```

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
