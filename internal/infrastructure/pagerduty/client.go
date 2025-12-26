package pagerduty

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/PagerDuty/go-pagerduty"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	domainerrors "github.com/qj0r9j0vc2/alert-bridge/internal/domain/errors"
)

// Client wraps the PagerDuty API client with domain-specific operations.
// Implements both alert.Notifier and ack.AckSyncer interfaces.
type Client struct {
	eventsClient    *pagerduty.Client
	routingKey      string
	serviceID       string
	fromEmail       string
	defaultSeverity string
	eventsAPIURL    string // Optional: for E2E testing with mock services
}

// NewClient creates a new PagerDuty client.
func NewClient(apiToken, routingKey, serviceID, fromEmail, defaultSeverity string, eventsAPIURL ...string) *Client {
	var client *pagerduty.Client
	if apiToken != "" {
		client = pagerduty.NewClient(apiToken)
	}

	if defaultSeverity == "" {
		defaultSeverity = "warning"
	}

	var apiURL string
	if len(eventsAPIURL) > 0 {
		apiURL = eventsAPIURL[0]
	}

	return &Client{
		eventsClient:    client,
		routingKey:      routingKey,
		serviceID:       serviceID,
		fromEmail:       fromEmail,
		defaultSeverity: defaultSeverity,
		eventsAPIURL:    apiURL,
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

	// Send the event
	var resp *pagerduty.V2EventResponse
	var err error

	if c.eventsAPIURL != "" {
		// Use custom Events API endpoint (for E2E testing)
		resp, err = c.sendEventHTTP(ctx, event)
	} else {
		// Use official PagerDuty library
		resp, err = pagerduty.ManageEventWithContext(ctx, *event)
	}

	if err != nil {
		return "", categorizePagerDutyError(err, "sending pagerduty event")
	}

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

	if c.eventsAPIURL != "" {
		// Use custom Events API endpoint (for E2E testing)
		_, err := c.sendEventHTTP(ctx, event)
		if err != nil {
			return categorizePagerDutyError(err, "updating pagerduty event")
		}
	} else {
		// Use official PagerDuty library
		_, err := pagerduty.ManageEventWithContext(ctx, *event)
		if err != nil {
			return categorizePagerDutyError(err, "updating pagerduty event")
		}
	}

	return nil
}

// Acknowledge acknowledges an incident in PagerDuty via Events API v2.
func (c *Client) Acknowledge(ctx context.Context, alert *entity.Alert, ackEvent *entity.AckEvent) error {
	if c.routingKey == "" {
		return fmt.Errorf("pagerduty routing key not configured")
	}

	dedupKey := alert.GetExternalReference("pagerduty")
	if dedupKey == "" {
		dedupKey = c.buildDedupKey(alert)
	}

	event := &pagerduty.V2Event{
		RoutingKey: c.routingKey,
		Action:     "acknowledge",
		DedupKey:   dedupKey,
	}

	_, err := pagerduty.ManageEventWithContext(ctx, *event)
	if err != nil {
		return categorizePagerDutyError(err, "acknowledging pagerduty event")
	}

	return nil
}

// Resolve resolves an incident in PagerDuty.
func (c *Client) Resolve(ctx context.Context, alert *entity.Alert) error {
	if c.routingKey == "" {
		return fmt.Errorf("pagerduty routing key not configured")
	}

	dedupKey := alert.GetExternalReference("pagerduty")
	if dedupKey == "" {
		dedupKey = c.buildDedupKey(alert)
	}

	event := &pagerduty.V2Event{
		RoutingKey: c.routingKey,
		Action:     "resolve",
		DedupKey:   dedupKey,
	}

	_, err := pagerduty.ManageEventWithContext(ctx, *event)
	if err != nil {
		return categorizePagerDutyError(err, "resolving pagerduty event")
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

// sendEventHTTP sends an event to a custom PagerDuty Events API endpoint via HTTP.
// Used for E2E testing with mock services.
func (c *Client) sendEventHTTP(ctx context.Context, event *pagerduty.V2Event) (*pagerduty.V2EventResponse, error) {
	// Marshal event to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshaling event: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.eventsAPIURL+"/v2/enqueue", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP response with status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var eventResp pagerduty.V2EventResponse
	if err := json.Unmarshal(body, &eventResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w, body: %s", err, string(body))
	}

	return &eventResp, nil
}

// categorizePagerDutyError wraps PagerDuty API errors as transient or permanent domain errors.
func categorizePagerDutyError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Check for network errors (transient)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return domainerrors.NewTransientError(
			fmt.Sprintf("%s: network error", operation),
			err,
		)
	}

	// Check for PagerDuty API errors
	var pdErr pagerduty.APIError
	if errors.As(err, &pdErr) {
		// Rate limiting (HTTP 429) - transient
		if pdErr.StatusCode == 429 {
			return domainerrors.NewTransientError(
				fmt.Sprintf("%s: rate limited", operation),
				err,
			)
		}

		// Server errors (5xx) - transient
		if pdErr.StatusCode >= 500 && pdErr.StatusCode < 600 {
			return domainerrors.NewTransientError(
				fmt.Sprintf("%s: pagerduty server error (status %d)", operation, pdErr.StatusCode),
				err,
			)
		}

		// Client errors (4xx) - permanent
		if pdErr.StatusCode >= 400 && pdErr.StatusCode < 500 {
			return domainerrors.NewPermanentError(
				fmt.Sprintf("%s: client error (status %d)", operation, pdErr.StatusCode),
				err,
			)
		}
	}

	// Check for context errors (transient)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return domainerrors.NewTransientError(
			fmt.Sprintf("%s: context timeout", operation),
			err,
		)
	}

	// Default to permanent error
	return domainerrors.NewPermanentError(
		fmt.Sprintf("%s: %v", operation, err),
		err,
	)
}
