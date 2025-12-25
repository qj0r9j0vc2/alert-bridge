package pagerduty

import (
	"context"
	"fmt"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/dto"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/ack"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/alert"
)

// HandleWebhookUseCase processes PagerDuty webhook events.
type HandleWebhookUseCase struct {
	alertRepo    repository.AlertRepository
	syncAckUC    *ack.SyncAckUseCase
	slackUpdater MessageUpdater
	logger       alert.Logger
}

// MessageUpdater defines the interface for updating messages.
type MessageUpdater interface {
	UpdateMessage(ctx context.Context, messageID string, alert *entity.Alert) error
}

// NewHandleWebhookUseCase creates a new HandleWebhookUseCase.
func NewHandleWebhookUseCase(
	alertRepo repository.AlertRepository,
	syncAckUC *ack.SyncAckUseCase,
	slackUpdater MessageUpdater,
	logger alert.Logger,
) *HandleWebhookUseCase {
	return &HandleWebhookUseCase{
		alertRepo:    alertRepo,
		syncAckUC:    syncAckUC,
		slackUpdater: slackUpdater,
		logger:       logger,
	}
}

// Execute processes a PagerDuty webhook event.
func (uc *HandleWebhookUseCase) Execute(ctx context.Context, input dto.HandlePagerDutyWebhookInput) (*dto.HandlePagerDutyWebhookOutput, error) {
	output := &dto.HandlePagerDutyWebhookOutput{}

	// Find the alert by incident key (which maps to our fingerprint)
	alertEntity, err := uc.findAlertByIncidentKey(ctx, input.IncidentKey, input.IncidentID)
	if err != nil {
		return nil, fmt.Errorf("finding alert: %w", err)
	}
	if alertEntity == nil {
		uc.logger.Debug("no alert found for PagerDuty incident",
			"incidentID", input.IncidentID,
			"incidentKey", input.IncidentKey,
		)
		output.Message = "no matching alert found"
		return output, nil
	}

	output.AlertID = alertEntity.ID

	// Handle based on event type
	switch input.EventType {
	case "incident.acknowledged":
		return uc.handleAcknowledged(ctx, alertEntity, input, output)

	case "incident.resolved":
		return uc.handleResolved(ctx, alertEntity, input, output)

	case "incident.unacknowledged":
		// PagerDuty unacknowledged - we don't sync this back to Slack
		// as it's typically a timeout, not a user action
		uc.logger.Info("incident unacknowledged (not syncing)",
			"alertID", alertEntity.ID,
			"incidentID", input.IncidentID,
		)
		output.Processed = true
		output.Message = "unacknowledged event noted"
		return output, nil

	case "incident.escalated":
		return uc.handleEscalated(ctx, alertEntity, input, output)

	case "incident.priority_updated":
		return uc.handlePriorityUpdated(ctx, alertEntity, input, output)

	case "incident.responder_added":
		return uc.handleResponderAdded(ctx, alertEntity, input, output)

	case "incident.status_update_published":
		return uc.handleStatusUpdatePublished(ctx, alertEntity, input, output)

	default:
		uc.logger.Debug("ignoring PagerDuty event type",
			"eventType", input.EventType,
		)
		output.Message = "event type not handled"
		return output, nil
	}
}

// handleAcknowledged processes an incident.acknowledged event.
func (uc *HandleWebhookUseCase) handleAcknowledged(
	ctx context.Context,
	alertEntity *entity.Alert,
	input dto.HandlePagerDutyWebhookInput,
	output *dto.HandlePagerDutyWebhookOutput,
) (*dto.HandlePagerDutyWebhookOutput, error) {
	// Skip if already acked
	if alertEntity.IsAcked() || alertEntity.IsResolved() {
		uc.logger.Debug("alert already acked/resolved, skipping PagerDuty ack sync",
			"alertID", alertEntity.ID,
			"state", alertEntity.State,
		)
		output.Processed = true
		output.Message = "already acknowledged"
		return output, nil
	}

	// Execute sync ack use case (this will update Slack)
	syncInput := ack.SyncAckInput{
		AlertID:   alertEntity.ID,
		Source:    entity.AckSourcePagerDuty,
		UserID:    input.UserID,
		UserEmail: input.UserEmail,
		UserName:  input.UserName,
	}

	ackOutput, err := uc.syncAckUC.Execute(ctx, syncInput)
	if err != nil {
		return nil, fmt.Errorf("syncing ack: %w", err)
	}

	// Update Slack message if we have a message ID
	if alertEntity.SlackMessageID != "" && uc.slackUpdater != nil {
		if err := uc.slackUpdater.UpdateMessage(ctx, alertEntity.SlackMessageID, ackOutput.Alert); err != nil {
			uc.logger.Error("failed to update Slack message",
				"alertID", alertEntity.ID,
				"slackMessageID", alertEntity.SlackMessageID,
				"error", err,
			)
		} else {
			uc.logger.Info("updated Slack message for PagerDuty ack",
				"alertID", alertEntity.ID,
				"slackMessageID", alertEntity.SlackMessageID,
			)
		}
	}

	output.Processed = true
	output.Message = fmt.Sprintf("acknowledged by %s", input.UserEmail)
	return output, nil
}

