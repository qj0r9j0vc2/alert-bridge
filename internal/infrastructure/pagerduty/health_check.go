package pagerduty

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthCheckResult represents the result of a health check operation.
type HealthCheckResult struct {
	Healthy       bool
	ServiceID     string
	ServiceName   string
	CheckedAt     time.Time
	Error         error
	ResponseTime  time.Duration
}

// HealthChecker defines the interface for on-demand PagerDuty health checks.
type HealthChecker interface {
	// Check performs an on-demand health check against PagerDuty REST API.
	// Returns cached result if check was performed within cache TTL (5 minutes).
	Check(ctx context.Context) (*HealthCheckResult, error)

	// LastCheckTime returns the timestamp of the last health check.
	LastCheckTime() time.Time

	// LastCheckResult returns the result of the last health check (may be nil if never checked).
	LastCheckResult() *HealthCheckResult
}

// healthChecker implements HealthChecker with thread-safe caching.
type healthChecker struct {
	client    RESTClient
	serviceID string
	cacheTTL  time.Duration

	mu         sync.RWMutex
	lastCheck  time.Time
	lastResult *HealthCheckResult
}

// NewHealthChecker creates a new HealthChecker with 5-minute cache TTL.
func NewHealthChecker(client RESTClient, serviceID string) HealthChecker {
	return &healthChecker{
		client:    client,
		serviceID: serviceID,
		cacheTTL:  5 * time.Minute,
	}
}

// Check performs an on-demand health check with caching.
// Returns cached result if available and within TTL, otherwise performs fresh check.
func (h *healthChecker) Check(ctx context.Context) (*HealthCheckResult, error) {
	// Check if we have a valid cached result
	h.mu.RLock()
	if h.lastResult != nil && time.Since(h.lastCheck) < h.cacheTTL {
		cachedResult := h.lastResult
		h.mu.RUnlock()
		return cachedResult, nil
	}
	h.mu.RUnlock()

	// Perform fresh health check
	startTime := time.Now()
	result := &HealthCheckResult{
		ServiceID: h.serviceID,
		CheckedAt: startTime,
	}

	// Call GetService to validate connectivity and service accessibility
	service, err := h.client.GetService(ctx, h.serviceID)

	responseTime := time.Since(startTime)
	result.ResponseTime = responseTime

	if err != nil {
		result.Healthy = false
		result.Error = err
	} else {
		result.Healthy = true
		result.ServiceName = service.Name
		result.Error = nil
	}

	// Cache the result
	h.mu.Lock()
	h.lastCheck = startTime
	h.lastResult = result
	h.mu.Unlock()

	return result, err
}

// LastCheckTime returns the timestamp of the last health check (thread-safe).
func (h *healthChecker) LastCheckTime() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastCheck
}

// LastCheckResult returns the result of the last health check (thread-safe).
// Returns nil if no health check has been performed yet.
func (h *healthChecker) LastCheckResult() *HealthCheckResult {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastResult
}

// String returns a human-readable representation of the health check result.
func (r *HealthCheckResult) String() string {
	if r == nil {
		return "no health check performed"
	}

	if r.Healthy {
		return fmt.Sprintf("✓ Healthy - service=%s (%s), checked=%s, response_time=%s",
			r.ServiceID, r.ServiceName, r.CheckedAt.Format(time.RFC3339), r.ResponseTime)
	}

	return fmt.Sprintf("✗ Unhealthy - service=%s, checked=%s, error=%v",
		r.ServiceID, r.CheckedAt.Format(time.RFC3339), r.Error)
}
