package dto

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
