# API Reference

## Endpoints Overview

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Liveness check |
| `/ready` | GET | Readiness check (verifies dependencies) |
| `/metrics` | GET | Prometheus metrics |
| `/-/reload` | POST | Hot reload configuration |
| `/webhook/alertmanager` | POST | Receive Alertmanager webhooks |
| `/webhook/slack/commands` | GET | List available slash commands |
| `/webhook/slack/commands` | POST | Handle Slack slash commands |
| `/webhook/slack/interactions` | POST | Handle Slack button interactions |
| `/webhook/slack/events` | POST | Handle Slack Event API |
| `/webhook/pagerduty` | POST | Receive PagerDuty webhooks |

## Health & Observability

### Liveness Check

Check if the service is running.

```http
GET /health
```

**Response:**
```json
{
  "status": "ok"
}
```

### Readiness Check

Check if the service is ready to handle requests (verifies database connectivity and dependencies).

```http
GET /ready
```

**Response (Success):**
```json
{
  "status": "ok"
}
```

**Response (Not Ready):**
```json
{
  "status": "not ready",
  "error": "database ping failed"
}
```

### Prometheus Metrics

Get application metrics in Prometheus format.

```http
GET /metrics
```

**Response:** Prometheus text format with metrics including:
- `alert_bridge_http_requests_total` - Total HTTP requests
- `alert_bridge_http_request_duration_seconds` - Request latency histogram
- `alert_bridge_alerts_processed_total` - Total alerts processed
- `alert_bridge_slack_messages_sent_total` - Slack messages sent

### Hot Reload Configuration

Reload configuration without restarting the service.

```http
POST /-/reload
```

**Response:**
```json
{
  "status": "ok",
  "message": "configuration reloaded"
}
```

## Alertmanager Webhook

Receive alerts from Alertmanager.

```http
POST /webhook/alertmanager
Content-Type: application/json
X-Alertmanager-Signature: v1=<hex_hmac_sha256>  # Optional: if webhook_secret is configured
```

**Request Body:**
```json
{
  "receiver": "alert-bridge",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "instance": "server-1"
      },
      "annotations": {
        "description": "CPU usage is above 90%",
        "summary": "High CPU on server-1"
      },
      "startsAt": "2025-01-15T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "fingerprint": "abc123"
    }
  ]
}
```

**Response:**
```json
{
  "status": "ok",
  "processed": 1,
  "failed": 0
}
```

### Alertmanager Configuration

Add to your Alertmanager configuration:

```yaml
receivers:
  - name: 'alert-bridge'
    webhook_configs:
      - url: 'http://alert-bridge:8080/webhook/alertmanager'
        send_resolved: true
```

#### Optional: Webhook Authentication

If you configure `alertmanager.webhook_secret` in Alert-Bridge, you must include an HMAC-SHA256 signature in the webhook request.

**Alert-Bridge Configuration:**
```yaml
alertmanager:
  webhook_secret: "your_shared_secret"
```

**Signature Generation (Python example):**
```python
import hmac
import hashlib

secret = "your_shared_secret"
body = b'{"alerts": [...]}'  # Raw JSON body

signature = hmac.new(
    secret.encode(),
    body,
    hashlib.sha256
).hexdigest()

header = f"v1={signature}"
```

**Note:** Alertmanager doesn't natively support HMAC signatures. You may need:
- A reverse proxy (nginx, envoy) to add the signature
- A custom webhook forwarder
- Run Alert-Bridge without authentication on a private network

## Slack Integration

### List Slash Commands

Get a list of available slash commands with their metadata.

```http
GET /webhook/slack/commands
```

**Response:**
```json
{
  "commands": [
    {
      "command": "/alert-status",
      "description": "Check current alert status",
      "usage_hint": "[critical|warning|info]",
      "request_url": "/webhook/slack/commands",
      "should_escape": false,
      "autocomplete_hint": "Filter alerts by severity level"
    },
    {
      "command": "/summary",
      "description": "Get alert summary statistics",
      "usage_hint": "[1h|24h|7d|1w|today|week|all]",
      "request_url": "/webhook/slack/commands",
      "should_escape": false,
      "autocomplete_hint": "Specify time period for summary"
    }
  ],
  "total": 2
}
```

### Slash Commands

Handle Slack slash commands. Slack sends commands as `application/x-www-form-urlencoded`.

```http
POST /webhook/slack/commands
Content-Type: application/x-www-form-urlencoded
X-Slack-Signature: v0=<hex_hmac_sha256>
X-Slack-Request-Timestamp: <unix_timestamp>
```

**Supported Commands:**

| Command | Usage | Description |
|---------|-------|-------------|
| `/alert-status` | `/alert-status [critical\|warning\|info]` | Check current alert status, optionally filtered by severity |
| `/summary` | `/summary [1h\|24h\|7d\|1w\|today\|week\|all]` | Get alert summary statistics for a time period |

**Response:** Immediate acknowledgment followed by delayed response via `response_url`.

```json
{
  "response_type": "ephemeral",
  "text": "Fetching alert status..."
}
```

### Slack Interactions

Handle button clicks and interactions from Slack messages.

```http
POST /webhook/slack/interactions
Content-Type: application/x-www-form-urlencoded
X-Slack-Signature: v0=<hex_hmac_sha256>
X-Slack-Request-Timestamp: <unix_timestamp>
```

