package entity

import "time"

// InteractiveComponent represents a Slack interactive component action (buttons, menus).
type InteractiveComponent struct {
	// Action type (e.g., "button", "static_select", "overflow")
	Type string

	// Action identifier (from Block Kit action_id)
	ActionID string

	// Block identifier (from Block Kit block_id)
	BlockID string

	// Action value (payload specific to action type)
	// For buttons: the value attribute
	// For menus: the selected option value
	Value string

	// User context
	UserID   string
	UserName string

	// Channel context
	ChannelID string

	// Team context
	TeamID string

	// Response mechanism
	ResponseURL string    // URL for updating message (valid 30 minutes)
	TriggerID   string    // ID for opening modals
	ActionedAt  time.Time // When action was performed
}

// IsButton returns true if this is a button click action.
func (c *InteractiveComponent) IsButton() bool {
	return c.Type == "button"
}

// IsSelect returns true if this is a select menu action.
func (c *InteractiveComponent) IsSelect() bool {
	return c.Type == "static_select" || c.Type == "multi_static_select"
}

// IsOverflow returns true if this is an overflow menu action.
func (c *InteractiveComponent) IsOverflow() bool {
	return c.Type == "overflow"
}
