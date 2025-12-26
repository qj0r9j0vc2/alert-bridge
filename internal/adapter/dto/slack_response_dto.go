package dto

import "github.com/slack-go/slack"

// SlackResponseDTO represents a Slack message response.
type SlackResponseDTO struct {
	ResponseType string             `json:"response_type"`         // "ephemeral" or "in_channel"
	Text         string             `json:"text"`                  // Plain text fallback
	Blocks       []slack.Block      `json:"blocks,omitempty"`      // Block Kit blocks
	Attachments  []slack.Attachment `json:"attachments,omitempty"` // Legacy attachments
}

// NewEphemeralResponse creates an ephemeral response (visible only to command invoker).
func NewEphemeralResponse(text string) *SlackResponseDTO {
	return &SlackResponseDTO{
		ResponseType: "ephemeral",
		Text:         text,
	}
}

// NewInChannelResponse creates an in-channel response (visible to everyone).
func NewInChannelResponse(text string) *SlackResponseDTO {
	return &SlackResponseDTO{
		ResponseType: "in_channel",
		Text:         text,
	}
}

// NewEphemeralWithBlocks creates an ephemeral response with Block Kit blocks.
func NewEphemeralWithBlocks(text string, blocks []slack.Block) *SlackResponseDTO {
	return &SlackResponseDTO{
		ResponseType: "ephemeral",
		Text:         text,
		Blocks:       blocks,
	}
}