// handleResolved processes an incident.resolved event.
func (uc *HandleWebhookUseCase) handleResolved(
	ctx context.Context,
	alertEntity *entity.Alert,
	input dto.HandlePagerDutyWebhookInput,
	output *dto.HandlePagerDutyWebhookOutput,
) (*dto.HandlePagerDutyWebhookOutput, error) {
	// Skip if already resolved
	if alertEntity.IsResolved() {
		output.Processed = true
		output.Message = "already resolved"
		return output, nil
	}

	// Resolve the alert
	alertEntity.Resolve(time.Now().UTC())
	if err := uc.alertRepo.Update(ctx, alertEntity); err != nil {
		return nil, fmt.Errorf("updating alert: %w", err)
	}

	// Update Slack message if we have a message ID
	if alertEntity.SlackMessageID != "" && uc.slackUpdater != nil {
		if err := uc.slackUpdater.UpdateMessage(ctx, alertEntity.SlackMessageID, alertEntity); err != nil {
			uc.logger.Error("failed to update Slack message for resolution",
				"alertID", alertEntity.ID,
				"slackMessageID", alertEntity.SlackMessageID,
				"error", err,
			)
		} else {
			uc.logger.Info("updated Slack message for PagerDuty resolution",
				"alertID", alertEntity.ID,
				"slackMessageID", alertEntity.SlackMessageID,
			)
		}
	}

	output.Processed = true
	output.Message = "resolved"
	return output, nil
}

// handleEscalated processes an incident.escalated event.
func (uc *HandleWebhookUseCase) handleEscalated(
	ctx context.Context,
	alertEntity *entity.Alert,
	input dto.HandlePagerDutyWebhookInput,
	output *dto.HandlePagerDutyWebhookOutput,
) (*dto.HandlePagerDutyWebhookOutput, error) {
	uc.logger.Info("incident escalated",
		"alertID", alertEntity.ID,
		"incidentID", input.IncidentID,
		"alertName", alertEntity.Name,
		"user", input.UserEmail,
	)

	output.Processed = true
	output.Message = "escalation noted"
	return output, nil
}

// handlePriorityUpdated processes an incident.priority_updated event.
func (uc *HandleWebhookUseCase) handlePriorityUpdated(
	ctx context.Context,
	alertEntity *entity.Alert,
	input dto.HandlePagerDutyWebhookInput,
	output *dto.HandlePagerDutyWebhookOutput,
) (*dto.HandlePagerDutyWebhookOutput, error) {
	uc.logger.Info("incident priority updated",
		"alertID", alertEntity.ID,
		"incidentID", input.IncidentID,
		"alertName", alertEntity.Name,
		"currentSeverity", alertEntity.Severity,
	)

	// Note: Priority update could potentially map to severity change,
	// but this requires additional webhook data (old/new priority).
	// For now, we just log the event.

	output.Processed = true
	output.Message = "priority update noted"
	return output, nil
}

// handleResponderAdded processes an incident.responder_added event.
func (uc *HandleWebhookUseCase) handleResponderAdded(
	ctx context.Context,
	alertEntity *entity.Alert,
	input dto.HandlePagerDutyWebhookInput,
	output *dto.HandlePagerDutyWebhookOutput,
) (*dto.HandlePagerDutyWebhookOutput, error) {
	uc.logger.Info("responder added to incident",
		"alertID", alertEntity.ID,
		"incidentID", input.IncidentID,
		"alertName", alertEntity.Name,
		"user", input.UserEmail,
	)

	output.Processed = true
	output.Message = "responder addition noted"
	return output, nil
}

// handleStatusUpdatePublished processes an incident.status_update_published event.
func (uc *HandleWebhookUseCase) handleStatusUpdatePublished(
	ctx context.Context,
	alertEntity *entity.Alert,
	input dto.HandlePagerDutyWebhookInput,
	output *dto.HandlePagerDutyWebhookOutput,
) (*dto.HandlePagerDutyWebhookOutput, error) {
	uc.logger.Info("status update published",
		"alertID", alertEntity.ID,
		"incidentID", input.IncidentID,
		"alertName", alertEntity.Name,
		"user", input.UserEmail,
	)

	output.Processed = true
	output.Message = "status update noted"
	return output, nil
}

// findAlertByIncidentKey finds an alert by PagerDuty incident key.
// The incident key typically maps to our fingerprint.
func (uc *HandleWebhookUseCase) findAlertByIncidentKey(ctx context.Context, incidentKey, incidentID string) (*entity.Alert, error) {
	// First, try to find by PagerDuty incident ID
	alertEntity, err := uc.alertRepo.FindByPagerDutyIncidentID(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	if alertEntity != nil {
		return alertEntity, nil
	}

	// Try to find by fingerprint (incident key)
	if incidentKey != "" {
		alerts, err := uc.alertRepo.FindByFingerprint(ctx, incidentKey)
		if err != nil {
			return nil, err
		}
		// Return the most recent firing alert
		for _, a := range alerts {
			if a.IsFiring() {
				return a, nil
			}
		}
		// Return any alert if no firing one found
		if len(alerts) > 0 {
			return alerts[0], nil
		}
	}

	return nil, nil
}
