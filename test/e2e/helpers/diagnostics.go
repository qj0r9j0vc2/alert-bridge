package helpers

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// DiagnosticsCollector collects diagnostic information for failed tests
type DiagnosticsCollector struct {
	outputDir string
}

// NewDiagnosticsCollector creates a new diagnostics collector
func NewDiagnosticsCollector(outputDir string) *DiagnosticsCollector {
	return &DiagnosticsCollector{
		outputDir: outputDir,
	}
}

// CollectAll collects all diagnostic information
func (d *DiagnosticsCollector) CollectAll(t *testing.T) {
	t.Helper()

	if err := os.MkdirAll(d.outputDir, 0755); err != nil {
		t.Logf("Failed to create diagnostics directory: %v", err)
		return
	}

	t.Logf("Collecting diagnostics to: %s", d.outputDir)

	// Collect service logs
	d.CollectServiceLogs(t)

	// Collect container states
	d.CollectContainerStates(t)

	// Collect test trace (if available)
	d.CollectTestTrace(t)

	t.Logf("✓ Diagnostics collected")
}

// CollectServiceLogs collects logs from all Docker services
func (d *DiagnosticsCollector) CollectServiceLogs(t *testing.T) {
	t.Helper()

	servicesDir := filepath.Join(d.outputDir, "services")
	if err := os.MkdirAll(servicesDir, 0755); err != nil {
		t.Logf("Failed to create services directory: %v", err)
		return
	}

	services := []string{
		"e2e-prometheus",
		"e2e-alertmanager",
		"e2e-alert-bridge",
		"e2e-mock-slack",
		"e2e-mock-pagerduty",
	}

	for _, service := range services {
		logFile := filepath.Join(servicesDir, service+".log")
		cmd := exec.Command("docker", "logs", service)

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Failed to collect logs for %s: %v", service, err)
			continue
		}

		if err := os.WriteFile(logFile, output, 0644); err != nil {
			t.Logf("Failed to write log file for %s: %v", service, err)
			continue
		}

		t.Logf("✓ Collected logs for %s", service)
	}
}

// CollectContainerStates collects Docker container states
func (d *DiagnosticsCollector) CollectContainerStates(t *testing.T) {
	t.Helper()

	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=e2e-", "--format", "{{json .}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Failed to collect container states: %v", err)
		return
	}

	stateFile := filepath.Join(d.outputDir, "containers.json")
	if err := os.WriteFile(stateFile, output, 0644); err != nil {
		t.Logf("Failed to write container states: %v", err)
		return
	}

	t.Logf("✓ Collected container states")
}

// CollectTestTrace collects test execution trace
func (d *DiagnosticsCollector) CollectTestTrace(t *testing.T) {
	t.Helper()

	trace := map[string]interface{}{
		"test_name":    t.Name(),
		"failed":       t.Failed(),
		"collected_at": time.Now().Format(time.RFC3339),
	}

	traceJSON, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		t.Logf("Failed to marshal test trace: %v", err)
		return
	}

	traceFile := filepath.Join(d.outputDir, "test-trace.json")
	if err := os.WriteFile(traceFile, traceJSON, 0644); err != nil {
		t.Logf("Failed to write test trace: %v", err)
		return
	}

	t.Logf("✓ Collected test trace")
}

// LogTestPhase logs a test phase for trace collection
func LogTestPhase(t *testing.T, phase string) {
	t.Helper()
	timestamp := time.Now().Format(time.RFC3339)
	t.Logf("[TRACE] %s | %s | %s", t.Name(), phase, timestamp)
}

// CaptureFailureDiagnostics captures diagnostics when a test fails
func CaptureFailureDiagnostics(t *testing.T, worktreeDir string) {
	t.Helper()

	if !t.Failed() {
		return
	}

	diagnosticsDir := filepath.Join(worktreeDir, "diagnostics")
	collector := NewDiagnosticsCollector(diagnosticsDir)
	collector.CollectAll(t)

	t.Logf("Diagnostics saved to: %s", diagnosticsDir)
	t.Logf("View logs: cat %s/services/*.log", diagnosticsDir)
	t.Logf("View containers: cat %s/containers.json | jq", diagnosticsDir)
}

// PrintServiceURLs prints URLs for manual testing
func PrintServiceURLs(t *testing.T) {
	t.Helper()

	t.Log("Service URLs:")
	t.Log("  Prometheus:     http://localhost:9000")
	t.Log("  Alertmanager:   http://localhost:9093")
	t.Log("  Alert-Bridge:   http://localhost:9080")
	t.Log("  Mock Slack:     http://localhost:9091")
	t.Log("  Mock PagerDuty: http://localhost:9092")
}

// DumpMockServiceState dumps the current state of mock services for debugging
func DumpMockServiceState(t *testing.T) {
	t.Helper()

	// Dump Slack messages
	slackMessages := QueryMockSlack(t, "")
	t.Logf("Mock Slack messages: %d", len(slackMessages))
	for i, msg := range slackMessages {
		t.Logf("  [%d] %s: %s", i, msg.MessageID, msg.Text)
	}

	// Dump PagerDuty events
	pdEvents := QueryMockPagerDuty(t, "", "")
	t.Logf("Mock PagerDuty events: %d", len(pdEvents))
	for i, evt := range pdEvents {
		t.Logf("  [%d] %s: %s (%s)", i, evt.EventID, evt.DedupKey, evt.EventAction)
	}
}

// QueryMockSlack queries mock Slack service
func QueryMockSlack(t *testing.T, fingerprint string) []SlackMessage {
	t.Helper()

	url := "http://localhost:9091/api/test/messages"
	if fingerprint != "" {
		url += "?fingerprint=" + fingerprint
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Logf("Failed to query mock Slack: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var messages []SlackMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		t.Logf("Failed to decode Slack messages: %v", err)
		return nil
	}

	return messages
}

// QueryMockPagerDuty queries mock PagerDuty service
func QueryMockPagerDuty(t *testing.T, dedupKey string, action string) []PagerDutyEvent {
	t.Helper()

	url := "http://localhost:9092/api/test/events"
	if dedupKey != "" {
		url += "?dedup_key=" + dedupKey
	}
	if action != "" {
		if dedupKey != "" {
			url += "&action=" + action
		} else {
			url += "?action=" + action
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Logf("Failed to query mock PagerDuty: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var events []PagerDutyEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Logf("Failed to decode PagerDuty events: %v", err)
		return nil
	}

	return events
}
