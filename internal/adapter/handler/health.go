package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// SlackStatusProvider provides Slack connection status.
type SlackStatusProvider interface {
	IsConnected() bool
	ConnectionID() string
	LastReconnect() time.Time
}

// HealthHandler handles health check requests.
type HealthHandler struct {
	startTime           time.Time
	slackEnabled        bool
	slackSocketMode     bool
	slackStatusProvider SlackStatusProvider
	mu                  sync.RWMutex
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		startTime: time.Now(),
	}
}

// SetSlackStatus configures Slack status reporting.
func (h *HealthHandler) SetSlackStatus(enabled, socketMode bool, provider SlackStatusProvider) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.slackEnabled = enabled
	h.slackSocketMode = socketMode
	h.slackStatusProvider = provider
}

// ServeHTTP handles GET /health and GET /ready
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	response := map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
	}

	// Add Slack status if enabled
	if h.slackEnabled {
		slackStatus := map[string]any{
			"enabled": true,
		}

		if h.slackSocketMode {
			slackStatus["mode"] = "socket"
			if h.slackStatusProvider != nil {
				slackStatus["connected"] = h.slackStatusProvider.IsConnected()
				if h.slackStatusProvider.IsConnected() {
					slackStatus["connection_id"] = h.slackStatusProvider.ConnectionID()
					slackStatus["last_reconnect"] = h.slackStatusProvider.LastReconnect().Format(time.RFC3339)
				}
			}
		} else {
			slackStatus["mode"] = "http"
			slackStatus["connected"] = true // HTTP mode doesn't maintain persistent connections
		}

		response["slack"] = slackStatus
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
