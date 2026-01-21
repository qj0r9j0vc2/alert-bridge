package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// Severity color codes for visual distinction
const (
	colorCritical = "#E01E5A" // Red
	colorWarning  = "#ECB22E" // Yellow/Orange
	colorInfo     = "#36C5F0" // Blue
	colorResolved = "#2EB67D" // Green
	colorAcked    = "#9B59B6" // Purple
)

// MessageBuilder constructs Slack Block Kit messages for alerts.
type MessageBuilder struct {
	silenceDurations []time.Duration
}

// NewMessageBuilder creates a new message builder with the given silence durations.
func NewMessageBuilder(silenceDurations []time.Duration) *MessageBuilder {
	if len(silenceDurations) == 0 {
		silenceDurations = []time.Duration{
			15 * time.Minute,
			1 * time.Hour,
			4 * time.Hour,
			24 * time.Hour,
		}
	}
	return &MessageBuilder{
		silenceDurations: silenceDurations,
	}
}

// BuildAlertMessage creates a Block Kit message for an alert.
func (b *MessageBuilder) BuildAlertMessage(alert *entity.Alert) []slack.Block {
	return b.buildMessage(alert, true, true)
}

// BuildAckedMessage creates a message for an acknowledged alert with silence button still available.
func (b *MessageBuilder) BuildAckedMessage(alert *entity.Alert) []slack.Block {
	return b.buildMessage(alert, false, true)
}

// BuildResolvedMessage creates a message for a resolved alert (no buttons).
func (b *MessageBuilder) BuildResolvedMessage(alert *entity.Alert) []slack.Block {
	return b.buildMessage(alert, false, false)
}

// buildMessage creates a Block Kit message with configurable button options.
func (b *MessageBuilder) buildMessage(alert *entity.Alert, showAckButton, showSilenceButton bool) []slack.Block {
	var blocks []slack.Block

	// Status banner with emoji and severity indicator
	blocks = append(blocks, b.buildStatusBanner(alert))

	// Alert name as header
	blocks = append(blocks, slack.NewHeaderBlock(
		slack.NewTextBlockObject(slack.PlainTextType, alert.Name, true, false),
	))

	// Summary section (if available)
	if alert.Summary != "" {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("_%s_", alert.Summary), false, false),
			nil, nil,
		))
	}

	// Alert details in a compact format
	blocks = append(blocks, b.buildDetailsSection(alert))

	// Thin divider
	blocks = append(blocks, slack.NewDividerBlock())

	// Timeline context
	blocks = append(blocks, b.buildTimelineContext(alert))

	// Action buttons (configurable)
	if showAckButton || showSilenceButton {
		if actionBlock := b.buildActionButtons(alert.ID, showAckButton, showSilenceButton); actionBlock != nil {
			blocks = append(blocks, actionBlock)
		}
	}

	return blocks
}

// buildStatusBanner creates a visual status banner at the top.
func (b *MessageBuilder) buildStatusBanner(alert *entity.Alert) *slack.SectionBlock {
	emoji, statusText, color := b.getStatusInfo(alert)

	// Create a visually distinct status line
	statusLine := fmt.Sprintf("%s  *%s*  %s", emoji, statusText, b.getSeverityBadge(alert))

	return slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, statusLine, false, false),
		nil,
		slack.NewAccessory(
			slack.NewImageBlockElement(
				b.getStatusIconURL(color),
				statusText,
			),
		),
	)
}

// getStatusInfo returns emoji, text, and color for the alert status.
func (b *MessageBuilder) getStatusInfo(alert *entity.Alert) (emoji, text, color string) {
	switch {
	case alert.IsResolved():
		return "✅", "RESOLVED", colorResolved
	case alert.IsAcked():
		return "👁️", "ACKNOWLEDGED", colorAcked
	case alert.Severity == entity.SeverityCritical:
		return "🚨", "CRITICAL", colorCritical
	case alert.Severity == entity.SeverityWarning:
		return "⚠️", "WARNING", colorWarning
	default:
		return "ℹ️", "INFO", colorInfo
	}
}

// getSeverityBadge returns a formatted severity badge.
func (b *MessageBuilder) getSeverityBadge(alert *entity.Alert) string {
	severity := strings.ToUpper(string(alert.Severity))
	switch alert.Severity {
	case entity.SeverityCritical:
		return fmt.Sprintf("`🔴 %s`", severity)
	case entity.SeverityWarning:
		return fmt.Sprintf("`🟡 %s`", severity)
	default:
		return fmt.Sprintf("`🔵 %s`", severity)
	}
}

