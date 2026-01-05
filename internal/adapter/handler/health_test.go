package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_ServeHTTP(t *testing.T) {
	h := NewHealthHandler()

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET returns 200",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST returns 405",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHealthHandler_ResponseFormat(t *testing.T) {
	h := NewHealthHandler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Check required fields
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}

	if _, ok := resp["timestamp"]; !ok {
		t.Error("expected timestamp in response")
	}

	if _, ok := resp["uptime"]; !ok {
		t.Error("expected uptime in response")
	}
}

// mockChecker implements ReadinessChecker for testing
type mockChecker struct {
	err error
}

func (m *mockChecker) Ping(ctx context.Context) error {
	return m.err
}

func TestReadyHandler_ServeHTTP_AllReady(t *testing.T) {
	h := NewReadyHandler()
	h.AddChecker("database", &mockChecker{err: nil})
	h.AddChecker("cache", &mockChecker{err: nil})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["ready"] != true {
		t.Errorf("expected ready=true, got %v", resp["ready"])
	}

	checks := resp["checks"].(map[string]any)
	for name, check := range checks {
		checkMap := check.(map[string]any)
		if checkMap["ready"] != true {
			t.Errorf("expected %s to be ready", name)
		}
	}
}

func TestReadyHandler_ServeHTTP_SomeNotReady(t *testing.T) {
	h := NewReadyHandler()
	h.AddChecker("database", &mockChecker{err: nil})
	h.AddChecker("external-api", &mockChecker{err: errors.New("connection refused")})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["ready"] != false {
		t.Errorf("expected ready=false, got %v", resp["ready"])
	}

	checks := resp["checks"].(map[string]any)
	
	dbCheck := checks["database"].(map[string]any)
	if dbCheck["ready"] != true {
		t.Error("expected database to be ready")
	}

	apiCheck := checks["external-api"].(map[string]any)
	if apiCheck["ready"] != false {
		t.Error("expected external-api to be not ready")
	}
	if apiCheck["error"] != "connection refused" {
		t.Errorf("expected error message, got %v", apiCheck["error"])
	}
}

func TestReadyHandler_ServeHTTP_NoCheckers(t *testing.T) {
	h := NewReadyHandler()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// With no checkers, should return ready
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with no checkers, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["ready"] != true {
		t.Errorf("expected ready=true with no checkers, got %v", resp["ready"])
	}
}

func TestReadyHandler_MethodNotAllowed(t *testing.T) {
	h := NewReadyHandler()

	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
