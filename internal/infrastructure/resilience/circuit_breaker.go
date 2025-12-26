package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	// StateClosed allows all requests through.
	StateClosed State = iota
	// StateOpen rejects all requests.
	StateOpen
	// StateHalfOpen allows limited requests to test recovery.
	StateHalfOpen
)

// String returns the string representation of the state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures.
type CircuitBreaker struct {
	name         string
	maxFailures  int
	timeout      time.Duration
	halfOpenSucc int // Successes needed in half-open to close

	mu           sync.RWMutex
	state        State
	failures     int
	lastFailTime time.Time
	successCount int
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		timeout:      timeout,
		halfOpenSucc: 2, // Require 2 successes to close
		state:        StateClosed,
	}
}

// Execute runs the given function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	err := fn()
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request should be allowed.
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailTime) > cb.timeout {
			// Transition to half-open
			cb.state = StateHalfOpen
			cb.successCount = 0
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen, StateClosed:
		return nil

	default:
		return nil
	}
}

// afterRequest updates the circuit breaker state based on the result.
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Failure
		cb.failures++
		cb.lastFailTime = time.Now()

		if cb.state == StateHalfOpen {
			// Failure in half-open -> reopen
			cb.state = StateOpen
		} else if cb.failures >= cb.maxFailures {
			// Too many failures -> open
			cb.state = StateOpen
		}
	} else {
		// Success
		if cb.state == StateHalfOpen {
			cb.successCount++
			if cb.successCount >= cb.halfOpenSucc {
				// Enough successes -> close
				cb.state = StateClosed
				cb.failures = 0
			}
		} else if cb.state == StateClosed {
			// Reset failure counter on success
			cb.failures = 0
		}
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Name returns the circuit breaker name.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// Failures returns the current failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}
