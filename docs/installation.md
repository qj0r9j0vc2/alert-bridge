# Installation

## Prerequisites

- Go 1.24 or later
- Slack workspace (optional)
- PagerDuty account (optional)

## Installation Steps

### 1. Clone the Repository

```bash
git clone https://github.com/qj0r9j0vc2/alert-bridge.git
cd alert-bridge
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Build the Application

```bash
go build -o alert-bridge ./cmd/alert-bridge
```

## Configuration

Create a `config/config.yaml` file based on the example:

```bash
cp config/config.example.yaml config/config.yaml
```

Edit `config/config.yaml` with your settings:

```yaml
server:
  port: 8080
  read_timeout: 5s
  write_timeout: 10s
  request_timeout: 25s
  shutdown_timeout: 30s

# Storage Configuration
storage:
  type: sqlite  # Options: memory, sqlite, mysql
  sqlite:
    path: ./data/alert-bridge.db

slack:
  enabled: true
  bot_token: ${SLACK_BOT_TOKEN}
  signing_secret: ${SLACK_SIGNING_SECRET}
  channel_id: ${SLACK_CHANNEL_ID}
  app_id: ${SLACK_APP_ID}  # Optional

  # Socket Mode (for local development, no public endpoints needed)
  socket_mode:
    enabled: false                            # Set to true for local dev
    app_token: ${SLACK_SOCKET_MODE_APP_TOKEN} # xapp-... token
    debug: false
    ping_interval: 30s

pagerduty:
  enabled: true
  api_token: ${PAGERDUTY_API_TOKEN}
  routing_key: ${PAGERDUTY_ROUTING_KEY}
  service_id: ${PAGERDUTY_SERVICE_ID}
  webhook_secret: ${PAGERDUTY_WEBHOOK_SECRET}
  from_email: ${PAGERDUTY_FROM_EMAIL}
  default_severity: warning  # critical, error, warning, info

# Alertmanager webhook settings
alertmanager:
  # Optional: HMAC-SHA256 webhook signature verification
  webhook_secret: ${ALERTMANAGER_WEBHOOK_SECRET}

alerting:
  deduplication_window: 5m
  resend_interval: 30m
  silence_durations: [15m, 1h, 4h, 24h]

logging:
  level: info
  format: json
```

## Running the Application

### Basic Usage

Start the server:

```bash
./alert-bridge
```

### Custom Config Path

```bash
CONFIG_PATH=/path/to/config.yaml ./alert-bridge
```

### Verify Running

```bash
curl http://localhost:8080/health
```

## Environment Variables

You can use environment variables in the config file using `${VAR_NAME}` syntax:

```yaml
slack:
  bot_token: ${SLACK_BOT_TOKEN}
  signing_secret: ${SLACK_SIGNING_SECRET}
```

### Supported Environment Variables

| Variable | Description |
|----------|-------------|
| `CONFIG_PATH` | Path to configuration file |
| **Server** | |
| `SERVER_PORT` | HTTP server port |
| **Slack** | |
| `SLACK_ENABLED` | Enable/disable Slack integration |
| `SLACK_BOT_TOKEN` | Bot User OAuth Token (xoxb-...) |
| `SLACK_SIGNING_SECRET` | Signing Secret for HTTP mode |
| `SLACK_CHANNEL_ID` | Default channel for alerts |
| `SLACK_APP_ID` | App ID for verification |
| `SLACK_SOCKET_MODE_ENABLED` | Enable Socket Mode for local dev |
| `SLACK_SOCKET_MODE_APP_TOKEN` | App-Level Token (xapp-...) |
| `SLACK_SOCKET_MODE_DEBUG` | Enable Socket Mode debug logging |
| `SLACK_SOCKET_MODE_PING_INTERVAL` | WebSocket ping interval (e.g., "30s") |
| **PagerDuty** | |
| `PAGERDUTY_ENABLED` | Enable/disable PagerDuty integration |
| `PAGERDUTY_API_TOKEN` | REST API Token |
| `PAGERDUTY_ROUTING_KEY` | Events API v2 Routing Key |
| `PAGERDUTY_SERVICE_ID` | Service ID for incidents |
| `PAGERDUTY_WEBHOOK_SECRET` | Webhook signature secret |
| `PAGERDUTY_FROM_EMAIL` | Email for API requests |
| `PAGERDUTY_DEFAULT_SEVERITY` | Default alert severity |
| **Alertmanager** | |
| `ALERTMANAGER_WEBHOOK_SECRET` | HMAC-SHA256 webhook secret |
| **Storage** | |
| `STORAGE_TYPE` | Storage backend (memory, sqlite, mysql) |
| `SQLITE_DATABASE_PATH` | SQLite database file path |
| **MySQL** | |
| `MYSQL_HOST` | MySQL primary host |
| `MYSQL_PORT` | MySQL primary port |
| `MYSQL_DATABASE` | MySQL database name |
| `MYSQL_USERNAME` | MySQL username |
| `MYSQL_PASSWORD` | MySQL password |
| `MYSQL_MAX_OPEN_CONNS` | Max open connections |
| `MYSQL_MAX_IDLE_CONNS` | Max idle connections |
| `MYSQL_CONN_MAX_LIFETIME` | Connection max lifetime (e.g., "3m") |
| `MYSQL_CONN_MAX_IDLE_TIME` | Connection max idle time |
| `MYSQL_REPLICA_ENABLED` | Enable read replica |
| `MYSQL_REPLICA_HOST` | Replica host |
| `MYSQL_REPLICA_PORT` | Replica port |
| `MYSQL_REPLICA_DATABASE` | Replica database |
| `MYSQL_REPLICA_USERNAME` | Replica username |
| `MYSQL_REPLICA_PASSWORD` | Replica password |
| **Logging** | |
| `LOG_LEVEL` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | Log format (json, text) |

### Example Usage

```bash
export CONFIG_PATH=/path/to/config.yaml
export SLACK_BOT_TOKEN=xoxb-your-token
export SLACK_SIGNING_SECRET=your-secret
export MYSQL_PASSWORD=secure-password
./alert-bridge
```

## Next Steps

- [Configure Storage](storage.md) - Choose and configure your storage backend
- [API Reference](api.md) - Learn about available endpoints
- [Deployment](deployment.md) - Deploy to production environments
