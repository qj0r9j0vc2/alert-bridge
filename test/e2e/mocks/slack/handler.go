package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SlackMessage represents a Slack Block Kit message
type SlackMessage struct {
	Channel  string        `json:"channel"`
	Text     string        `json:"text,omitempty"`
	Blocks   []interface{} `json:"blocks,omitempty"`
	ThreadTS string        `json:"thread_ts,omitempty"`
	TS       string        `json:"ts,omitempty"` // For updates
}

// SlackResponse represents the Slack API response
type SlackResponse struct {
	OK      bool   `json:"ok"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	Error   string `json:"error,omitempty"`
	Details string `json:"details,omitempty"`
}

// StoredMessage represents a message stored in the mock service
type StoredMessage struct {
	SlackMessage
	MessageID  string    `json:"message_id"`
	ReceivedAt time.Time `json:"received_at"`
}

// MockSlackHandler handles Slack API requests
type MockSlackHandler struct {
	mu       sync.RWMutex
	messages []StoredMessage
	nextTS   int64
}

// NewMockSlackHandler creates a new mock Slack handler
func NewMockSlackHandler() *MockSlackHandler {
	return &MockSlackHandler{
		messages: make([]StoredMessage, 0),
		nextTS:   time.Now().Unix(),
	}
}

// ServeHTTP implements the http.Handler interface
func (h *MockSlackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/health":
		h.handleHealth(w, r)
	case "/api/chat.postMessage":
		h.handlePostMessage(w, r)
	case "/api/chat.update":
		h.handleUpdateMessage(w, r)
	case "/api/test/messages":
		h.handleGetMessages(w, r)
	case "/api/test/reset":
		h.handleReset(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleHealth returns service health status
func (h *MockSlackHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// handlePostMessage validates and stores a new Slack message
func (h *MockSlackHandler) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var msg SlackMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{
			OK:      false,
			Error:   "invalid_json",
			Details: err.Error(),
		})
		return
	}

	// Validate required fields
	if msg.Channel == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{
			OK:      false,
			Error:   "invalid_channel",
			Details: "Channel is required",
		})
		return
	}

	// Validate Block Kit structure
	if err := h.validateBlocks(msg.Blocks); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{
			OK:      false,
			Error:   "invalid_blocks",
			Details: err.Error(),
		})
		return
	}

	// Generate message timestamp
	h.mu.Lock()
	h.nextTS++
	ts := fmt.Sprintf("%d.%06d", h.nextTS, time.Now().Nanosecond()/1000)

	// Store message
	stored := StoredMessage{
		SlackMessage: msg,
		MessageID:    ts,
		ReceivedAt:   time.Now(),
	}
	stored.TS = ts
	h.messages = append(h.messages, stored)
	h.mu.Unlock()

	// Return success response
	json.NewEncoder(w).Encode(SlackResponse{
		OK:      true,
		Channel: msg.Channel,
		TS:      ts,
	})
}

// handleUpdateMessage updates an existing message
func (h *MockSlackHandler) handleUpdateMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var msg SlackMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{
			OK:      false,
			Error:   "invalid_json",
			Details: err.Error(),
		})
		return
	}

	if msg.TS == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SlackResponse{
			OK:      false,
			Error:   "message_not_found",
			Details: "ts is required for updates",
		})
		return
	}

	// Find and update message
	h.mu.Lock()
	found := false
	for i, stored := range h.messages {
		if stored.MessageID == msg.TS {
			h.messages[i].SlackMessage = msg
			h.messages[i].TS = msg.TS
			found = true
			break
		}
	}
	h.mu.Unlock()

	if !found {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(SlackResponse{
			OK:      false,
			Error:   "message_not_found",
			Details: fmt.Sprintf("Message with ts=%s not found", msg.TS),
		})
		return
	}

	json.NewEncoder(w).Encode(SlackResponse{
		OK:      true,
		Channel: msg.Channel,
		TS:      msg.TS,
	})
}

// handleGetMessages returns stored messages (test helper endpoint)
func (h *MockSlackHandler) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse query filters
	fingerprint := r.URL.Query().Get("fingerprint")
	channel := r.URL.Query().Get("channel")

	h.mu.RLock()
	result := make([]StoredMessage, 0)
	for _, msg := range h.messages {
		// Apply filters
		if channel != "" && msg.Channel != channel {
			continue
		}
		if fingerprint != "" {
			// Check if fingerprint appears in text or blocks
			if !h.containsFingerprint(msg, fingerprint) {
				continue
			}
		}
		result = append(result, msg)
	}
	h.mu.RUnlock()

	json.NewEncoder(w).Encode(result)
}

// handleReset clears all stored messages (test helper endpoint)
func (h *MockSlackHandler) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	h.mu.Lock()
	cleared := len(h.messages)
	h.messages = make([]StoredMessage, 0)
	h.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "reset_complete",
		"messages_cleared": cleared,
	})
}

// validateBlocks validates Slack Block Kit structure
func (h *MockSlackHandler) validateBlocks(blocks []interface{}) error {
	if len(blocks) == 0 {
		return nil // Blocks are optional
	}

	for i, block := range blocks {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			return fmt.Errorf("block[%d] is not a valid object", i)
		}

		blockType, ok := blockMap["type"].(string)
		if !ok {
			return fmt.Errorf("block[%d] missing required 'type' field", i)
		}

		// Validate based on block type
		switch blockType {
		case "header":
			if err := h.validateHeaderBlock(blockMap, i); err != nil {
				return err
			}
		case "section":
			if err := h.validateSectionBlock(blockMap, i); err != nil {
				return err
			}
		case "actions", "divider":
			// These types have flexible validation
		default:
			return fmt.Errorf("block[%d] has unsupported type: %s", i, blockType)
		}
	}

	return nil
}

// validateHeaderBlock validates header block structure
func (h *MockSlackHandler) validateHeaderBlock(block map[string]interface{}, index int) error {
	text, ok := block["text"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("block[%d] header missing required 'text' field", index)
	}

	textType, ok := text["type"].(string)
	if !ok || textType != "plain_text" {
		return fmt.Errorf("block[%d] header text must have type 'plain_text'", index)
	}

	if _, ok := text["text"].(string); !ok {
		return fmt.Errorf("block[%d] header text missing 'text' string", index)
	}

	return nil
}

// validateSectionBlock validates section block structure
func (h *MockSlackHandler) validateSectionBlock(block map[string]interface{}, index int) error {
	// Section blocks can have either 'text' or 'fields', both are optional
	return nil
}

// containsFingerprint checks if a message contains a fingerprint
func (h *MockSlackHandler) containsFingerprint(msg StoredMessage, fingerprint string) bool {
	// Check in text field
	if msg.Text != "" && contains(msg.Text, fingerprint) {
		return true
	}

	// Check in blocks (simplified search)
	blocksJSON, _ := json.Marshal(msg.Blocks)
	return contains(string(blocksJSON), fingerprint)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
