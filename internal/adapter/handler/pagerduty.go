package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/dto"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/alert"
	pdUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/pagerduty"
)

// PagerDutyWebhookHandler handles PagerDuty V3 webhook events.
// NOTE: Signature verification is handled by middleware.PagerDutyAuth middleware.
type PagerDutyWebhookHandler struct {
	handleWebhook *pdUseCase.HandleWebhookUseCase
	logger        alert.Logger
}

// NewPagerDutyWebhookHandler creates a new PagerDuty webhook handler.
func NewPagerDutyWebhookHandler(
	handleWebhook *pdUseCase.HandleWebhookUseCase,
	logger alert.Logger,
) *PagerDutyWebhookHandler {
	return &PagerDutyWebhookHandler{
		handleWebhook: handleWebhook,
		logger:        logger,
	}
}

// ServeHTTP handles POST /webhook/pagerduty
func (h *PagerDutyWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Parse webhook payload
	var payload dto.PagerDutyWebhookV3
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("failed to parse PagerDuty webhook payload", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var processed, skipped int

	// Process each message
	for _, msg := range payload.Messages {
		event := msg.Event

		// Skip unsupported event types
		if !dto.IsSupportedEventType(event.EventType) {
			h.logger.Debug("skipping unsupported event type",
				"eventType", event.EventType,
			)
			skipped++
			continue
		}

		// Build input
		input := dto.HandlePagerDutyWebhookInput{
			EventType:     event.EventType,
			IncidentID:    event.Data.ID,
			IncidentKey:   event.Data.IncidentKey,
			Status:        event.Data.Status,
			ResolveReason: event.Data.ResolveReason,
		}

		// Extract user info from agent or last status change
		if event.Agent != nil {
			input.UserID = event.Agent.ID
			input.UserEmail = event.Agent.Email
			input.UserName = event.Agent.Name
		} else if event.Data.LastStatusChangeBy != nil {
			input.UserID = event.Data.LastStatusChangeBy.ID
			input.UserEmail = event.Data.LastStatusChangeBy.Email
			input.UserName = event.Data.LastStatusChangeBy.Summary
		}

		// For acknowledged events, try to get acknowledger info
		if event.EventType == "incident.acknowledged" && len(event.Data.Acknowledgers) > 0 {
			acker := event.Data.Acknowledgers[len(event.Data.Acknowledgers)-1].Acknowledger
			input.UserID = acker.ID
			input.UserEmail = acker.Email
			input.UserName = acker.Summary
		}

		// Execute use case
		output, err := h.handleWebhook.Execute(ctx, input)
		if err != nil {
			h.logger.Error("failed to handle PagerDuty webhook",
				"eventType", event.EventType,
				"incidentID", event.Data.ID,
				"error", err,
			)
			continue
		}

		if output.Processed {
			processed++
			h.logger.Info("PagerDuty webhook processed",
				"eventType", event.EventType,
				"incidentID", event.Data.ID,
				"alertID", output.AlertID,
				"message", output.Message,
			)
		} else {
			skipped++
		}
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"processed": processed,
		"skipped":   skipped,
	})
}
