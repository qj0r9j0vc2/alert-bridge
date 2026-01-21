package dto

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SilenceAction represents the action to perform on silences.
type SilenceAction string

const (
	SilenceActionCreate    SilenceAction = "create"
	SilenceActionList      SilenceAction = "list"
	SilenceActionDelete    SilenceAction = "delete"
	SilenceActionOpenModal SilenceAction = "open_modal" // Opens the create silence modal
	SilenceActionFromModal SilenceAction = "from_modal" // Created from modal submission
)

// SilenceRequest represents a request to manage silences.
type SilenceRequest struct {
	Action    SilenceAction
	Duration  time.Duration
	Reason    string
	SilenceID string            // For delete action
	Matchers  map[string]string // Label matchers (key=value pairs)
	UserID    string
	UserName  string
	TriggerID string // For opening modals
}

// SlackCommandDTO represents a parsed Slack slash command.
type SlackCommandDTO struct {
	Command     string // The command name (e.g., "/alert-status")
	Text        string // The text after the command
	UserID      string // The user who invoked the command
	UserName    string // The user's display name
	ChannelID   string // The channel where command was invoked
	ChannelName string // The channel's display name
	TeamID      string // The workspace/team ID
	ResponseURL string // URL for delayed responses
	TriggerID   string // Trigger ID for opening modals
}

// ParsedArgs returns the command text as structured arguments.
// For /alert-status, this returns the severity filter.
func (dto *SlackCommandDTO) ParsedArgs() map[string]string {
	args := make(map[string]string)

	if dto.Text != "" {
		// For simple commands like /alert-status critical
		// treat the text as the severity filter
		args["severity"] = dto.Text
	}

	return args
}

// SeverityFilter extracts the severity filter from command text.
// Returns the severity or empty string for all severities.
func (dto *SlackCommandDTO) SeverityFilter() string {
	args := dto.ParsedArgs()
	return args["severity"]
}

// periodRegex matches time period formats like "1h", "24h", "7d", "1w", "30m"
var periodRegex = regexp.MustCompile(`^(\d+)([mhdw])$`)

// PeriodFilter extracts a time period from command text.
// Supported formats: 30m (minutes), 1h/24h (hours), 7d (days), 1w (weeks)
// Returns 0 for no period filter (show all active alerts).
// Default period is 24h if no valid period is specified but text is present.
func (dto *SlackCommandDTO) PeriodFilter() time.Duration {
	text := strings.TrimSpace(strings.ToLower(dto.Text))
	if text == "" {
		return 0 // No filter, show all active alerts
	}

	// Check for common aliases
	switch text {
	case "today":
		return 24 * time.Hour
	case "week", "thisweek":
		return 7 * 24 * time.Hour
	case "all":
		return 0
	}

	// Parse period format (e.g., "1h", "24h", "7d", "1w")
	matches := periodRegex.FindStringSubmatch(text)
	if matches == nil {
		return 0 // Invalid format, show all
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil || value <= 0 {
		return 0
	}

	unit := matches[2]
	switch unit {
	case "m":
		return time.Duration(value) * time.Minute
	case "h":
		return time.Duration(value) * time.Hour
	case "d":
		return time.Duration(value) * 24 * time.Hour
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour
	default:
		return 0
	}
}

// PeriodDescription returns a human-readable description of the period filter.
func (dto *SlackCommandDTO) PeriodDescription() string {
	period := dto.PeriodFilter()
	if period == 0 {
		return "all time"
	}

	hours := int(period.Hours())
	if hours < 1 {
		return period.String()
	}
	if hours < 24 {
		return "last " + strconv.Itoa(hours) + " hour(s)"
	}
	days := hours / 24
	if days < 7 {
		return "last " + strconv.Itoa(days) + " day(s)"
	}
	weeks := days / 7
	return "last " + strconv.Itoa(weeks) + " week(s)"
}

// ParseSilenceRequest parses the command text for /silence command.
// Usage: /silence [create|list|delete] [options]
// Examples:
//   - /silence create               - Opens modal to create a silence
//   - /silence list                 - List all active silences
//   - /silence delete <id>          - Delete a silence by ID
func (d *SlackCommandDTO) ParseSilenceRequest() *SilenceRequest {
	parts := strings.Fields(d.Text)

	req := &SilenceRequest{
		UserID:    d.UserID,
		UserName:  d.UserName,
		TriggerID: d.TriggerID,
		Action:    SilenceActionList, // Default action
		Duration:  1 * time.Hour,     // Default duration
		Matchers:  make(map[string]string),
	}

	if len(parts) == 0 {
		return req
	}

	// Parse action
	action := strings.ToLower(parts[0])
	switch action {
	case "create":
		// Open modal for creating silence with label selection
		req.Action = SilenceActionOpenModal
	case "list":
		req.Action = SilenceActionList
	case "delete":
		req.Action = SilenceActionDelete
		if len(parts) >= 2 {
			req.SilenceID = parts[1]
		}
	default:
		// If first part is a duration, open modal with pre-filled duration
		if dur := parseDuration(action); dur > 0 {
			req.Action = SilenceActionOpenModal
			req.Duration = dur
		}
	}

	return req
}

// parseDuration parses duration strings like "1h", "30m", "7d", "1w"
func parseDuration(s string) time.Duration {
	matches := periodRegex.FindStringSubmatch(strings.ToLower(s))
	if matches == nil {
		return 0
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil || value <= 0 {
		return 0
	}

	switch matches[2] {
	case "m":
		return time.Duration(value) * time.Minute
	case "h":
		return time.Duration(value) * time.Hour
	case "d":
		return time.Duration(value) * 24 * time.Hour
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour
	default:
		return 0
	}
}
