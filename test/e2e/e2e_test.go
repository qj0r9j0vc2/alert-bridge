package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/test/e2e/helpers"
)

// TestMain sets up and tears down the E2E test environment
func TestMain(m *testing.M) {
	// Note: Environment setup (worktree, Docker) is handled by scripts/e2e-setup.sh
	// This TestMain is for any Go-specific initialization

	// Initialize test reporter
	helpers.InitReporter("test-report.json")

	// Run tests
	exitCode := m.Run()

	// Save and print report
	if err := helpers.SaveReportIfInitialized(); err != nil {
		println("Failed to save test report:", err.Error())
	}
	helpers.PrintSummaryIfInitialized()

	os.Exit(exitCode)
}

// TestE2ESetup verifies the E2E environment is properly set up
func TestE2ESetup(t *testing.T) {
	helpers.LogTestPhase(t, "setup_environment")

	// Wait for all services to become healthy
	helpers.WaitForAllServices(t)

	// Reset mock services to clean state
	helpers.ResetMockServices(t)

	// Print service URLs for debugging
	helpers.PrintServiceURLs(t)

	t.Log("✓ E2E environment is ready")
}

// TestAlertCreationSlack tests alert delivery to Slack
func TestAlertCreationSlack(t *testing.T) {
	startTime := time.Now()
	if reporter := helpers.GetReporter(); reporter != nil {
		reporter.StartTest(t.Name())
		defer func() {
			reporter.RecordPhase("total_execution", startTime)
			reporter.EndTest(t)
		}()
	}

	helpers.LogTestPhase(t, "test_start")

	// Setup: Wait for services and reset state
	phaseStart := time.Now()
	helpers.WaitForAllServices(t)
	helpers.ResetMockServices(t)
	if reporter := helpers.GetReporter(); reporter != nil {
		reporter.RecordPhase("setup_environment", phaseStart)
	}

	// Create test alert
	helpers.LogTestPhase(t, "create_alert")
	alert := helpers.CreateTestAlert("high_cpu_critical", nil)

	// Send alert to Alertmanager
	helpers.LogTestPhase(t, "send_alert")
	helpers.SendAlertToAlertmanager(t, alert)

	// Wait for alert delivery
	helpers.LogTestPhase(t, "wait_for_delivery")
	time.Sleep(3 * time.Second) // Allow time for processing

	// Verify Slack message was received
	helpers.LogTestPhase(t, "verify_slack_delivery")
	msg := helpers.AssertSlackMessageReceived(t, alert.Fingerprint)

	// Verify message contains alert information
	helpers.LogTestPhase(t, "verify_message_content")
	helpers.AssertSlackMessageContains(t, msg, "HighCPU")
	helpers.AssertSlackMessageContains(t, msg, "server-01")

	helpers.LogTestPhase(t, "test_complete")
	t.Log("✓ Alert successfully delivered to Slack")
}

// TestAlertCreationPagerDuty tests alert delivery to PagerDuty
func TestAlertCreationPagerDuty(t *testing.T) {
	helpers.LogTestPhase(t, "test_start")

	// Setup: Wait for services and reset state
	helpers.WaitForAllServices(t)
	helpers.ResetMockServices(t)

	// Create test alert
	helpers.LogTestPhase(t, "create_alert")
	alert := helpers.CreateTestAlert("service_down_critical", nil)

	// Send alert to Alertmanager
	helpers.LogTestPhase(t, "send_alert")
	helpers.SendAlertToAlertmanager(t, alert)

	// Wait for alert delivery
	helpers.LogTestPhase(t, "wait_for_delivery")
	time.Sleep(3 * time.Second)

	// Verify PagerDuty event was received
	helpers.LogTestPhase(t, "verify_pagerduty_delivery")
	event := helpers.AssertPagerDutyEventReceived(t, alert.Fingerprint, "trigger")

	// Verify event properties
	helpers.LogTestPhase(t, "verify_event_properties")
	helpers.AssertPagerDutyEventAction(t, event, "trigger")
	helpers.AssertPagerDutyEventSeverity(t, event, "critical")

	helpers.LogTestPhase(t, "test_complete")
	t.Log("✓ Alert successfully delivered to PagerDuty")
}

// TestAlertDeduplication tests that duplicate alerts are not sent multiple times
func TestAlertDeduplication(t *testing.T) {
	helpers.LogTestPhase(t, "test_start")

	// Setup
	helpers.WaitForAllServices(t)
	helpers.ResetMockServices(t)

	// Create test alert
	helpers.LogTestPhase(t, "create_first_alert")
	alert := helpers.CreateTestAlert("duplicate_test_alert", nil)

	// Send first alert
	helpers.LogTestPhase(t, "send_first_alert")
	helpers.SendAlertToAlertmanager(t, alert)
	time.Sleep(2 * time.Second)

	// Send duplicate alert (same fingerprint)
	helpers.LogTestPhase(t, "send_duplicate_alert")
	helpers.SendAlertToAlertmanager(t, alert)
	time.Sleep(2 * time.Second)

	// Verify only one Slack message was sent
	helpers.LogTestPhase(t, "verify_deduplication")
	helpers.AssertSlackMessageCount(t, 1, alert.Fingerprint)

	helpers.LogTestPhase(t, "test_complete")
	t.Log("✓ Alert deduplication working correctly")
}

