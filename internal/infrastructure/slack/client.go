package slack

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	domainerrors "github.com/qj0r9j0vc2/alert-bridge/internal/domain/errors"
)

// Client wraps the Slack API client with domain-specific operations.
// Implements the alert.Notifier interface.
type Client struct {
	api            *slack.Client
	channelID      string
	messageBuilder *MessageBuilder
}

// NewClient creates a new Slack client.
func NewClient(botToken, channelID string, silenceDurations []time.Duration, apiURL ...string) *Client {
	var api *slack.Client
	if len(apiURL) > 0 && apiURL[0] != "" {
		// Use custom API URL (for E2E testing)
		api = slack.New(botToken, slack.OptionAPIURL(apiURL[0]))
	} else {
		api = slack.New(botToken)
	}

	return &Client{
		api:            api,
		channelID:      channelID,
		messageBuilder: NewMessageBuilder(silenceDurations),
	}
}

// Notify sends an alert to Slack.
// Returns the message ID in the format "channel:timestamp".
func (c *Client) Notify(ctx context.Context, alert *entity.Alert) (string, error) {
	blocks := c.messageBuilder.BuildAlertMessage(alert)

	options := []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}

	channelID, timestamp, err := c.api.PostMessageContext(ctx, c.channelID, options...)
	if err != nil {
		return "", categorizeSlackError(err, "posting slack message")
	}

	// Return channel:timestamp as message ID
	return fmt.Sprintf("%s:%s", channelID, timestamp), nil
}

// UpdateMessage updates an existing Slack message.
func (c *Client) UpdateMessage(ctx context.Context, messageID string, alert *entity.Alert) error {
	channelID, timestamp, err := parseMessageID(messageID)
	if err != nil {
		return err
	}

	var blocks []slack.Block
	if alert.IsActive() {
		blocks = c.messageBuilder.BuildAlertMessage(alert)
	} else {
		// For acked/resolved alerts, build without action buttons
		blocks = c.messageBuilder.BuildAckedMessage(alert)
	}

	options := []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}

	_, _, _, err = c.api.UpdateMessageContext(ctx, channelID, timestamp, options...)
	if err != nil {
		return categorizeSlackError(err, "updating slack message")
	}

	return nil
}

// Name returns the notifier identifier.
func (c *Client) Name() string {
	return "slack"
}

// PostThreadReply posts a reply in a thread.
func (c *Client) PostThreadReply(ctx context.Context, messageID, text string) error {
	channelID, timestamp, err := parseMessageID(messageID)
	if err != nil {
		return err
	}

	options := []slack.MsgOption{
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(timestamp),
	}

	_, _, err = c.api.PostMessageContext(ctx, channelID, options...)
	if err != nil {
		return categorizeSlackError(err, "posting thread reply")
	}

	return nil
}

// GetUserInfo retrieves user information by ID.
func (c *Client) GetUserInfo(ctx context.Context, userID string) (*slack.User, error) {
	user, err := c.api.GetUserInfoContext(ctx, userID)
	if err != nil {
		return nil, categorizeSlackError(err, "getting user info")
	}
	return user, nil
}

// GetUserEmail retrieves a user's email by their ID.
func (c *Client) GetUserEmail(ctx context.Context, userID string) (string, error) {
	user, err := c.GetUserInfo(ctx, userID)
	if err != nil {
		return "", err
	}
	return user.Profile.Email, nil
}

// AddReaction adds an emoji reaction to a message.
func (c *Client) AddReaction(ctx context.Context, messageID, emoji string) error {
	channelID, timestamp, err := parseMessageID(messageID)
	if err != nil {
		return err
	}

	err = c.api.AddReactionContext(ctx, emoji, slack.ItemRef{
		Channel:   channelID,
		Timestamp: timestamp,
	})
	if err != nil {
		return categorizeSlackError(err, "adding reaction")
	}

	return nil
}

// categorizeSlackError wraps Slack API errors as transient or permanent domain errors.
func categorizeSlackError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Check for network errors (transient)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return domainerrors.NewTransientError(
			fmt.Sprintf("%s: network error", operation),
			err,
		)
	}

	// Check for Slack API errors
	var slackErr slack.SlackErrorResponse
	if errors.As(err, &slackErr) {
		switch slackErr.Err {
		// Rate limiting - transient
		case "rate_limited":
			return domainerrors.NewTransientError(
				fmt.Sprintf("%s: rate limited", operation),
				err,
			)

		// Server errors - transient
		case "internal_error", "fatal_error", "service_unavailable":
			return domainerrors.NewTransientError(
				fmt.Sprintf("%s: slack server error", operation),
				err,
			)

		// Client errors - permanent
		case "invalid_auth", "account_inactive", "token_revoked", "no_permission",
			"channel_not_found", "not_in_channel", "is_archived":
			return domainerrors.NewPermanentError(
				fmt.Sprintf("%s: %s", operation, slackErr.Err),
				err,
			)

		// Default to permanent for unknown Slack errors
		default:
			return domainerrors.NewPermanentError(
				fmt.Sprintf("%s: %s", operation, slackErr.Err),
				err,
			)
		}
	}

	// Check for context errors (transient)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return domainerrors.NewTransientError(
			fmt.Sprintf("%s: context timeout", operation),
			err,
		)
	}

	// Default to permanent error
	return domainerrors.NewPermanentError(
		fmt.Sprintf("%s: %v", operation, err),
		err,
	)
}

// parseMessageID parses a message ID in the format "channel:timestamp".
func parseMessageID(messageID string) (channelID, timestamp string, err error) {
	parts := strings.SplitN(messageID, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid message ID format: %s", messageID)
	}
	return parts[0], parts[1], nil
}
