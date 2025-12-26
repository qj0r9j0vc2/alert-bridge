package handler

import (
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/slack"
	slackSDK "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// SocketModeHandler wraps the Socket Mode client and routes events to use cases.
type SocketModeHandler struct {
	client *slack.SocketModeClient
	logger slack.Logger
}

// NewSocketModeHandler creates a new Socket Mode handler.
func NewSocketModeHandler(client *slack.SocketModeClient, logger slack.Logger) *SocketModeHandler {
	return &SocketModeHandler{
		client: client,
		logger: logger,
	}
}

// HandleEvent handles Events API events.
func (h *SocketModeHandler) HandleEvent(evt *socketmode.Event) error {
	h.logger.Debug("Handling Events API event", "type", evt.Type)

	eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		h.logger.Error("Failed to cast to EventsAPIEvent")
		return nil
	}

	// Route to specific event handlers based on inner event type
	h.logger.Info("Events API event received", "inner_type", eventsAPI.InnerEvent.Type)

	// TODO: Implement specific event handlers
	// For now, just log the event
	switch eventsAPI.InnerEvent.Type {
	case "app_mention":
		h.logger.Info("App mention event received")
	default:
		h.logger.Debug("Unhandled inner event type", "type", eventsAPI.InnerEvent.Type)
	}

	return nil
}

// HandleCommand handles slash command events.
func (h *SocketModeHandler) HandleCommand(cmd *slackSDK.SlashCommand) error {
	h.logger.Info("Handling slash command",
		"command", cmd.Command,
		"user", cmd.UserID,
		"channel", cmd.ChannelID)

	// TODO: Route to command use cases
	// For now, just log the command
	h.logger.Debug("Slash command details",
		"text", cmd.Text,
		"response_url", cmd.ResponseURL)

	return nil
}

// HandleInteraction handles interactive component events.
func (h *SocketModeHandler) HandleInteraction(callback *slackSDK.InteractionCallback) error {
	h.logger.Info("Handling interaction",
		"type", callback.Type,
		"user", callback.User.ID,
		"channel", callback.Channel.ID)

	// TODO: Route to interaction use cases
	// For now, just log the interaction
	if len(callback.ActionCallback.BlockActions) > 0 {
		action := callback.ActionCallback.BlockActions[0]
		h.logger.Debug("Block action details",
			"action_id", action.ActionID,
			"block_id", action.BlockID,
			"value", action.Value)
	}

	return nil
}
