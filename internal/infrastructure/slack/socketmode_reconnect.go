package slack

import (
	"math"
	"time"
)

// ReconnectionConfig holds configuration for reconnection logic.
type ReconnectionConfig struct {
	InitialBackoff    time.Duration // Initial backoff delay (default: 500ms)
	MaxBackoff        time.Duration // Maximum backoff delay (default: 60s)
	BackoffMultiplier float64       // Backoff multiplier (default: 1.5)
	MaxRetries        int           // Maximum consecutive failures before circuit breaker opens (default: 5)
}

// DefaultReconnectionConfig returns default reconnection configuration.
func DefaultReconnectionConfig() ReconnectionConfig {
	return ReconnectionConfig{
		InitialBackoff:    500 * time.Millisecond,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 1.5,
		MaxRetries:        5,
	}
}

// CircuitBreaker tracks connection failure state.
type CircuitBreaker struct {
	consecutiveFailures int
	maxFailures         int
	isOpen              bool
	lastFailure         time.Time
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(maxFailures int) *CircuitBreaker {
	return &CircuitBreaker{
		consecutiveFailures: 0,
		maxFailures:         maxFailures,
		isOpen:              false,
	}
}

// RecordFailure records a connection failure.
// Returns true if circuit breaker is now open.
func (cb *CircuitBreaker) RecordFailure() bool {
	cb.consecutiveFailures++
	cb.lastFailure = time.Now()

	if cb.consecutiveFailures >= cb.maxFailures {
		cb.isOpen = true
		return true
	}

	return false
}

// RecordSuccess resets the circuit breaker on successful connection.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.consecutiveFailures = 0
	cb.isOpen = false
}

// IsOpen returns true if the circuit breaker is open.
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.isOpen
}

// ConsecutiveFailures returns the number of consecutive failures.
func (cb *CircuitBreaker) ConsecutiveFailures() int {
	return cb.consecutiveFailures
}

// CalculateBackoff calculates the backoff duration based on attempt number.
// Uses exponential backoff with jitter.
func CalculateBackoff(cfg ReconnectionConfig, attempt int) time.Duration {
	// Calculate exponential backoff
	backoff := float64(cfg.InitialBackoff) * math.Pow(cfg.BackoffMultiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}

	return time.Duration(backoff)
}

// ShouldRetry determines if reconnection should be attempted.
// Returns false if circuit breaker is open.
func ShouldRetry(cb *CircuitBreaker) bool {
	return !cb.IsOpen()
}
