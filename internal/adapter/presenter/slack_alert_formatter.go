package presenter

import (
	"fmt"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	slackUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/slack"
	"github.com/slack-go/slack"
)

// SlackAlertFormatter formats alerts for Slack messages using Block Kit.
type SlackAlertFormatter struct{}

// NewSlackAlertFormatter creates a new Slack alert formatter.
func NewSlackAlertFormatter() *SlackAlertFormatter {
	return &SlackAlertFormatter{}
}

// FormatAlertStatus formats a list of alerts into Slack Block Kit blocks.
// Returns blocks ready to be included in a Slack message.
func (f *SlackAlertFormatter) FormatAlertStatus(alerts []*entity.Alert, severityFilter string) []slack.Block {
	blocks := []slack.Block{}

	// Header block
	headerText := "Alert Status Dashboard"
	if severityFilter != "" {
		headerText = fmt.Sprintf("Alert Status Dashboard - %s Alerts", f.formatSeverity(severityFilter))
	}

	blocks = append(blocks, slack.NewHeaderBlock(
		slack.NewTextBlockObject(slack.PlainTextType, headerText, false, false),
	))

	// Summary block
	summaryText := fmt.Sprintf("Total Active Alerts: %d", len(alerts))
	if severityFilter != "" {
		summaryText += fmt.Sprintf(" (filtered by severity: %s)", severityFilter)
	}

	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, summaryText, false, false),
		nil, nil,
	))

	// Divider
	blocks = append(blocks, slack.NewDividerBlock())

	// Alert sections
	if len(alerts) == 0 {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "No active alerts at this time.", false, false),
			nil, nil,
		))
	} else {
		for i, alert := range alerts {
			// Limit to 10 alerts in response
			if i >= 10 {
				blocks = append(blocks, slack.NewSectionBlock(
					slack.NewTextBlockObject(slack.MarkdownType,
						fmt.Sprintf("_Showing 10 of %d alerts_", len(alerts)),
						false, false),
					nil, nil,
				))
				break
			}

			blocks = append(blocks, f.formatAlert(alert))

			// Add divider between alerts (except after last one)
			if i < len(alerts)-1 && i < 9 {
				blocks = append(blocks, slack.NewDividerBlock())
			}
		}
	}

	// Footer context
	footerText := fmt.Sprintf("Last updated: %s", time.Now().Format("Jan 02, 2006 at 3:04 PM"))
	blocks = append(blocks, slack.NewContextBlock(
		"",
		slack.NewTextBlockObject(slack.MarkdownType, footerText, false, false),
	))

	return blocks
}

// formatAlert formats a single alert into a Slack section block.
func (f *SlackAlertFormatter) formatAlert(alert *entity.Alert) *slack.SectionBlock {
	// Severity indicator
	severityMarker := f.getSeverityMarker(alert.Severity)

	// Build alert text
	text := fmt.Sprintf("*%s %s*\n", severityMarker, alert.Name)

	if alert.Summary != "" {
		text += fmt.Sprintf("%s\n", alert.Summary)
	}

	// Alert details
	details := []string{}

	if alert.Instance != "" {
		details = append(details, fmt.Sprintf("Instance: `%s`", alert.Instance))
	}

	if alert.Target != "" {
		details = append(details, fmt.Sprintf("Target: `%s`", alert.Target))
	}

	// Duration since fired
	duration := time.Since(alert.FiredAt)
	details = append(details, fmt.Sprintf("Duration: %s", f.formatDuration(duration)))

	// State
	stateText := string(alert.State)
	if alert.IsAcked() && alert.AckedBy != "" {
		stateText = fmt.Sprintf("Acknowledged by %s", alert.AckedBy)
	}
	details = append(details, fmt.Sprintf("State: %s", stateText))

	text += fmt.Sprintf("_%s_", f.joinDetails(details))

	return slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, text, false, false),
		nil, nil,
	)
}

