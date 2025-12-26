package entity

import (
	"strings"
	"time"
)

// SlackCommand represents a slash command invocation from Slack.
type SlackCommand struct {
	// Command metadata
	CommandText string // e.g., "/alert-status"
	Args        string // e.g., "critical" or "severity=warning"

	// User context
	UserID   string // Slack user ID (U123ABC)
	UserName string // Slack username for display

	// Channel context
	ChannelID   string // Channel where command was invoked (C456DEF)
	ChannelName string // Channel name for display

	// Team context
	TeamID     string // Slack workspace ID (T789GHI)
	TeamDomain string // Workspace domain

	// Response mechanism
	ResponseURL string    // URL for delayed responses (valid 30 minutes)
	TriggerID   string    // ID for opening modals/dialogs
	InvokedAt   time.Time // When command was invoked
}

// ParsedArgs returns structured arguments from Args string.
// Supports both single values and key=value pairs.
// Examples:
//   - "critical" -> {"severity": "critical"}
//   - "severity=warning limit=50" -> {"severity": "warning", "limit": "50"}
func (c *SlackCommand) ParsedArgs() map[string]string {
	args := make(map[string]string)
	if c.Args == "" {
		return args
	}

	// Split by whitespace
	parts := strings.Fields(c.Args)

	for _, part := range parts {
		// Check if it's a key=value pair
		if idx := strings.Index(part, "="); idx > 0 {
			key := part[:idx]
			value := part[idx+1:]
			args[key] = value
		} else {
			// Single value: default key "severity" for backward compatibility
			args["severity"] = part
		}
	}

	return args
}

// SeverityFilter extracts severity filter from arguments.
// Returns "critical", "warning", "info", or "" (all severities).
func (c *SlackCommand) SeverityFilter() string {
	args := c.ParsedArgs()
	if severity, ok := args["severity"]; ok {
		severity = strings.ToLower(severity)
		// Validate against known severities
		switch severity {
		case "critical", "warning", "info":
			return severity
		}
	}
	return "" // Return empty string for "all severities"
}
