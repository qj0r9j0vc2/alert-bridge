package presenter

import (
	"fmt"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
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