// getSeverityMarker returns a text marker for the severity level.
func (f *SlackAlertFormatter) getSeverityMarker(severity entity.AlertSeverity) string {
	switch severity {
	case entity.SeverityCritical:
		return "[CRITICAL]"
	case entity.SeverityWarning:
		return "[WARNING]"
	case entity.SeverityInfo:
		return "[INFO]"
	default:
		return "[UNKNOWN]"
	}
}

// formatSeverity capitalizes severity for display.
func (f *SlackAlertFormatter) formatSeverity(severity string) string {
	switch severity {
	case "critical":
		return "Critical"
	case "warning":
		return "Warning"
	case "info":
		return "Info"
	default:
		return severity
	}
}

// formatDuration formats a duration in human-readable form.
func (f *SlackAlertFormatter) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// joinDetails joins detail strings with a separator.
func (f *SlackAlertFormatter) joinDetails(details []string) string {
	result := ""
	for i, detail := range details {
		if i > 0 {
			result += " | "
		}
		result += detail
	}
	return result
}

// FormatAlertSummary formats an AlertSummary into Slack Block Kit blocks.
// The periodDesc parameter describes the time period for the summary (e.g., "last 24 hour(s)").
// Returns blocks ready to be included in a Slack message.
func (f *SlackAlertFormatter) FormatAlertSummary(summary *entity.AlertSummary, periodDesc string) []slack.Block {
	blocks := []slack.Block{}

	// Header block with period
	headerText := "Alert Summary Dashboard"
	if periodDesc != "" && periodDesc != "all time" {
		headerText = fmt.Sprintf("Alert Summary - %s", periodDesc)
	}
	blocks = append(blocks, slack.NewHeaderBlock(
		slack.NewTextBlockObject(slack.PlainTextType, headerText, false, false),
	))

	// Overview section
	overviewText := fmt.Sprintf("*Total Alerts:* %d", summary.TotalAlerts)
	if periodDesc != "" {
		overviewText += fmt.Sprintf(" (%s)", periodDesc)
	}
	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, overviewText, false, false),
		nil, nil,
	))

	blocks = append(blocks, slack.NewDividerBlock())

	// Severity breakdown section
	severityText := "*Alerts by Severity:*\n"
	severityText += fmt.Sprintf("[CRITICAL] %d\n", summary.CriticalCount())
	severityText += fmt.Sprintf("[WARNING] %d\n", summary.WarningCount())
	severityText += fmt.Sprintf("[INFO] %d", summary.InfoCount())

	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, severityText, false, false),
		nil, nil,
	))

	// State breakdown section
	stateText := "*Alerts by State:*\n"
	stateText += fmt.Sprintf("Active: %d\n", summary.ActiveCount())
	stateText += fmt.Sprintf("Acknowledged: %d", summary.AcknowledgedCount())

	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, stateText, false, false),
		nil, nil,
	))

	blocks = append(blocks, slack.NewDividerBlock())

	// Top instance section
	if topInstance := summary.TopInstance(); topInstance != "" {
		instanceText := fmt.Sprintf("*Instance with Most Alerts:*\n`%s` with %d alert(s)",
			topInstance, summary.TopInstanceCount())
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, instanceText, false, false),
			nil, nil,
		))
	}

	// Top instances breakdown (top 5)
	if len(summary.AlertsByInstance) > 0 {
		instanceBreakdown := f.formatTopInstances(summary.AlertsByInstance, 5)
		if instanceBreakdown != "" {
			blocks = append(blocks, slack.NewSectionBlock(
				slack.NewTextBlockObject(slack.MarkdownType, "*Top Instances:*\n"+instanceBreakdown, false, false),
				nil, nil,
			))
		}
	}

	blocks = append(blocks, slack.NewDividerBlock())

	// Top acknowledgers section
	if len(summary.TopAcknowledgers) > 0 {
		ackText := "*Top Acknowledgers:*\n"
		for i, ack := range summary.TopAcknowledgers {
			ackText += fmt.Sprintf("%d. %s: %d acknowledgment(s)\n", i+1, ack.UserName, ack.Count)
		}
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, ackText, false, false),
			nil, nil,
		))
	} else {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*Top Acknowledgers:*\n_No acknowledgments recorded_", false, false),
			nil, nil,
		))
	}

	// Footer context
	footerText := fmt.Sprintf("Summary generated: %s", time.Now().Format("Jan 02, 2006 at 3:04 PM"))
	blocks = append(blocks, slack.NewContextBlock(
		"",
		slack.NewTextBlockObject(slack.MarkdownType, footerText, false, false),
	))

	return blocks
}

