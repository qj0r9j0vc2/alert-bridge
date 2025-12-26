package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// V2Event represents a PagerDuty Events API v2 event
type V2Event struct {
	RoutingKey  string       `json:"routing_key"`
	EventAction string       `json:"event_action"`
	DedupKey    string       `json:"dedup_key,omitempty"`
	Payload     *EventPayload `json:"payload,omitempty"`
	Images      []Image      `json:"images,omitempty"`
	Links       []Link       `json:"links,omitempty"`
}

// EventPayload represents the event payload
type EventPayload struct {
	Summary       string                 `json:"summary"`
	Source        string                 `json:"source"`
	Severity      string                 `json:"severity"`
	Timestamp     string                 `json:"timestamp,omitempty"`
	Component     string                 `json:"component,omitempty"`
	Group         string                 `json:"group,omitempty"`
	Class         string                 `json:"class,omitempty"`
	CustomDetails map[string]interface{} `json:"custom_details,omitempty"`
}

// Image represents an image attachment
type Image struct {
	Src  string `json:"src"`
	Href string `json:"href,omitempty"`
	Alt  string `json:"alt,omitempty"`
}

// Link represents a link attachment
type Link struct {
	Href string `json:"href"`
	Text string `json:"text,omitempty"`
}

// V2EventResponse represents the PagerDuty API response
type V2EventResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	DedupKey string `json:"dedup_key,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Errors  []string `json:"errors,omitempty"`
}

// StoredEvent represents an event stored in the mock service
type StoredEvent struct {
	V2Event
	EventID    string    `json:"event_id"`
	ReceivedAt time.Time `json:"received_at"`
}

// MockPagerDutyHandler handles PagerDuty API requests
type MockPagerDutyHandler struct {
	mu       sync.RWMutex
	events   []StoredEvent
	nextID   int64
}

// NewMockPagerDutyHandler creates a new mock PagerDuty handler
func NewMockPagerDutyHandler() *MockPagerDutyHandler {
	return &MockPagerDutyHandler{
		events: make([]StoredEvent, 0),
		nextID: 1,
	}
}

// ServeHTTP implements the http.Handler interface
func (h *MockPagerDutyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/health":
		h.handleHealth(w, r)
	case "/v2/enqueue":
		h.handleEnqueue(w, r)
	case "/api/test/events":
		h.handleGetEvents(w, r)
	case "/api/test/reset":
		h.handleReset(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleHealth returns service health status
func (h *MockPagerDutyHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// handleEnqueue validates and stores a PagerDuty event
func (h *MockPagerDutyHandler) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var event V2Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Status:  "error",
			Message: "Invalid JSON",
			Errors:  []string{err.Error()},
		})
		return
	}

	// Validate event
	if errs := h.validateEvent(&event); len(errs) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Status:  "error",
			Message: "Validation failed",
			Errors:  errs,
		})
		return
	}

	// Generate event ID
	h.mu.Lock()
	eventID := fmt.Sprintf("evt_%d", h.nextID)
	h.nextID++

	// Use dedup_key if provided, otherwise generate one
	dedupKey := event.DedupKey
	if dedupKey == "" {
		dedupKey = eventID
	}

	// Store event
	stored := StoredEvent{
		V2Event:    event,
		EventID:    eventID,
		ReceivedAt: time.Now(),
	}
	stored.DedupKey = dedupKey
	h.events = append(h.events, stored)
	h.mu.Unlock()

	// Return success response (202 Accepted)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(V2EventResponse{
		Status:   "success",
		Message:  "Event processed",
		DedupKey: dedupKey,
	})
}

// handleGetEvents returns stored events (test helper endpoint)
func (h *MockPagerDutyHandler) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse query filters
	dedupKey := r.URL.Query().Get("dedup_key")
	action := r.URL.Query().Get("action")

	h.mu.RLock()
	result := make([]StoredEvent, 0)
	for _, evt := range h.events {
		// Apply filters
		if dedupKey != "" && evt.DedupKey != dedupKey {
			continue
		}
		if action != "" && evt.EventAction != action {
			continue
		}
		result = append(result, evt)
	}
	h.mu.RUnlock()

	json.NewEncoder(w).Encode(result)
}

// handleReset clears all stored events (test helper endpoint)
func (h *MockPagerDutyHandler) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	h.mu.Lock()
	cleared := len(h.events)
	h.events = make([]StoredEvent, 0)
	h.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "reset_complete",
		"events_cleared": cleared,
	})
}

// validateEvent validates a PagerDuty event
func (h *MockPagerDutyHandler) validateEvent(event *V2Event) []string {
	errors := make([]string, 0)

	// Validate routing_key
	if event.RoutingKey == "" {
		errors = append(errors, "routing_key is required")
	}

	// Validate event_action
	validActions := map[string]bool{
		"trigger":     true,
		"acknowledge": true,
		"resolve":     true,
	}
	if !validActions[event.EventAction] {
		errors = append(errors, fmt.Sprintf("event_action must be one of: trigger, acknowledge, resolve (got: %s)", event.EventAction))
	}

	// For trigger events, payload is required
	if event.EventAction == "trigger" {
		if event.Payload == nil {
			errors = append(errors, "payload is required for trigger events")
		} else {
			// Validate payload fields
			if event.Payload.Summary == "" {
				errors = append(errors, "payload.summary is required for trigger events")
			}
			if len(event.Payload.Summary) > 1024 {
				errors = append(errors, "payload.summary must not exceed 1024 characters")
			}
			if event.Payload.Source == "" {
				errors = append(errors, "payload.source is required for trigger events")
			}
			if event.Payload.Severity == "" {
				errors = append(errors, "payload.severity is required for trigger events")
			} else {
				validSeverities := map[string]bool{
					"critical": true,
					"error":    true,
					"warning":  true,
					"info":     true,
				}
				if !validSeverities[event.Payload.Severity] {
					errors = append(errors, fmt.Sprintf("payload.severity must be one of: critical, error, warning, info (got: %s)", event.Payload.Severity))
				}
			}
		}
	}

	return errors
}