// getStatusIconURL returns a placeholder for status-colored icon.
// In production, this could link to actual hosted status icons.
func (b *MessageBuilder) getStatusIconURL(color string) string {
	// Use a simple colored square placeholder
	// You can replace this with actual hosted icons
	return "https://via.placeholder.com/48/" + strings.TrimPrefix(color, "#") + "/FFFFFF?text=+"
}

// buildDetailsSection creates the details section with fields in a clean layout.
func (b *MessageBuilder) buildDetailsSection(alert *entity.Alert) *slack.SectionBlock {
	var fields []*slack.TextBlockObject

	// Instance
	if alert.Instance != "" {
		fields = append(fields,
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("*🖥️ Instance*\n`%s`", alert.Instance), false, false))
	}

	// Target
	if alert.Target != "" {
		fields = append(fields,
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("*🎯 Target*\n`%s`", alert.Target), false, false))
	}

	// State
	fields = append(fields,
		slack.NewTextBlockObject(slack.MarkdownType,
			fmt.Sprintf("*📊 State*\n%s", b.formatState(alert.State)), false, false))

	// Fingerprint (shortened for display)
	if alert.Fingerprint != "" {
		fp := alert.Fingerprint
		if len(fp) > 12 {
			fp = fp[:12] + "..."
		}
		fields = append(fields,
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("*🔑 ID*\n`%s`", fp), false, false))
	}

	return slack.NewSectionBlock(nil, fields, nil)
}

// buildTimelineContext creates the timeline context with fired/acked/resolved times.
func (b *MessageBuilder) buildTimelineContext(alert *entity.Alert) *slack.ContextBlock {
	var elements []slack.MixedElement

	// Fired time
	firedAt := alert.FiredAt.Format("Jan 2, 15:04 MST")
	elements = append(elements,
		slack.NewTextBlockObject(slack.MarkdownType,
			fmt.Sprintf("🔥 Fired: *%s*", firedAt), false, false))

	// Acknowledged info
	if alert.IsAcked() && alert.AckedBy != "" {
		ackedAt := "unknown"
		if alert.AckedAt != nil {
			ackedAt = alert.AckedAt.Format("15:04 MST")
		}
		elements = append(elements,
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("  •  👁️ Acked by *%s* at %s", alert.AckedBy, ackedAt), false, false))
	}

	// Resolved info
	if alert.IsResolved() && alert.ResolvedAt != nil {
		resolvedAt := alert.ResolvedAt.Format("15:04 MST")
		elements = append(elements,
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("  •  ✅ Resolved: *%s*", resolvedAt), false, false))
	}

	return slack.NewContextBlock("", elements...)
}

// buildActionButtons creates the interactive action buttons.
func (b *MessageBuilder) buildActionButtons(alertID string, showAck, showSilence bool) *slack.ActionBlock {
	var elements []slack.BlockElement

	// Acknowledge button
	if showAck {
		ackBtn := slack.NewButtonBlockElement(
			fmt.Sprintf("ack_%s", alertID),
			alertID,
			slack.NewTextBlockObject(slack.PlainTextType, "✓ Acknowledge", true, false),
		)
		ackBtn.Style = slack.StylePrimary
		elements = append(elements, ackBtn)
	}

	// Silence duration dropdown
	if showSilence {
		options := make([]*slack.OptionBlockObject, len(b.silenceDurations))
		for i, d := range b.silenceDurations {
			options[i] = slack.NewOptionBlockObject(
				d.String(),
				slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("🔕 %s", b.formatDuration(d)), false, false),
				nil,
			)
		}

		silenceSelect := slack.NewOptionsSelectBlockElement(
			slack.OptTypeStatic,
			slack.NewTextBlockObject(slack.PlainTextType, "🔕 Silence...", false, false),
			fmt.Sprintf("silence_%s", alertID),
			options...,
		)
		elements = append(elements, silenceSelect)
	}

	if len(elements) == 0 {
		return nil
	}

	return slack.NewActionBlock(fmt.Sprintf("actions_%s", alertID), elements...)
}

// formatDuration formats a duration for display.
func (b *MessageBuilder) formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// formatState formats the alert state for display.
func (b *MessageBuilder) formatState(state entity.AlertState) string {
	switch state {
	case entity.StateActive:
		return "🔴 Firing"
	case entity.StateAcked:
		return "👁️ Acknowledged"
	case entity.StateResolved:
		return "✅ Resolved"
	default:
		return string(state)
	}
}
