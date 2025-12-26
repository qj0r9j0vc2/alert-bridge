package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/dto"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/ack"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/alert"
)

// HandleInteractionUseCase processes Slack button clicks and other interactions.
type HandleInteractionUseCase struct {
	alertRepo   repository.AlertRepository
	silenceRepo repository.SilenceRepository
	syncAckUC   *ack.SyncAckUseCase
	slackClient SlackClient
	logger      alert.Logger
}

// SlackClient defines the required Slack client operations.
type SlackClient interface {
	GetUserEmail(ctx context.Context, userID string) (string, error)
	UpdateMessage(ctx context.Context, messageID string, alert *entity.Alert) error
	PostThreadReply(ctx context.Context, messageID, text string) error
}

// NewHandleInteractionUseCase creates a new HandleInteractionUseCase.
func NewHandleInteractionUseCase(
	alertRepo repository.AlertRepository,
	silenceRepo repository.SilenceRepository,
	syncAckUC *ack.SyncAckUseCase,
	slackClient SlackClient,
	logger alert.Logger,
) *HandleInteractionUseCase {
	return &HandleInteractionUseCase{
		alertRepo:   alertRepo,
		silenceRepo: silenceRepo,
		syncAckUC:   syncAckUC,
		slackClient: slackClient,
		logger:      logger,
	}
}

// Execute processes a Slack interaction.
func (uc *HandleInteractionUseCase) Execute(ctx context.Context, input dto.SlackInteractionInput) (*dto.SlackInteractionOutput, error) {
	// Parse action type from action ID
	actionType, alertID := parseActionID(input.ActionID)

	// Get user email
	userEmail := input.UserEmail
	if userEmail == "" {
		var err error
		userEmail, err = uc.slackClient.GetUserEmail(ctx, input.UserID)
		if err != nil {
			uc.logger.Warn("failed to get user email",
				"userID", input.UserID,
				"error", err,
			)
			userEmail = input.UserID // Fallback to user ID
		}
	}

	switch actionType {
	case "ack":
		return uc.handleAck(ctx, alertID, input, userEmail)
	case "note":
		return uc.handleNote(ctx, alertID, input, userEmail)
	case "silence":
		return uc.handleSilence(ctx, alertID, input, userEmail)
	default:
		return nil, fmt.Errorf("unknown action type: %s", actionType)
	}
}

// handleAck handles the acknowledge action.
func (uc *HandleInteractionUseCase) handleAck(ctx context.Context, alertID string, input dto.SlackInteractionInput, userEmail string) (*dto.SlackInteractionOutput, error) {
	// Execute sync ack use case
	syncInput := ack.SyncAckInput{
		AlertID:   alertID,
		Source:    entity.AckSourceSlack,
		UserID:    input.UserID,
		UserEmail: userEmail,
		UserName:  input.UserName,
	}

	output, err := uc.syncAckUC.Execute(ctx, syncInput)
	if err != nil {
		return nil, fmt.Errorf("syncing ack: %w", err)
	}

	// Update Slack message to show acknowledged state
	messageID := fmt.Sprintf("%s:%s", input.ChannelID, input.MessageTS)
	if err := uc.slackClient.UpdateMessage(ctx, messageID, output.Alert); err != nil {
		uc.logger.Error("failed to update Slack message",
			"messageID", messageID,
			"error", err,
		)
	}

	return &dto.SlackInteractionOutput{
		Success: true,
		Message: fmt.Sprintf("Alert acknowledged by %s", input.UserName),
	}, nil
}

// handleNote handles the add note action.
func (uc *HandleInteractionUseCase) handleNote(ctx context.Context, alertID string, input dto.SlackInteractionInput, userEmail string) (*dto.SlackInteractionOutput, error) {
	// For now, just post a thread reply indicating note functionality
	// In a full implementation, this would open a modal for note input
	messageID := fmt.Sprintf("%s:%s", input.ChannelID, input.MessageTS)

	note := fmt.Sprintf("📝 Note requested by %s", input.UserName)
	if err := uc.slackClient.PostThreadReply(ctx, messageID, note); err != nil {
		uc.logger.Error("failed to post thread reply",
			"messageID", messageID,
			"error", err,
		)
	}

	return &dto.SlackInteractionOutput{
		Success: true,
		Message: "Note added",
	}, nil
}

// handleSilence handles the silence action.
func (uc *HandleInteractionUseCase) handleSilence(ctx context.Context, alertID string, input dto.SlackInteractionInput, userEmail string) (*dto.SlackInteractionOutput, error) {
	// Parse duration from value
	duration, err := time.ParseDuration(input.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid silence duration: %w", err)
	}

	// Load the alert
	alertEntity, err := uc.alertRepo.FindByID(ctx, alertID)
	if err != nil {
		return nil, fmt.Errorf("finding alert: %w", err)
	}
	if alertEntity == nil {
		return nil, entity.ErrAlertNotFound
	}

	// Create silence
	silence, err := entity.NewSilenceMark(duration, input.UserName, userEmail, entity.AckSourceSlack)
	if err != nil {
		return nil, fmt.Errorf("creating silence: %w", err)
	}

	// Set silence target (fingerprint-based for similar alerts)
	silence.ForFingerprint(alertEntity.Fingerprint)
	silence.WithReason(fmt.Sprintf("Silenced from Slack by %s", input.UserName))

	// Save silence
	if err := uc.silenceRepo.Save(ctx, silence); err != nil {
		return nil, fmt.Errorf("saving silence: %w", err)
	}

	// Also acknowledge the alert
	syncInput := ack.SyncAckInput{
		AlertID:   alertID,
		Source:    entity.AckSourceSlack,
		UserID:    input.UserID,
		UserEmail: userEmail,
		UserName:  input.UserName,
		Duration:  &duration,
	}

	ackOutput, err := uc.syncAckUC.Execute(ctx, syncInput)
	if err != nil {
		uc.logger.Warn("failed to sync ack with silence",
			"alertID", alertID,
			"error", err,
		)
	}

	// Update Slack message
	messageID := fmt.Sprintf("%s:%s", input.ChannelID, input.MessageTS)
	if ackOutput != nil && ackOutput.Alert != nil {
		if err := uc.slackClient.UpdateMessage(ctx, messageID, ackOutput.Alert); err != nil {
			uc.logger.Error("failed to update Slack message",
				"messageID", messageID,
				"error", err,
			)
		}
	}

	// Post thread reply about silence
	silenceMsg := fmt.Sprintf("🔕 Silenced for %s by %s (until %s)",
		formatDuration(duration),
		input.UserName,
		silence.EndAt.Format("Jan 2, 15:04 MST"),
	)
	if err := uc.slackClient.PostThreadReply(ctx, messageID, silenceMsg); err != nil {
		uc.logger.Error("failed to post silence notification",
			"messageID", messageID,
			"error", err,
		)
	}

	return &dto.SlackInteractionOutput{
		Success:      true,
		Message:      fmt.Sprintf("Silenced for %s", formatDuration(duration)),
		SilenceID:    silence.ID,
		SilenceEndAt: &silence.EndAt,
	}, nil
}

// parseActionID parses an action ID like "ack_<alertID>" into action type and alert ID.
func parseActionID(actionID string) (actionType, alertID string) {
	parts := strings.SplitN(actionID, "_", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return actionID, ""
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
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
