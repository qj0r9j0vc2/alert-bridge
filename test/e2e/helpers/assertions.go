package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// SlackMessage represents a Slack message from the mock service
type SlackMessage struct {
	Channel    string        `json:"channel"`
	Text       string        `json:"text"`
	Blocks     []interface{} `json:"blocks"`
	MessageID  string        `json:"message_id"`
	ReceivedAt string        `json:"received_at"`
}

// PagerDutyEvent represents a PagerDuty event from the mock service
type PagerDutyEvent struct {
	RoutingKey  string                 `json:"routing_key"`
	EventAction string                 `json:"event_action"`
	DedupKey    string                 `json:"dedup_key"`
	Payload     map[string]interface{} `json:"payload"`
	EventID     string                 `json:"event_id"`
	ReceivedAt  string                 `json:"received_at"`
}

// AssertSlackMessageReceived asserts that a Slack message was received with the given fingerprint
func AssertSlackMessageReceived(t *testing.T, fingerprint string) SlackMessage {
	t.Helper()

	url := fmt.Sprintf("http://localhost:9091/api/test/messages?fingerprint=%s", fingerprint)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to query mock Slack: %v", err)
	}
	defer resp.Body.Close()

	var messages []SlackMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		t.Fatalf("Failed to decode Slack messages: %v", err)
	}

	if len(messages) == 0 {
		t.Fatalf("Expected Slack message with fingerprint %s, but none found", fingerprint)
	}

	if len(messages) > 1 {
		t.Logf("Warning: Found %d Slack messages with fingerprint %s, returning first", len(messages), fingerprint)
	}

	return messages[0]
}

// AssertSlackMessageCount asserts the number of Slack messages received
func AssertSlackMessageCount(t *testing.T, expected int, fingerprint string) {
	t.Helper()

	url := "http://localhost:9091/api/test/messages"
	if fingerprint != "" {
		url += "?fingerprint=" + fingerprint
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to query mock Slack: %v", err)
	}
	defer resp.Body.Close()

	var messages []SlackMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		t.Fatalf("Failed to decode Slack messages: %v", err)
	}

	if len(messages) != expected {
		t.Fatalf("Expected %d Slack messages, got %d", expected, len(messages))
	}

	t.Logf("✓ Slack message count: %d", len(messages))
}

// AssertSlackMessageContains asserts that a Slack message contains expected text
func AssertSlackMessageContains(t *testing.T, msg SlackMessage, expectedText string) {
	t.Helper()

	// Check in text field
	if contains(msg.Text, expectedText) {
		return
	}

	// Check in blocks
	blocksJSON, _ := json.Marshal(msg.Blocks)
	if contains(string(blocksJSON), expectedText) {
		return
	}

	t.Fatalf("Slack message does not contain expected text: %s", expectedText)
}

// AssertPagerDutyEventReceived asserts that a PagerDuty event was received
func AssertPagerDutyEventReceived(t *testing.T, dedupKey string, action string) PagerDutyEvent {
	t.Helper()

	url := fmt.Sprintf("http://localhost:9092/api/test/events?dedup_key=%s", dedupKey)
	if action != "" {
		url += "&action=" + action
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to query mock PagerDuty: %v", err)
	}
	defer resp.Body.Close()

	var events []PagerDutyEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("Failed to decode PagerDuty events: %v", err)
	}

	if len(events) == 0 {
		t.Fatalf("Expected PagerDuty event with dedup_key %s, but none found", dedupKey)
	}

	if len(events) > 1 {
		t.Logf("Warning: Found %d PagerDuty events with dedup_key %s, returning first", len(events), dedupKey)
	}

	return events[0]
}

// AssertPagerDutyEventCount asserts the number of PagerDuty events received
func AssertPagerDutyEventCount(t *testing.T, expected int, dedupKey string) {
	t.Helper()

	url := "http://localhost:9092/api/test/events"
	if dedupKey != "" {
		url += "?dedup_key=" + dedupKey
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to query mock PagerDuty: %v", err)
	}
	defer resp.Body.Close()

	var events []PagerDutyEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("Failed to decode PagerDuty events: %v", err)
	}

	if len(events) != expected {
		t.Fatalf("Expected %d PagerDuty events, got %d", expected, len(events))
	}

	t.Logf("✓ PagerDuty event count: %d", len(events))
}

// AssertPagerDutyEventAction asserts the action of a PagerDuty event
func AssertPagerDutyEventAction(t *testing.T, event PagerDutyEvent, expectedAction string) {
	t.Helper()

	if event.EventAction != expectedAction {
		t.Fatalf("Expected PagerDuty event action %s, got %s", expectedAction, event.EventAction)
	}

	t.Logf("✓ PagerDuty event action: %s", event.EventAction)
}

// AssertPagerDutyEventSeverity asserts the severity in the event payload
func AssertPagerDutyEventSeverity(t *testing.T, event PagerDutyEvent, expectedSeverity string) {
	t.Helper()

	if event.Payload == nil {
		t.Fatal("PagerDuty event payload is nil")
	}

	severity, ok := event.Payload["severity"].(string)
	if !ok {
		t.Fatal("PagerDuty event payload missing severity field")
	}

	if severity != expectedSeverity {
		t.Fatalf("Expected PagerDuty event severity %s, got %s", expectedSeverity, severity)
	}

	t.Logf("✓ PagerDuty event severity: %s", severity)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
