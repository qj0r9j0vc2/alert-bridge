# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Slack CLI Integration
- Full Slack app integration with dual-mode support:
  - Socket Mode for local development (no public endpoints required)
  - HTTP Mode for production deployments (webhook-based)
- Slash command `/alert-status` for querying active alerts with severity filtering
- Interactive message buttons for alert acknowledgment from Slack
- Event subscriptions for app mentions and reactions
- HMAC-SHA256 signature verification for webhook security
- Slack manifest (`manifest.json`) with automated validation workflow
- Socket Mode client with exponential backoff reconnection (500ms to 60s)
- Connection health monitoring and status reporting in `/health` endpoint
- App configuration hot reload support
- CI/CD workflows for automated manifest validation and deployment

#### Infrastructure
- `internal/infrastructure/slack/socketmode_client.go` - Socket Mode WebSocket client wrapper
- `internal/infrastructure/slack/socketmode_reconnect.go` - Reconnection logic with circuit breaker
- `internal/infrastructure/slack/logger_adapter.go` - slog to Slack Logger adapter
- `internal/infrastructure/slack/signature_verifier.go` - Webhook signature verification
- `internal/infrastructure/server/server.go` - Enhanced server with Socket Mode support

#### Handlers
- `internal/adapter/handler/slack_commands.go` - Slash command handler with <2s SLA
- `internal/adapter/handler/slack_socketmode_handler.go` - Socket Mode event router
- Enhanced `health.go` with Slack connection status reporting

#### Use Cases
- `internal/usecase/slack/query_alert_status.go` - Alert status query with severity filtering

#### DTOs and Presenters
- `internal/adapter/dto/slack_command_dto.go` - Slash command request/response models
- `internal/adapter/dto/slack_response_dto.go` - Slack response builders (ephemeral/in-channel)
- `internal/adapter/presenter/slack_alert_formatter.go` - Block Kit alert formatting

#### Configuration
- Socket Mode configuration in `config.yaml`:
  - `slack.socket_mode.enabled` - Enable/disable Socket Mode
  - `slack.socket_mode.app_token` - App-level token (xapp-...)
  - `slack.socket_mode.debug` - Debug logging toggle
  - `slack.socket_mode.ping_interval` - WebSocket ping interval
- Mutual exclusivity validation (Socket Mode OR HTTP Mode, not both)

#### Deployment
- Kubernetes manifests (`k8s/deployment.yaml`, `k8s/ingress.yaml`)
- GitHub Actions workflow for automated deployment on version tags
- Manifest sync workflow with diff checking

#### Documentation
- `specs/feat-slack-cli-integration/quickstart.md` - Quick start guide
- `specs/feat-slack-cli-integration/TROUBLESHOOTING.md` - Common issues and solutions
- Updated README.md with Slack integration features

### Changed
- `internal/infrastructure/server/server.go` signature changed to return error
- Health handler now reports Slack connection status (mode, connected, connection_id)
- Router supports both Socket Mode and HTTP Mode handlers

### Performance
- Slash commands respond within 2 seconds (SLA)
- Immediate acknowledgment + async processing pattern
- Sub-millisecond database queries with indexed lookups
- Efficient Block Kit formatting (limit 10 alerts per response)

### Security
- HMAC-SHA256 webhook signature verification
- Timestamp validation with 5-minute window (replay attack prevention)
- Constant-time signature comparison (timing attack prevention)
- Mutual TLS support for Kubernetes ingress

## [0.1.0] - Initial Release

### Added
- Alertmanager webhook integration
- PagerDuty bidirectional sync
- Basic Slack notifications
- SQLite and MySQL persistent storage
- Alert silence management
- Acknowledgment audit trail