This endpoint handles:
- Acknowledge button clicks
- Add note actions
- Silence duration selections

**Request:** Form-encoded Slack interaction payload with `payload` field containing JSON.

**Response:**
```json
{
  "response_type": "in_channel",
  "text": "Alert acknowledged"
}
```

### Slack Events

Receive events from Slack Event API.

```http
POST /webhook/slack/events
Content-Type: application/json
X-Slack-Signature: v0=<hex_hmac_sha256>
X-Slack-Request-Timestamp: <unix_timestamp>
```

**URL Verification Request:**
```json
{
  "type": "url_verification",
  "challenge": "challenge_token",
  "token": "verification_token"
}
```

**URL Verification Response:**
```json
{
  "challenge": "challenge_token"
}
```

**Event Callback Request:**
```json
{
  "type": "event_callback",
  "event": {
    "type": "app_mention",
    "text": "@AlertBridge help",
    "user": "U123456",
    "channel": "C123456"
  }
}
```

**Event Callback Response:**
```json
{
  "ok": true
}
```

### Slack App Configuration

Configure your Slack App:

1. **Slash Commands**
   - Command: `/alert-status`
   - Request URL: `https://your-domain.com/webhook/slack/commands`
   - Short Description: Check current alert status
   - Usage Hint: `[critical|warning|info]`

   - Command: `/summary`
   - Request URL: `https://your-domain.com/webhook/slack/commands`
   - Short Description: Get alert summary statistics
   - Usage Hint: `[1h|24h|7d|1w|today|week|all]`

2. **Interactivity & Shortcuts**
   - Request URL: `https://your-domain.com/webhook/slack/interactions`

3. **Event Subscriptions**
   - Request URL: `https://your-domain.com/webhook/slack/events`
   - Subscribe to bot events: `app_mention`, `message.channels`

4. **OAuth & Permissions**
   - Bot Token Scopes: `chat:write`, `chat:write.public`, `commands`, `reactions:write`

## PagerDuty Integration

### PagerDuty Webhook

Receive incident updates from PagerDuty (V3 webhooks).

```http
POST /webhook/pagerduty
Content-Type: application/json
X-PagerDuty-Signature: v1=<hex_hmac_sha256>
```

**Request Body:**
```json
{
  "messages": [
    {
      "event": {
        "event_type": "incident.acknowledged",
        "data": {
          "id": "PINC123",
          "incident_key": "alert-fingerprint-abc123",
          "status": "acknowledged",
          "title": "High CPU Alert",
          "acknowledgers": [
            {
              "acknowledger": {
                "id": "PUSER123",
                "email": "oncall@example.com",
                "summary": "On-Call Engineer"
              }
            }
          ]
        },
        "agent": {
          "id": "PUSER123",
          "email": "oncall@example.com",
          "name": "On-Call Engineer"
        }
      }
    }
  ]
}
```

**Supported Event Types:**
- `incident.acknowledged` - Incident was acknowledged
- `incident.resolved` - Incident was resolved

**Response:**
```json
{
  "status": "ok",
  "processed": 1,
  "skipped": 0
}
```

### PagerDuty Webhook Setup

1. Navigate to **Integrations -> Generic Webhooks (v3)** in PagerDuty
2. Click **+ New Webhook**
3. Set **Destination URL**: `https://your-alert-bridge.example.com/webhook/pagerduty`
4. Subscribe to events:
   - `incident.acknowledged`
   - `incident.resolved`
5. Copy the **Webhook Secret** (format: `whsec_...`)
6. Configure the secret in Alert-Bridge:
   ```yaml
   pagerduty:
     webhook_secret: "whsec_..."
   ```

## Authentication

### Slack Request Verification

All Slack webhook endpoints verify requests using the Slack signing secret:

1. Slack sends `X-Slack-Signature` and `X-Slack-Request-Timestamp` headers
2. Alert-Bridge computes expected signature: `v0=HMAC-SHA256(signing_secret, "v0:{timestamp}:{body}")`
3. Request is rejected if signature doesn't match or timestamp is >5 minutes old

### PagerDuty Request Verification

PagerDuty webhook requests are verified using HMAC-SHA256:

1. PagerDuty sends `X-PagerDuty-Signature` header
2. Alert-Bridge computes expected signature using the webhook secret
3. Request is rejected if signature doesn't match

### Alertmanager Authentication (Optional)

When `alertmanager.webhook_secret` is configured:

1. Expects `X-Alertmanager-Signature: v1=<hex_hmac_sha256>` header
2. Computes HMAC-SHA256 of request body with the shared secret
3. Rejects requests with invalid or missing signatures

## Error Responses

All endpoints return consistent error responses:

**400 Bad Request:**
```json
{
  "error": "invalid payload"
}
```

**401 Unauthorized:**
```json
{
  "error": "invalid signature"
}
```

**405 Method Not Allowed:**
```json
{
  "error": "method not allowed"
}
```

**500 Internal Server Error:**
```json
{
  "error": "internal server error"
}
```

## Next Steps

- [Installation](installation.md) - Set up the application
- [Deployment](deployment.md) - Deploy to production
- [Storage Options](storage.md) - Configure persistent storage
