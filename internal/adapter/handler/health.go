package handler

import (
	"context"
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


// ReadinessChecker provides a way to check if a dependency is ready.
// Implementations should return nil if ready, or an error describing the issue.
type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

// ReadyHandler handles readiness check requests.
// Unlike HealthHandler (liveness), this checks actual dependencies.
type ReadyHandler struct {
	checkers map[string]ReadinessChecker
	mu       sync.RWMutex
}

// NewReadyHandler creates a new readiness handler.
func NewReadyHandler() *ReadyHandler {
	return &ReadyHandler{
		checkers: make(map[string]ReadinessChecker),
	}
}

// AddChecker registers a dependency checker with the given name.
func (h *ReadyHandler) AddChecker(name string, checker ReadinessChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// ServeHTTP handles GET /ready
func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	ctx := r.Context()
	checks := make(map[string]any)
	allReady := true

	for name, checker := range h.checkers {
		if err := checker.Ping(ctx); err != nil {
			checks[name] = map[string]any{
				"ready": false,
				"error": err.Error(),
			}
			allReady = false
		} else {
			checks[name] = map[string]any{
				"ready": true,
			}
		}
	}

	response := map[string]any{
		"ready":     allReady,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks":    checks,
	}

	w.Header().Set("Content-Type", "application/json")

	if allReady {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}
