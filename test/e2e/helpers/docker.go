package helpers

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// WaitForService waits for a service to become healthy
func WaitForService(t *testing.T, healthURL string, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Service %s did not become healthy within %v", healthURL, timeout)
		case <-ticker.C:
			resp, err := http.Get(healthURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				t.Logf("✓ Service %s is healthy", healthURL)
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// WaitForAllServices waits for all E2E services to become healthy
func WaitForAllServices(t *testing.T) {
	t.Helper()

	services := map[string]string{
		"prometheus":     "http://localhost:9000/-/healthy",
		"alertmanager":   "http://localhost:9093/-/healthy",
		"alert-bridge":   "http://localhost:9080/health",
		"mock-slack":     "http://localhost:9091/health",
		"mock-pagerduty": "http://localhost:9092/health",
	}

	timeout := 60 * time.Second

	for name, url := range services {
		t.Logf("Waiting for %s to become healthy...", name)
		WaitForService(t, url, timeout)
	}

	t.Log("✓ All services are healthy")
}

// ResetMockServices resets the state of all mock services
func ResetMockServices(t *testing.T) {
	t.Helper()

	// Reset mock Slack
	resp, err := http.Post("http://localhost:9091/api/test/reset", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to reset mock Slack: %v", err)
	}
	resp.Body.Close()

	// Reset mock PagerDuty
	resp, err = http.Post("http://localhost:9092/api/test/reset", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to reset mock PagerDuty: %v", err)
	}
	resp.Body.Close()

	t.Log("✓ Mock services reset")
}

// ServiceHealthCheck performs a health check on a service
func ServiceHealthCheck(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}