// formatTopInstances formats the top N instances by alert count.
func (f *SlackAlertFormatter) formatTopInstances(instances map[string]int, limit int) string {
	// Convert to slice for sorting
	type instanceCount struct {
		name  string
		count int
	}
	sorted := make([]instanceCount, 0, len(instances))
	for name, count := range instances {
		sorted = append(sorted, instanceCount{name: name, count: count})
	}

	// Sort by count descending
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Take top N
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	// Format as string
	result := ""
	for i, ic := range sorted {
		result += fmt.Sprintf("%d. `%s`: %d alert(s)\n", i+1, ic.name, ic.count)
	}
	return result
}

// FormatSilenceResult formats a SilenceResult into Slack Block Kit blocks.
func (f *SlackAlertFormatter) FormatSilenceResult(result *slackUseCase.SilenceResult) []slack.Block {
	blocks := []slack.Block{}

	// Header block
	headerText := "Silence Management"
	blocks = append(blocks, slack.NewHeaderBlock(
		slack.NewTextBlockObject(slack.PlainTextType, headerText, false, false),
	))

	// Message section
	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, result.Message, false, false),
		nil, nil,
	))

	blocks = append(blocks, slack.NewDividerBlock())

	// Content based on action result
	if result.Created != nil {
		blocks = append(blocks, f.formatSilenceDetails(result.Created, "Created"))
	}

	if result.Deleted != nil {
		blocks = append(blocks, f.formatSilenceDetails(result.Deleted, "Deleted"))
	}

	if len(result.Silences) > 0 {
		for i, silence := range result.Silences {
			if i >= 10 {
				blocks = append(blocks, slack.NewSectionBlock(
					slack.NewTextBlockObject(slack.MarkdownType,
						fmt.Sprintf("_Showing 10 of %d silences_", len(result.Silences)),
						false, false),
					nil, nil,
				))
				break
			}
			blocks = append(blocks, f.formatSilenceDetails(silence, ""))
			if i < len(result.Silences)-1 && i < 9 {
				blocks = append(blocks, slack.NewDividerBlock())
			}
		}
	} else if result.Created == nil && result.Deleted == nil {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "_No active silences_", false, false),
			nil, nil,
		))
	}

	// Footer context
	footerText := fmt.Sprintf("Updated: %s", time.Now().Format("Jan 02, 2006 at 3:04 PM"))
	blocks = append(blocks, slack.NewContextBlock(
		"",
		slack.NewTextBlockObject(slack.MarkdownType, footerText, false, false),
	))

	return blocks
}

// formatSilenceDetails formats a single silence into a Slack section block.
func (f *SlackAlertFormatter) formatSilenceDetails(silence *entity.SilenceMark, prefix string) *slack.SectionBlock {
	text := ""
	if prefix != "" {
		text = fmt.Sprintf("*%s Silence*\n", prefix)
	}

	text += fmt.Sprintf("ID: `%s`\n", silence.ID)

	if silence.Reason != "" {
		text += fmt.Sprintf("Reason: %s\n", silence.Reason)
	}

	// Duration info
	remaining := silence.RemainingDuration()
	if remaining > 0 {
		text += fmt.Sprintf("Remaining: %s\n", f.formatDuration(remaining))
	}

	text += fmt.Sprintf("Expires: %s\n", silence.EndAt.Format("Jan 02, 2006 at 3:04 PM"))
	text += fmt.Sprintf("Created by: %s", silence.CreatedBy)

	return slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, text, false, false),
		nil, nil,
	)
}
