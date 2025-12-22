package handler

import (
	"encoding/json"
	"net/http"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/dto"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/alert"
)

// AlertmanagerHandler handles Alertmanager webhook requests.
type AlertmanagerHandler struct {
	processAlert *alert.ProcessAlertUseCase
	logger       alert.Logger
}

// NewAlertmanagerHandler creates a new handler.
func NewAlertmanagerHandler(processAlert *alert.ProcessAlertUseCase, logger alert.Logger) *AlertmanagerHandler {
	return &AlertmanagerHandler{
		processAlert: processAlert,
		logger:       logger,
	}
}

// ServeHTTP handles POST /webhook/alertmanager
func (h *AlertmanagerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload dto.AlertmanagerWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Error("failed to decode alertmanager payload",
			"error", err,
		)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var processed, failed int

	// Process each alert in the payload
	for _, alertData := range payload.Alerts {
		input := dto.ToProcessAlertInput(alertData)

		output, err := h.processAlert.Execute(ctx, input)
		if err != nil {
			h.logger.Error("failed to process alert",
				"fingerprint", alertData.Fingerprint,
				"status", alertData.Status,
				"error", err,
			)
			failed++
			continue
		}

		processed++
		h.logger.Info("alert processed",
			"alertID", output.AlertID,
			"fingerprint", alertData.Fingerprint,
			"status", alertData.Status,
			"isNew", output.IsNew,
			"isSilenced", output.IsSilenced,
			"notificationsSent", output.NotificationsSent,
		)
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"processed": processed,
		"failed":    failed,
	})
}
