package entity

import "time"

// SlackEvent represents an event from Slack's Events API.
type SlackEvent struct {
	// Event type (e.g., "app_mention", "message")
	Type string

	// Context
	TeamID    string
	UserID    string
	ChannelID string

	// Message content (for text-based events)
	Text      string
	Timestamp string // Message timestamp for threading

	// Thread context
	ThreadTS string // Thread timestamp (if in thread)

	// Metadata
	EventID   string    // Unique event identifier (for deduplication)
	EventTime time.Time // When event occurred
}

// IsAppMention returns true if this is an app_mention event.
func (e *SlackEvent) IsAppMention() bool {
	return e.Type == "app_mention"
}

// IsInThread returns true if the event is part of a thread.
func (e *SlackEvent) IsInThread() bool {
	return e.ThreadTS != ""
}
