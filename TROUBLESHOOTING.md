# Slack CLI Integration Troubleshooting

Common issues and solutions for Slack CLI integration with Alert Bridge.

## Socket Mode Issues

### Connection Failures

**Symptom**: Socket Mode client fails to connect or immediately disconnects.

**Possible Causes**:
1. Invalid or expired `SLACK_APP_TOKEN`
2. Token missing `connections:write` scope
3. Network connectivity issues
4. Firewall blocking WebSocket connections

**Solutions**:
```bash
# Verify token has correct scopes
slack auth test

# Check token in config
grep SLACK_APP_TOKEN config/config.yaml

# Test connectivity
curl -H "Authorization: Bearer $SLACK_APP_TOKEN" https://slack.com/api/apps.connections.open

# Enable debug logging
export SLACK_SOCKET_MODE_DEBUG=true
./alert-bridge
```

**Expected behavior**: Connection establishes within 2-3 seconds, health endpoint shows:
```json
{
  "slack": {
    "mode": "socket",
    "connected": true,
    "connection_id": "C01234567"
  }
}
```

### Frequent Reconnections

**Symptom**: Logs show repeated "reconnecting" messages.

**Possible Causes**:
1. Unstable network connection
2. Server-side rate limiting
3. Ping timeout too aggressive

**Solutions**:
```yaml
# Increase ping interval in config.yaml
slack:
  socket_mode:
    enabled: true
    ping_interval: 60s  # Increase from 30s
```

**Monitoring**:
```bash
# Check reconnection frequency
curl http://localhost:8080/health | jq '.slack.last_reconnect'

# Monitor logs
tail -f logs/alert-bridge.log | grep "Socket Mode"
```

## HTTP Mode Issues

### Signature Verification Failures

**Symptom**: Slack webhooks return 401 Unauthorized or signature verification errors.

**Possible Causes**:
1. Incorrect `SLACK_SIGNING_SECRET`
2. Request body already read/modified by middleware
3. Clock skew >5 minutes
4. Proxy/CDN modifying request

**Solutions**:
```bash
# Verify signing secret matches Slack app settings
# Go to: https://api.slack.com/apps/YOUR_APP_ID/general
# Compare with config value

# Check server time sync
timedatectl status  # Linux
ntpdate -q pool.ntp.org  # Verify NTP sync

# Test signature verification manually
curl -X POST http://localhost:8080/webhook/slack/commands \
  -H "X-Slack-Request-Timestamp: $(date +%s)" \
  -H "X-Slack-Signature: v0=COMPUTE_HMAC_SHA256" \
  -d "command=/alert-status&text=critical"
```

**Debug**:
```go
// Enable signature verification logging
// Edit internal/infrastructure/slack/signature_verifier.go
logger.Debug("signature verification",
  "timestamp", timestamp,
  "expected", expectedSig,
  "provided", providedSig)
```

### URL Verification Challenge

**Symptom**: Event subscriptions fail to enable with "verification failed" error.

**Solution**:
Ensure your server responds to Slack's challenge request:
```bash
# Slack sends:
POST /webhook/slack/events
{
  "type": "url_verification",
  "challenge": "3eZbrw1aBm2rZgRNFdxV2595E9CY3gmdALWMmHkvFXO7tYXAYM8P"
}

# Server must respond with:
HTTP/1.1 200 OK
Content-Type: application/json
{
  "challenge": "3eZbrw1aBm2rZgRNFdxV2595E9CY3gmdALWMmHkvFXO7tYXAYM8P"
}
```

Check SlackEventsHandler handles `url_verification` type correctly.

## Slash Command Issues

### Command Timeouts

**Symptom**: User sees "Operation timeout" error in Slack.

**Possible Causes**:
1. Handler takes >3s to respond (Slack timeout)
2. Database query slow
3. Response URL POST failed

**Solutions**:
```bash
# Check response times in logs
grep "slash command processed" logs/alert-bridge.log | jq '.response_time_ms'

# Verify immediate acknowledgment sent
# Should see "Fetching alert status..." immediately

# Check database performance
EXPLAIN SELECT * FROM alerts WHERE status = 'firing' AND severity = 'critical';

# Add indexes if needed
CREATE INDEX idx_alerts_status_severity ON alerts(status, severity);
```

**SLA Monitoring**:
```bash
# Track SLA compliance
grep "sla_met" logs/alert-bridge.log | grep "false"
```

### Commands Not Appearing

**Symptom**: `/alert-status` not available in Slack.

**Possible Causes**:
1. Manifest not deployed to Slack app
2. App not installed in workspace
3. Command scope limited to specific channels

**Solutions**:
```bash
# Validate manifest
./scripts/validate-manifest.sh

# Deploy manifest
slack deploy --source-dir . --app YOUR_APP_ID

# Verify installation
slack apps list
slack apps info YOUR_APP_ID

# Reinstall app if needed
# Go to: https://api.slack.com/apps/YOUR_APP_ID/install-on-team
```

