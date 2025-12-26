package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/slack-go/slack"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/dto"
	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/presenter"
	slackUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/slack"
)

// SlackCommandsHandler handles Slack slash command webhooks (HTTP Mode).
type SlackCommandsHandler struct {
	queryAlertStatus *slackUseCase.QueryAlertStatusUseCase
	formatter        *presenter.SlackAlertFormatter
	logger           *slog.Logger
}

// NewSlackCommandsHandler creates a new slash commands handler.
func NewSlackCommandsHandler(
	queryAlertStatus *slackUseCase.QueryAlertStatusUseCase,
	logger *slog.Logger,
) *SlackCommandsHandler {
	return &SlackCommandsHandler{
		queryAlertStatus: queryAlertStatus,
		formatter:        presenter.NewSlackAlertFormatter(),
		logger:           logger,
	}
}

// ServeHTTP implements http.Handler interface.
func (h *SlackCommandsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.HandleSlashCommand(w, r)
}

// HandleSlashCommand handles POST /webhook/slack/commands requests.
// Slack sends slash commands as application/x-www-form-urlencoded.
func (h *SlackCommandsHandler) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Parse slash command from request
	cmd, err := slack.SlashCommandParse(r)
	if err != nil {
		h.logger.Error("failed to parse slash command", "error", err.Error())
		http.Error(w, "Invalid slash command", http.StatusBadRequest)
		return
	}

	// Convert to DTO
	cmdDTO := &dto.SlackCommandDTO{
		Command:     cmd.Command,
		Text:        cmd.Text,
		UserID:      cmd.UserID,
		UserName:    cmd.UserName,
		ChannelID:   cmd.ChannelID,
		ChannelName: cmd.ChannelName,
		TeamID:      cmd.TeamID,
		ResponseURL: cmd.ResponseURL,
		TriggerID:   cmd.TriggerID,
	}

	h.logger.Info("received slash command",
		"command", cmdDTO.Command,
		"user_id", cmdDTO.UserID,
		"channel_id", cmdDTO.ChannelID,
		"text", cmdDTO.Text)

	// Send immediate acknowledgment (Slack requires response within 3 seconds)
	immediateResponse := dto.NewEphemeralResponse("Fetching alert status...")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(immediateResponse); err != nil {
		h.logger.Error("failed to encode immediate response", "error", err.Error())
		return
	}

	// Process command asynchronously and send delayed response
	go h.processCommand(r.Context(), cmdDTO, startTime)
}

// processCommand processes the command and sends delayed response via response_url.
func (h *SlackCommandsHandler) processCommand(ctx context.Context, cmd *dto.SlackCommandDTO, startTime time.Time) {
	// Route based on command
	switch cmd.Command {
	case "/alert-status":
		h.handleAlertStatus(ctx, cmd, startTime)
	default:
		h.logger.Warn("unhandled slash command", "command", cmd.Command)
		h.sendDelayedResponse(cmd.ResponseURL, dto.NewEphemeralResponse("Unknown command"))
	}
}

// handleAlertStatus handles /alert-status command.
func (h *SlackCommandsHandler) handleAlertStatus(ctx context.Context, cmd *dto.SlackCommandDTO, startTime time.Time) {
	// Extract severity filter from command text
	severity := cmd.SeverityFilter()

	// Query alerts
	alerts, err := h.queryAlertStatus.Execute(ctx, severity)
	if err != nil {
		h.logger.Error("failed to query alert status",
			"error", err.Error(),
			"user_id", cmd.UserID,
			"severity", severity)

		h.sendDelayedResponse(cmd.ResponseURL,
			dto.NewEphemeralResponse("Failed to fetch alert status. Please try again later."))
		return
	}

	// Format alerts as Slack blocks
	blocks := h.formatter.FormatAlertStatus(alerts, severity)

	// Create response
	response := dto.NewEphemeralWithBlocks(
		fmt.Sprintf("Found %d active alert(s)", len(alerts)),
		blocks,
	)

	// Send delayed response
	h.sendDelayedResponse(cmd.ResponseURL, response)

	// Log response time (SLA: <2s)
	elapsed := time.Since(startTime)
	h.logger.Info("slash command processed",
		"command", cmd.Command,
		"user_id", cmd.UserID,
		"severity", severity,
		"alert_count", len(alerts),
		"response_time_ms", elapsed.Milliseconds(),
		"sla_met", elapsed < 2*time.Second)
}

// sendDelayedResponse sends a delayed response to Slack via response_url.
func (h *SlackCommandsHandler) sendDelayedResponse(responseURL string, response *dto.SlackResponseDTO) {
	if responseURL == "" {
		h.logger.Error("response_url is empty, cannot send delayed response")
		return
	}

	// Marshal response to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		h.logger.Error("failed to marshal delayed response", "error", err.Error())
		return
	}

	// POST to response_url
	resp, err := http.Post(responseURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		h.logger.Error("failed to send delayed response", "error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("delayed response failed",
			"status_code", resp.StatusCode,
			"status", resp.Status)
		return
	}

	h.logger.Debug("delayed response sent successfully")
}
