package helpers

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// Alert represents a Prometheus alert
type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt,omitempty"`
	Fingerprint string            `json:"fingerprint"`
}

// AlertmanagerWebhook represents the webhook payload sent by Alertmanager
type AlertmanagerWebhook struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []Alert           `json:"alerts"`
}

// CreateTestAlert creates a test alert from a fixture
func CreateTestAlert(fixtureName string, overrideLabels map[string]string) Alert {
	// Load fixture
	fixture := GetAlertFixture(fixtureName)

	// Override labels if provided
	if overrideLabels != nil {
		for k, v := range overrideLabels {
			fixture.Labels[k] = v
		}
	}

	// Set timestamps
	fixture.StartsAt = time.Now()

	// Generate fingerprint
	fixture.Fingerprint = GenerateFingerprint(fixture.Labels)

	return fixture
}

// SendAlertToAlertmanager sends an alert directly to Alertmanager
func SendAlertToAlertmanager(t *testing.T, alert Alert) {
	t.Helper()

	// Wrap in array (Alertmanager expects array of alerts)
	alerts := []Alert{alert}

	payload, err := json.Marshal(alerts)
	if err != nil {
		t.Fatalf("Failed to marshal alert: %v", err)
	}

	resp, err := http.Post(
		"http://localhost:9093/api/v1/alerts",
		"application/json",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		t.Fatalf("Failed to send alert to Alertmanager: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Alertmanager returned status %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("✓ Alert sent to Alertmanager: %s", alert.Labels["alertname"])
}

// SendAlertToAlertBridge sends an alert directly to Alert-Bridge webhook
func SendAlertToAlertBridge(t *testing.T, alerts []Alert) {
	t.Helper()

	webhook := AlertmanagerWebhook{
		Version:         "4",
		GroupKey:        "test-group",
		TruncatedAlerts: 0,
		Status:          "firing",
		Receiver:        "alert-bridge",
		GroupLabels:     make(map[string]string),
		CommonLabels:    make(map[string]string),
		Alerts:          alerts,
		ExternalURL:     "http://alertmanager:9093",
	}

	if len(alerts) > 0 {
		webhook.CommonLabels = alerts[0].Labels
		webhook.CommonAnnotations = alerts[0].Annotations
	}

	payload, err := json.Marshal(webhook)
	if err != nil {
		t.Fatalf("Failed to marshal webhook: %v", err)
	}

	resp, err := http.Post(
		"http://localhost:9080/webhook/alertmanager",
		"application/json",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		t.Fatalf("Failed to send alert to Alert-Bridge: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Alert-Bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("✓ Alert sent to Alert-Bridge webhook")
}

// ResolveAlert marks an alert as resolved
func ResolveAlert(t *testing.T, alert Alert) Alert {
	t.Helper()

	resolvedAlert := alert
	resolvedAlert.EndsAt = time.Now()

	return resolvedAlert
}

// GenerateFingerprint generates a fingerprint for an alert based on labels
func GenerateFingerprint(labels map[string]string) string {
	// Sort labels and create deterministic string
	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}

	// Simple fingerprint generation (in real Prometheus this is more complex)
	hash := sha256.New()
	for _, k := range keys {
		hash.Write([]byte(k))
		hash.Write([]byte(labels[k]))
	}

	return fmt.Sprintf("%x", hash.Sum(nil))[:16]
}

// WaitForAlertDelivery waits for an alert to be delivered to mock services
func WaitForAlertDelivery(t *testing.T, fingerprint string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check Slack
		resp, err := http.Get(fmt.Sprintf("http://localhost:9091/api/test/messages?fingerprint=%s", fingerprint))
		if err == nil {
			defer resp.Body.Close()
			var messages []interface{}
			json.NewDecoder(resp.Body).Decode(&messages)
			if len(messages) > 0 {
				t.Logf("✓ Alert delivered to Slack")
				return
			}
		}

		// Check PagerDuty
		resp, err = http.Get(fmt.Sprintf("http://localhost:9092/api/test/events?dedup_key=%s", fingerprint))
		if err == nil {
			defer resp.Body.Close()
			var events []interface{}
			json.NewDecoder(resp.Body).Decode(&events)
			if len(events) > 0 {
				t.Logf("✓ Alert delivered to PagerDuty")
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("Alert %s was not delivered within %v", fingerprint, timeout)
}
