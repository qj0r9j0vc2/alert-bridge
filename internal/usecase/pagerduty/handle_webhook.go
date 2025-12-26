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
	slackMessageID := alertEntity.GetExternalReference("slack")
	if slackMessageID != "" && uc.slackUpdater != nil {
		if err := uc.slackUpdater.UpdateMessage(ctx, slackMessageID, ackOutput.Alert); err != nil {
			uc.logger.Error("failed to update Slack message",
				"alertID", alertEntity.ID,
				"slackMessageID", slackMessageID,
				"error", err,
			)
		} else {
			uc.logger.Info("updated Slack message for PagerDuty ack",
				"alertID", alertEntity.ID,
				"slackMessageID", slackMessageID,
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
	slackMessageID := alertEntity.GetExternalReference("slack")
	if slackMessageID != "" && uc.slackUpdater != nil {
		if err := uc.slackUpdater.UpdateMessage(ctx, slackMessageID, alertEntity); err != nil {
			uc.logger.Error("failed to update Slack message for resolution",
				"alertID", alertEntity.ID,
				"slackMessageID", slackMessageID,
				"error", err,
			)
		} else {
			uc.logger.Info("updated Slack message for PagerDuty resolution",
				"alertID", alertEntity.ID,
				"slackMessageID", slackMessageID,
			)
		}
	}

	output.Processed = true
	output.Message = "resolved"
	return output, nil
}

// findAlertByIncidentKey finds an alert by PagerDuty incident key.
// The incident key typically maps to our fingerprint.
// Uses two-tier lookup strategy: primary by incident ID, fallback to fingerprint.
func (uc *HandleWebhookUseCase) findAlertByIncidentKey(ctx context.Context, incidentKey, incidentID string) (*entity.Alert, error) {
	// First, try to find by PagerDuty incident ID (primary lookup)
	if incidentID != "" {
		start := time.Now()
		alertEntity, err := uc.alertRepo.FindByExternalReference(ctx, "pagerduty", incidentID)
		duration := time.Since(start)

		// Log query performance
		if duration > 10*time.Millisecond {
			uc.logger.Warn("slow database query",
				"query", "FindByExternalReference",
				"system", "pagerduty",
				"duration_ms", duration.Milliseconds(),
			)
		}

		if err != nil {
			uc.logger.Error("error finding alert by incident ID",
				"incidentID", incidentID,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
			return nil, err
		}

		if alertEntity != nil {
			uc.logger.Debug("alert found by incident ID",
				"alertID", alertEntity.ID,
				"incidentID", incidentID,
				"lookup_method", "incident_id",
				"duration_ms", duration.Milliseconds(),
			)
			return alertEntity, nil
		}

		uc.logger.Debug("no alert found by incident ID, trying fingerprint fallback",
			"incidentID", incidentID,
			"duration_ms", duration.Milliseconds(),
		)
	}

	// Fallback: Try to find by fingerprint (incident key)
	if incidentKey != "" {
		start := time.Now()
		alerts, err := uc.alertRepo.FindByFingerprint(ctx, incidentKey)
		duration := time.Since(start)

		// Log query performance
		if duration > 10*time.Millisecond {
			uc.logger.Warn("slow database query",
				"query", "FindByFingerprint",
				"fingerprint", incidentKey,
				"duration_ms", duration.Milliseconds(),
			)
		}

		if err != nil {
			uc.logger.Error("error finding alert by fingerprint",
				"fingerprint", incidentKey,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
			return nil, err
		}

		// Return the most recent firing alert (preferred)
		for _, a := range alerts {
			if a.IsFiring() {
				uc.logger.Debug("alert found by fingerprint",
					"alertID", a.ID,
					"fingerprint", incidentKey,
					"lookup_method", "fingerprint",
					"alert_count", len(alerts),
					"preferred", "firing",
					"duration_ms", duration.Milliseconds(),
				)
				return a, nil
			}
		}

		// Return any alert if no firing one found
		if len(alerts) > 0 {
			uc.logger.Debug("alert found by fingerprint (no firing alert)",
				"alertID", alerts[0].ID,
				"fingerprint", incidentKey,
				"lookup_method", "fingerprint",
				"alert_count", len(alerts),
				"duration_ms", duration.Milliseconds(),
			)
			return alerts[0], nil
		}

		uc.logger.Debug("no alert found by fingerprint",
			"fingerprint", incidentKey,
			"duration_ms", duration.Milliseconds(),
		)
	}

	return nil, nil
}
