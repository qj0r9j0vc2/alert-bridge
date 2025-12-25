package pagerduty

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// Client wraps the PagerDuty API client with domain-specific operations.
// Implements both alert.Notifier and ack.AckSyncer interfaces.
type Client struct {
	eventsClient    *pagerduty.Client
	routingKey      string
	serviceID       string
	fromEmail       string
	defaultSeverity string
	retryPolicy     *RetryPolicy
	restClient      RESTClient
	healthChecker   HealthChecker
}

// NewClient creates a new PagerDuty client with retry policy and optional REST API features.
func NewClient(apiToken, routingKey, serviceID, fromEmail, defaultSeverity string) *Client {
	var eventsClient *pagerduty.Client
	if apiToken != "" {
		eventsClient = pagerduty.NewClient(apiToken)
	}

	if defaultSeverity == "" {
		defaultSeverity = "warning"
	}

	// Initialize retry policy with exponential backoff
	retryPolicy := DefaultRetryPolicy()

	// Initialize REST API client if api_token is provided
	var restClient RESTClient
	if apiToken != "" {
		restClient = NewRESTClient(apiToken, fromEmail, retryPolicy)
		slog.Info("PagerDuty REST API client initialized", "from_email", fromEmail)
	}

	// Initialize health checker if both api_token and service_id are provided
	var healthChecker HealthChecker
	if apiToken != "" && serviceID != "" {
		healthChecker = NewHealthChecker(restClient, serviceID)
		slog.Info("PagerDuty health checker initialized", "service_id", serviceID)
	}

	return &Client{
		eventsClient:    eventsClient,
		routingKey:      routingKey,
		serviceID:       serviceID,
		fromEmail:       fromEmail,
		defaultSeverity: defaultSeverity,
		retryPolicy:     retryPolicy,
		restClient:      restClient,
		healthChecker:   healthChecker,
	}
}

// Notify creates a PagerDuty incident for an alert.
// Returns the incident/dedup key as message ID.
func (c *Client) Notify(ctx context.Context, alert *entity.Alert) (string, error) {
	if c.routingKey == "" {
		return "", fmt.Errorf("pagerduty routing key not configured")
	}

	// Build the event
	event := &pagerduty.V2Event{
		RoutingKey: c.routingKey,
		Action:     "trigger",
		DedupKey:   c.buildDedupKey(alert),
		Payload: &pagerduty.V2Payload{
			Summary:   c.buildSummary(alert),
			Source:    alert.Instance,
			Severity:  c.mapSeverity(alert.Severity),
			Timestamp: alert.FiredAt.Format("2006-01-02T15:04:05.000Z"),
			Component: alert.Target,
			Group:     alert.GetLabel("job"),
			Class:     alert.Name,
			Details:   c.buildDetails(alert),
		},
	}

	// Add custom details
	if event.Payload.Details == nil {
		event.Payload.Details = make(map[string]interface{})
	}

	// Send the event with retry logic
	var resp *pagerduty.V2EventResponse
	startTime := time.Now()

	err := c.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
		var retryErr error
		resp, retryErr = pagerduty.ManageEventWithContext(ctx, *event)
		return retryErr
	})

	responseTime := time.Since(startTime)

	if err != nil {
		slog.Error("Failed to send PagerDuty event after retries",
			"alert_id", alert.ID,
			"alert_name", alert.Name,
			"action", "trigger",
			"response_time", responseTime,
			"error", err)
		return "", fmt.Errorf("sending pagerduty event: %w", err)
	}

	slog.Info("PagerDuty event sent successfully",
		"alert_id", alert.ID,
		"alert_name", alert.Name,
		"action", "trigger",
		"dedup_key", resp.DedupKey,
		"response_time", responseTime)

	// Return dedup key as the incident identifier
	return resp.DedupKey, nil
}

