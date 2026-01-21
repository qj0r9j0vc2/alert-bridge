# Release Testing Guide

## 1. Local GoReleaser Test

### Snapshot Build (No Docker Push)

```bash
# Dry run - build binaries only
goreleaser build --snapshot --clean

# Verify binaries
ls -lh dist/

# Test a binary
./dist/alert-bridge_linux_amd64_v1/alert-bridge --help
```

### Full Release Simulation

```bash
# Simulate complete release (no push to GitHub/ghcr.io)
goreleaser release --snapshot --clean

# Check generated archives
ls -lh dist/*.tar.gz dist/*.zip

# Verify Docker images built locally
docker images | grep alert-bridge
```

## 2. Docker Image Testing

### Pull and Test Released Image

```bash
# Pull the image (after actual release)
docker pull ghcr.io/qj0r9j0vc2/alert-bridge:latest

# Verify multi-arch support
docker manifest inspect ghcr.io/qj0r9j0vc2/alert-bridge:latest

# Run basic health check
docker run --rm ghcr.io/qj0r9j0vc2/alert-bridge:latest --version
```

### Local Integration Test with Docker Compose

Create `docker-compose.test.yaml`:

```yaml
version: '3.8'

services:
  # Prometheus
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./test/prometheus.yml:/etc/prometheus/prometheus.yml
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'

  # Alertmanager
  alertmanager:
    image: prom/alertmanager:latest
    ports:
      - "9093:9093"
    volumes:
      - ./test/alertmanager.yml:/etc/alertmanager/alertmanager.yml
    command:
      - '--config.file=/etc/alertmanager/alertmanager.yml'

  # Alert Bridge (your application)
  alert-bridge:
    image: ghcr.io/qj0r9j0vc2/alert-bridge:latest
    ports:
      - "8080:8080"
    environment:
      - SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}
      - SLACK_SIGNING_SECRET=${SLACK_SIGNING_SECRET}
      - SLACK_CHANNEL_ID=${SLACK_CHANNEL_ID}
    volumes:
      - ./test/alert-bridge-config.yaml:/app/config/config.yaml
    depends_on:
      - alertmanager

  # Mock Slack (for testing without real Slack)
  mock-slack:
    image: mockserver/mockserver:latest
    ports:
      - "1080:1080"
    environment:
      - MOCKSERVER_INITIALIZATION_JSON_PATH=/config/mockserver.json
```

## 3. Alertmanager Integration Test

### Test Configuration Files

**test/alertmanager.yml:**

```yaml
global:
  resolve_timeout: 1m

route:
  group_by: ['alertname', 'cluster']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'alert-bridge'

receivers:
  - name: 'alert-bridge'
    webhook_configs:
      - url: 'http://alert-bridge:8080/webhook/alertmanager'
        send_resolved: true
```

**test/prometheus.yml:**

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']

rule_files:
  - '/etc/prometheus/alert.rules.yml'

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
```

**test/alert.rules.yml:**

```yaml
groups:
  - name: test_alerts
    interval: 10s
    rules:
      - alert: TestAlert
        expr: up == 1
        for: 5s
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "Test alert for integration testing"
          description: "This is a test alert to verify the pipeline"
```

**test/alert-bridge-config.yaml:**

```yaml
server:
  port: 8080

storage:
  type: sqlite
  sqlite:
    path: /app/data/alerts.db

slack:
  enabled: true
  bot_token: ${SLACK_BOT_TOKEN}
  signing_secret: ${SLACK_SIGNING_SECRET}
  channel_id: ${SLACK_CHANNEL_ID}

pagerduty:
  enabled: false

alerting:
  deduplication_window: 5m
  resend_interval: 30m

logging:
  level: debug
  format: json
```

### Run Integration Test

```bash
# Set up test environment
mkdir -p test data

# Create test configs (see above)

# Start the stack
docker-compose -f docker-compose.test.yaml up -d

# Wait for services to be ready
sleep 10

# Check all services are healthy
docker-compose -f docker-compose.test.yaml ps

# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Check Alertmanager config
curl http://localhost:9093/api/v2/status

# Trigger a test alert manually
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[
    {
      "labels": {
        "alertname": "TestAlert",
        "severity": "critical",
        "instance": "test-instance"
      },
      "annotations": {
        "summary": "Manual test alert",
        "description": "Testing alert-bridge integration"
      },
      "startsAt": "2025-12-25T00:00:00Z"
    }
  ]'

# Check alert-bridge logs
docker-compose -f docker-compose.test.yaml logs -f alert-bridge

# Verify alert-bridge is healthy and received the alert
curl http://localhost:8080/health
# Check logs for "alert processed" entries

# Clean up
docker-compose -f docker-compose.test.yaml down
```

## 4. End-to-End Release Test

### Test Release Workflow

```bash
# Create a test tag (lightweight, won't trigger workflow)
git tag -a v0.0.1-test -m "Test release"

# Push to test the workflow (use a test tag pattern if needed)
git push origin v0.0.1-test

# Monitor GitHub Actions
gh run list --workflow=release.yaml

# Watch the workflow
gh run watch

# If successful, check the release
gh release view v0.0.1-test

# Clean up test release
gh release delete v0.0.1-test --yes
git tag -d v0.0.1-test
git push origin :refs/tags/v0.0.1-test
```

## 5. Production Release Checklist

Before creating a production release:

- [ ] All unit tests pass: `go test ./...`
- [ ] Integration tests pass: `docker-compose -f docker-compose.test.yaml up`
- [ ] GoReleaser config valid: `goreleaser check`
- [ ] Snapshot build successful: `goreleaser build --snapshot --clean`
- [ ] Docker multi-arch builds work locally
- [ ] Alertmanager webhook integration verified
- [ ] Slack integration tested (or mock verified)
- [ ] Database migrations tested (SQLite and MySQL)
- [ ] Documentation updated
- [ ] CHANGELOG reviewed

### Create Production Release

```bash
# Ensure main branch is up to date
git checkout main
git pull origin main

# Create semantic version tag
git tag -a v1.0.0 -m "Release v1.0.0: GoReleaser CI/CD"

# Push tag to trigger workflow
git push origin v1.0.0

# Monitor release
gh run watch

# Verify release artifacts
gh release view v1.0.0

# Test released binaries
curl -LO https://github.com/qj0r9j0vc2/alert-bridge/releases/download/v1.0.0/alert-bridge_1.0.0_linux_amd64.tar.gz
tar xzf alert-bridge_1.0.0_linux_amd64.tar.gz
./alert-bridge --version

# Test released Docker image
docker pull ghcr.io/qj0r9j0vc2/alert-bridge:v1.0.0
docker run --rm ghcr.io/qj0r9j0vc2/alert-bridge:v1.0.0 --version
```

## 6. Continuous Monitoring

After release, monitor:

```bash
# Check GitHub Actions runs
gh run list --workflow=release.yaml --limit 5

# Monitor Docker image pulls
# (check GitHub Packages insights)

# Check for issues
gh issue list --label "bug" --label "release:v1.0.0"
```

## 7. Rollback Plan

If a release fails:

```bash
# Delete the GitHub release
gh release delete v1.0.0 --yes

# Delete the tag
git tag -d v1.0.0
git push origin :refs/tags/v1.0.0

# Fix issues and re-release with new version
git tag -a v1.0.1 -m "Release v1.0.1: Fix issue"
git push origin v1.0.1
```