// TestAlertResolution tests alert resolution notifications
func TestAlertResolution(t *testing.T) {
	helpers.LogTestPhase(t, "test_start")

	// Setup
	helpers.WaitForAllServices(t)
	helpers.ResetMockServices(t)

	// Create and send firing alert
	helpers.LogTestPhase(t, "send_firing_alert")
	alert := helpers.CreateTestAlert("memory_pressure_warning", nil)
	helpers.SendAlertToAlertmanager(t, alert)
	time.Sleep(2 * time.Second)

	// Verify initial PagerDuty trigger event
	helpers.LogTestPhase(t, "verify_trigger_event")
	helpers.AssertPagerDutyEventReceived(t, alert.Fingerprint, "trigger")

	// Send resolved alert
	helpers.LogTestPhase(t, "send_resolved_alert")
	resolvedAlert := helpers.ResolveAlert(t, alert)
	helpers.SendAlertToAlertmanager(t, resolvedAlert)
	time.Sleep(3 * time.Second)

	// Verify PagerDuty resolve event was sent
	helpers.LogTestPhase(t, "verify_resolve_event")
	resolveEvent := helpers.AssertPagerDutyEventReceived(t, alert.Fingerprint, "resolve")
	helpers.AssertPagerDutyEventAction(t, resolveEvent, "resolve")

	// Verify Slack received resolution message
	helpers.LogTestPhase(t, "verify_slack_resolution")
	helpers.AssertSlackMessageCount(t, 2, alert.Fingerprint) // Firing + Resolved

	helpers.LogTestPhase(t, "test_complete")
	t.Log("✓ Alert resolution notifications working correctly")
}

// TestMultipleAlertsGrouping tests that multiple alerts are properly grouped
func TestMultipleAlertsGrouping(t *testing.T) {
	helpers.LogTestPhase(t, "test_start")

	// Setup
	helpers.WaitForAllServices(t)
	helpers.ResetMockServices(t)

	// Send multiple alerts of same severity
	helpers.LogTestPhase(t, "send_multiple_alerts")
	alert1 := helpers.CreateTestAlert("high_cpu_critical", nil)
	alert2 := helpers.CreateTestAlert("disk_space_critical", nil)
	alert3 := helpers.CreateTestAlert("backup_failed_critical", nil)

	helpers.SendAlertToAlertmanager(t, alert1)
	helpers.SendAlertToAlertmanager(t, alert2)
	helpers.SendAlertToAlertmanager(t, alert3)

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify all alerts were delivered
	helpers.LogTestPhase(t, "verify_all_delivered")
	helpers.AssertSlackMessageReceived(t, alert1.Fingerprint)
	helpers.AssertSlackMessageReceived(t, alert2.Fingerprint)
	helpers.AssertSlackMessageReceived(t, alert3.Fingerprint)

	helpers.LogTestPhase(t, "test_complete")
	t.Log("✓ Multiple alerts handled correctly")
}

// TestDifferentSeverityLevels tests alerts with different severity levels
func TestDifferentSeverityLevels(t *testing.T) {
	helpers.LogTestPhase(t, "test_start")

	// Setup
	helpers.WaitForAllServices(t)
	helpers.ResetMockServices(t)

	// Send critical alert
	helpers.LogTestPhase(t, "send_critical_alert")
	criticalAlert := helpers.CreateTestAlert("high_cpu_critical", nil)
	helpers.SendAlertToAlertmanager(t, criticalAlert)
	time.Sleep(2 * time.Second)

	// Send warning alert
	helpers.LogTestPhase(t, "send_warning_alert")
	warningAlert := helpers.CreateTestAlert("memory_pressure_warning", nil)
	helpers.SendAlertToAlertmanager(t, warningAlert)
	time.Sleep(2 * time.Second)

	// Verify both were delivered with correct severity
	helpers.LogTestPhase(t, "verify_critical_severity")
	criticalEvent := helpers.AssertPagerDutyEventReceived(t, criticalAlert.Fingerprint, "trigger")
	helpers.AssertPagerDutyEventSeverity(t, criticalEvent, "critical")

	helpers.LogTestPhase(t, "verify_warning_severity")
	warningEvent := helpers.AssertPagerDutyEventReceived(t, warningAlert.Fingerprint, "trigger")
	helpers.AssertPagerDutyEventSeverity(t, warningEvent, "warning")

	helpers.LogTestPhase(t, "test_complete")
	t.Log("✓ Different severity levels handled correctly")
}