// UpdateMessage updates an existing PagerDuty incident.
// For resolved alerts, it sends a resolve event.
func (c *Client) UpdateMessage(ctx context.Context, dedupKey string, alert *entity.Alert) error {
	if c.routingKey == "" {
		return fmt.Errorf("pagerduty routing key not configured")
	}

	action := "trigger"
	if alert.IsResolved() {
		action = "resolve"
	} else if alert.IsAcked() {
		action = "acknowledge"
	}

	event := &pagerduty.V2Event{
		RoutingKey: c.routingKey,
		Action:     action,
		DedupKey:   dedupKey,
	}

	// For trigger/acknowledge, include payload
	if action != "resolve" {
		event.Payload = &pagerduty.V2Payload{
			Summary:  c.buildSummary(alert),
			Source:   alert.Instance,
			Severity: c.mapSeverity(alert.Severity),
		}
	}

	_, err := pagerduty.ManageEventWithContext(ctx, *event)
	if err != nil {
		return fmt.Errorf("updating pagerduty event: %w", err)
	}

	return nil
}

// Acknowledge acknowledges an incident in PagerDuty via Events API v2.
func (c *Client) Acknowledge(ctx context.Context, alert *entity.Alert, ackEvent *entity.AckEvent) error {
	if c.routingKey == "" {
		return fmt.Errorf("pagerduty routing key not configured")
	}

	dedupKey := alert.PagerDutyIncidentID
	if dedupKey == "" {
		dedupKey = c.buildDedupKey(alert)
	}

	event := &pagerduty.V2Event{
		RoutingKey: c.routingKey,
		Action:     "acknowledge",
		DedupKey:   dedupKey,
	}

	// Acknowledge with retry logic
	startTime := time.Now()

	err := c.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
		_, retryErr := pagerduty.ManageEventWithContext(ctx, *event)
		return retryErr
	})

	responseTime := time.Since(startTime)

	if err != nil {
		slog.Error("Failed to acknowledge PagerDuty event after retries",
			"alert_id", alert.ID,
			"alert_name", alert.Name,
			"dedup_key", dedupKey,
			"response_time", responseTime,
			"error", err)
		return fmt.Errorf("acknowledging pagerduty event: %w", err)
	}

	slog.Info("PagerDuty event acknowledged successfully",
		"alert_id", alert.ID,
		"alert_name", alert.Name,
		"dedup_key", dedupKey,
		"response_time", responseTime)

	// Add incident note if REST API is available and note exists
	if c.restClient != nil && ackEvent.Note != "" {
		c.createIncidentNote(ctx, dedupKey, alert, ackEvent)
	}

	return nil
}

// createIncidentNote creates an incident note via REST API (non-blocking).
// Errors are logged but do not fail the acknowledge operation.
func (c *Client) createIncidentNote(ctx context.Context, incidentID string, alert *entity.Alert, ackEvent *entity.AckEvent) {
	noteContent := formatIncidentNote(alert, ackEvent, c.fromEmail)
	noteStartTime := time.Now()

	note, err := c.restClient.CreateIncidentNote(ctx, incidentID, noteContent)
	noteResponseTime := time.Since(noteStartTime)

	if err != nil {
		slog.Warn("Failed to create incident note (ack still succeeded)",
			"alert_id", alert.ID,
			"incident_id", incidentID,
			"response_time", noteResponseTime,
			"error", err)

		// Trigger health check after REST API failure
		if c.healthChecker != nil {
			go func() {
				if healthErr := c.RunHealthCheck(context.Background()); healthErr != nil {
					slog.Debug("Post-failure health check completed", "healthy", false)
				}
			}()
		}
		return
	}

	slog.Info("Incident note created successfully",
		"alert_id", alert.ID,
		"incident_id", incidentID,
		"note_id", note.ID,
		"user", ackEvent.UserName,
		"response_time", noteResponseTime)
}

