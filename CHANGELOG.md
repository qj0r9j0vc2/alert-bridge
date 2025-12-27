# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.1] - 2025-12-27

### Added

- **Alertmanager Integration**: Webhook receiver for processing alerts
- **Slack Integration**: Full app integration with Socket Mode and HTTP Mode
  - Socket Mode for local development (WebSocket, no public endpoints)
  - HTTP Mode for production (webhook with HMAC-SHA256 verification)
  - `/alert-status` slash command with severity filtering
  - Interactive buttons for alert acknowledgment
  - Connection health monitoring in `/health` endpoint
- **PagerDuty Integration**: Bidirectional sync for acknowledgments
  - Webhook receiver with HMAC-SHA256 verification
  - Incident creation via Events API v2
  - REST API integration for acknowledgments
- **Persistent Storage**: SQLite and MySQL support
  - Alert, acknowledgment event, and silence storage
  - Indexed queries for sub-millisecond performance
- **Alert Management**: Silence creation and management
- **Audit Trail**: Complete acknowledgment event history with source attribution
- **Configuration**: Hot reload without service restart
- **Deployment**: Kubernetes manifests with health probes and TLS ingress
- **CI/CD**: GitHub Actions workflows for manifest validation and deployment

### Performance

- Slash command response time: <2 seconds
- Database queries: Sub-millisecond with indexed lookups
- Webhook processing: <2 seconds end-to-end

### Security

- HMAC-SHA256 signature verification for all webhooks
- Timestamp validation (5-minute window) for replay attack prevention
- Constant-time signature comparison for timing attack prevention