## Permission Issues

### Missing Scopes

**Symptom**: API calls return `missing_scope` error.

**Required Scopes**:
- `chat:write` - Send messages
- `commands` - Register slash commands
- `connections:write` - Socket Mode (App token only)

**Solutions**:
```bash
# Check current scopes
slack auth test

# Update manifest with required scopes
vim manifest.json
# Add to oauth_config.scopes.bot: ["chat:write", "commands"]

# Redeploy and reinstall
slack deploy
# Reinstall via https://api.slack.com/apps/YOUR_APP_ID/install-on-team
```

## Configuration Issues

### Environment Variable Not Loaded

**Symptom**: Config values empty or default despite setting env vars.

**Solutions**:
```bash
# Verify environment variables set
env | grep SLACK

# Check config loading
export CONFIG_PATH=config/config.yaml
./alert-bridge

# Test config validation
go test ./internal/infrastructure/config -v -run TestValidate
```

### Mutual Exclusivity Violations

**Symptom**: Config validation fails with "socket_mode and http_mode cannot both be enabled".

**Solution**:
Choose ONE mode:
```yaml
# Socket Mode (local development)
slack:
  enabled: true
  bot_token: ${SLACK_BOT_TOKEN}
  socket_mode:
    enabled: true
    app_token: ${SLACK_APP_TOKEN}

# OR HTTP Mode (production)
slack:
  enabled: true
  bot_token: ${SLACK_BOT_TOKEN}
  signing_secret: ${SLACK_SIGNING_SECRET}
  socket_mode:
    enabled: false
```

## Manifest Issues

### Validation Failures

**Symptom**: `slack manifest validate` reports schema errors.

**Common Errors**:
1. Missing required fields (`display_information.name`)
2. Invalid URL format
3. Scope not in allowed list

**Solutions**:
```bash
# Validate locally
./scripts/validate-manifest.sh

# Check against schema
slack manifest validate --manifest manifest.json

# Compare with working example
curl https://api.slack.com/reference/manifests | jq .
```

### Deployment Sync Issues

**Symptom**: Changes to manifest.json not reflected in Slack app.

**Solutions**:
```bash
# Check deployment status
slack deploy --source-dir . --app YOUR_APP_ID --verbose

# Verify manifest matches deployed version
slack manifest diff --source manifest.json --app YOUR_APP_ID

# Force redeploy
slack deploy --source-dir . --app YOUR_APP_ID --force
```

## Health Check Issues

### Health Endpoint Returns Unhealthy

**Symptom**: `GET /health` shows `"slack": {"connected": false}`.

**Debug Steps**:
```bash
# Check health endpoint
curl http://localhost:8080/health | jq .

# Verify Socket Mode goroutine running
curl http://localhost:8080/health | jq '.slack.connection_id'
# Should show non-empty string if connected

# Check logs for connection errors
grep "Socket Mode" logs/alert-bridge.log | tail -20
```

## Performance Issues

### High Latency on Slash Commands

**Symptom**: Commands take >1s to respond (SLA: <2s).

**Optimization**:
```bash
# Profile database queries
go test -bench=BenchmarkGetActiveAlerts -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Add database indexes
CREATE INDEX idx_alerts_active ON alerts(status, severity, created_at DESC);

# Limit result set
SELECT * FROM alerts WHERE status = 'firing' LIMIT 10;

# Cache frequently accessed data
# Consider Redis for active alerts
```

## CI/CD Issues

### GitHub Actions Manifest Sync Fails

**Symptom**: `slack-manifest-sync.yml` workflow fails.

**Possible Causes**:
1. Missing `SLACK_APP_ID` secret
2. Missing `SLACK_BOT_TOKEN` secret
3. Insufficient permissions

**Solutions**:
```bash
# Verify GitHub secrets exist
# Go to: https://github.com/YOUR_ORG/alert-bridge/settings/secrets/actions

# Required secrets:
# - SLACK_APP_ID
# - SLACK_BOT_TOKEN (with apps:write scope)

# Test locally
SLACK_APP_ID=A01234567 \
SLACK_BOT_TOKEN=xoxb-... \
slack manifest validate --manifest manifest.json
```

## Getting Help

If issues persist after trying these solutions:

1. **Enable Debug Logging**:
   ```yaml
   slack:
     socket_mode:
       debug: true
   ```

2. **Check Slack API Status**: https://status.slack.com/

3. **Review Slack API Logs**: https://api.slack.com/apps/YOUR_APP_ID/event-subscriptions

4. **File an Issue**: https://github.com/qj0r9j0vc2/alert-bridge/issues

**Include in bug reports**:
- Full error message and stack trace
- Configuration (sanitize secrets)
- Health endpoint output
- Relevant log snippets
- Slack app configuration (scopes, event subscriptions)