// Resolve resolves an incident in PagerDuty.
func (c *Client) Resolve(ctx context.Context, alert *entity.Alert) error {
	if c.routingKey == "" {
		return fmt.Errorf("pagerduty routing key not configured")
	}

	dedupKey := alert.PagerDutyIncidentID
	if dedupKey == "" {
		dedupKey = c.buildDedupKey(alert)
	}

	event := &pagerduty.V2Event{
		RoutingKey: c.routingKey,
		Action:     "resolve",
		DedupKey:   dedupKey,
	}

	// Resolve with retry logic
	startTime := time.Now()

	err := c.retryPolicy.WithRetry(ctx, func(ctx context.Context) error {
		_, retryErr := pagerduty.ManageEventWithContext(ctx, *event)
		return retryErr
	})

	responseTime := time.Since(startTime)

	if err != nil {
		slog.Error("Failed to resolve PagerDuty event after retries",
			"alert_id", alert.ID,
			"alert_name", alert.Name,
			"dedup_key", dedupKey,
			"response_time", responseTime,
			"error", err)
		return fmt.Errorf("resolving pagerduty event: %w", err)
	}

	slog.Info("PagerDuty event resolved successfully",
		"alert_id", alert.ID,
		"alert_name", alert.Name,
		"dedup_key", dedupKey,
		"response_time", responseTime)

	return nil
}

// RunHealthCheck performs an on-demand health check if health checker is available.
// Returns error if health check fails, nil if check passes or checker unavailable.
func (c *Client) RunHealthCheck(ctx context.Context) error {
	if c.healthChecker == nil {
		slog.Debug("PagerDuty health check skipped: health checker not configured")
		return nil
	}

	result, err := c.healthChecker.Check(ctx)
	if err != nil {
		slog.Error("PagerDuty health check failed",
			"service_id", c.serviceID,
			"error", err)
		return err
	}

	if result != nil {
		slog.Info("PagerDuty health check completed",
			"healthy", result.Healthy,
			"service_id", result.ServiceID,
			"service_name", result.ServiceName,
			"response_time", result.ResponseTime,
			"checked_at", result.CheckedAt)
	}

	return nil
}

// Name returns the notifier identifier.
func (c *Client) Name() string {
	return "pagerduty"
}

// SupportsAck returns true as PagerDuty supports acknowledgment.
func (c *Client) SupportsAck() bool {
	return true
}

// buildDedupKey creates a deduplication key for the alert.
func (c *Client) buildDedupKey(alert *entity.Alert) string {
	// Use fingerprint if available, otherwise use alert ID
	if alert.Fingerprint != "" {
		return alert.Fingerprint
	}
	return alert.ID
}

// buildSummary creates the incident summary.
func (c *Client) buildSummary(alert *entity.Alert) string {
	var parts []string

	// Add severity prefix
	switch alert.Severity {
	case entity.SeverityCritical:
		parts = append(parts, "[CRITICAL]")
	case entity.SeverityWarning:
		parts = append(parts, "[WARNING]")
	default:
		parts = append(parts, "[INFO]")
	}

	// Add alert name
	parts = append(parts, alert.Name)

	// Add instance if available
	if alert.Instance != "" {
		parts = append(parts, fmt.Sprintf("on %s", alert.Instance))
	}

	// Add summary if available
	if alert.Summary != "" {
		parts = append(parts, "-", alert.Summary)
	}

	return strings.Join(parts, " ")
}

// buildDetails creates the incident details map.
func (c *Client) buildDetails(alert *entity.Alert) map[string]interface{} {
	details := map[string]interface{}{
		"alert_id":    alert.ID,
		"fingerprint": alert.Fingerprint,
		"name":        alert.Name,
		"instance":    alert.Instance,
		"target":      alert.Target,
		"severity":    string(alert.Severity),
		"state":       string(alert.State),
		"fired_at":    alert.FiredAt.Format("2006-01-02T15:04:05Z"),
	}

	if alert.Summary != "" {
		details["summary"] = alert.Summary
	}
	if alert.Description != "" {
		details["description"] = alert.Description
	}

	// Add labels
	if len(alert.Labels) > 0 {
		details["labels"] = alert.Labels
	}

	// Add annotations
	if len(alert.Annotations) > 0 {
		details["annotations"] = alert.Annotations
	}

	return details
}

// mapSeverity maps alert severity to PagerDuty severity.
func (c *Client) mapSeverity(severity entity.AlertSeverity) string {
	switch severity {
	case entity.SeverityCritical:
		return "critical"
	case entity.SeverityWarning:
		return "warning"
	default:
		return c.defaultSeverity
	}
}
