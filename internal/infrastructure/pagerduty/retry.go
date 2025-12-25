package pagerduty

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// RetryPolicy defines the configuration for exponential backoff retry logic.
type RetryPolicy struct {
	InitialDelay time.Duration // Initial delay between retries (100ms)
	Multiplier   float64       // Multiplier for exponential backoff (2x)
	MaxDelay     time.Duration // Maximum delay between retries (5s)
	MaxRetries   int           // Maximum number of retry attempts (3)
}

// DefaultRetryPolicy returns a RetryPolicy configured with standard exponential backoff parameters.
// Initial: 100ms, Multiplier: 2x, Max: 5s, Max retries: 3
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		InitialDelay: 100 * time.Millisecond,
		Multiplier:   2.0,
		MaxDelay:     5 * time.Second,
		MaxRetries:   3,
	}
}

// WithRetry executes the provided function with exponential backoff retry logic.
// It retries on transient errors (5xx, 429, network errors) and respects context cancellation.
// Returns the result of the last attempt or an error if all retries exhausted.
func (r *RetryPolicy) WithRetry(ctx context.Context, operation func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		// Execute the operation
		err := operation(ctx)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Check if we've exhausted retries
		if attempt == r.MaxRetries {
			return fmt.Errorf("max retries (%d) exhausted: %w", r.MaxRetries, lastErr)
		}

		// Calculate delay for next attempt
		delay := r.calculateDelay(attempt)

		// Wait with context cancellation support
		select {
		case <-time.After(delay):
			// Continue to next retry attempt
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", r.MaxRetries, lastErr)
}

// calculateDelay computes the exponential backoff delay for a given attempt.
// Formula: min(InitialDelay * (Multiplier ^ attempt), MaxDelay)
func (r *RetryPolicy) calculateDelay(attempt int) time.Duration {
	// Calculate exponential delay: InitialDelay * (Multiplier ^ attempt)
	delay := float64(r.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= r.Multiplier
	}

	// Cap at MaxDelay
	if time.Duration(delay) > r.MaxDelay {
		return r.MaxDelay
	}

	return time.Duration(delay)
}

// IsRetryable determines if an error should trigger a retry attempt.
// Retryable errors: 5xx server errors, 429 rate limits, network errors
// Non-retryable errors: 4xx client errors, context cancellation, validation errors
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation - never retry
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for HTTP errors via PagerDuty SDK error
	// The go-pagerduty SDK wraps HTTP errors, so we need to check error message
	errMsg := err.Error()

	// HTTP 5xx errors - server errors (retryable)
	if containsAny(errMsg, []string{"500", "502", "503", "504", "internal server error", "bad gateway", "service unavailable", "gateway timeout"}) {
		return true
	}

	// HTTP 429 - rate limit exceeded (retryable)
	if containsAny(errMsg, []string{"429", "too many requests", "rate limit"}) {
		return true
	}

	// Network errors (retryable)
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Temporary network errors are retryable
		if netErr.Temporary() {
			return true
		}
		// Timeout errors are retryable
		if netErr.Timeout() {
			return true
		}
	}

	// Connection errors (retryable)
	if containsAny(errMsg, []string{"connection refused", "connection reset", "no route to host", "network unreachable"}) {
		return true
	}

	// HTTP 4xx errors - client errors (NOT retryable)
	// These indicate bad request, authentication issues, not found, etc.
	if containsAny(errMsg, []string{"400", "401", "403", "404", "bad request", "unauthorized", "forbidden", "not found"}) {
		return false
	}

	// Default: treat unknown errors as non-retryable to avoid infinite loops
	return false
}

// containsAny checks if the string contains any of the given substrings (case-insensitive).
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains performs case-insensitive substring check.
func contains(s, substr string) bool {
	// Simple case-insensitive check
	sLower := toLower(s)
	substrLower := toLower(substr)
	return containsSimple(sLower, substrLower)
}

// toLower converts string to lowercase (simple ASCII implementation)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// containsSimple checks if s contains substr
func containsSimple(s, substr string) bool {
	return len(s) >= len(substr) && indexSimple(s, substr) >= 0
}

// indexSimple finds the index of substr in s, or -1 if not found
func indexSimple(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	if n > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
