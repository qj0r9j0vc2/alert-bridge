package alert

import (
	"context"
	"math"
	"math/rand"
	"time"

	domainerrors "github.com/qj0r9j0vc2/alert-bridge/internal/domain/errors"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/observability"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/resilience"
)

// RetryPolicy defines the retry behavior for failed operations.
type RetryPolicy struct {
	MaxAttempts     int           // Maximum number of retry attempts (including first try)
	InitialInterval time.Duration // Initial backoff interval
	MaxInterval     time.Duration // Maximum backoff interval
	Multiplier      float64       // Backoff multiplier
	JitterFactor    float64       // Random jitter factor (0.0-1.0)
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2.0,
		JitterFactor:    0.1,
	}
}

// RetryableNotifier wraps a Notifier with retry logic and circuit breaker for transient failures.
// It implements the Decorator pattern to add retry and resilience capabilities.
type RetryableNotifier struct {
	notifier       Notifier
	policy         RetryPolicy
	logger         Logger
	metrics        *observability.Metrics
	circuitBreaker *resilience.CircuitBreaker
}

// NewRetryableNotifier creates a new RetryableNotifier with the given policy.
func NewRetryableNotifier(notifier Notifier, policy RetryPolicy, logger Logger, metrics *observability.Metrics) *RetryableNotifier {
	// Create circuit breaker for this notifier
	cb := resilience.NewCircuitBreaker(
		notifier.Name(),
		5,              // max 5 failures
		30*time.Second, // 30s timeout
	)

	return &RetryableNotifier{
		notifier:       notifier,
		policy:         policy,
		logger:         logger,
		metrics:        metrics,
		circuitBreaker: cb,
	}
}

// Notify sends a notification with retry logic for transient failures.
func (r *RetryableNotifier) Notify(ctx context.Context, alert *entity.Alert) (string, error) {
	start := time.Now()
	var lastErr error
	var messageID string
	var success bool
	retriesUsed := 0

	defer func() {
		duration := time.Since(start)
		if r.metrics != nil {
			r.metrics.RecordNotificationSent(
				ctx,
				r.notifier.Name(),
				success,
				duration,
				retriesUsed,
			)
		}
	}()

	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		if attempt > 1 {
			retriesUsed++
		}

		// Attempt notification through circuit breaker
		cbErr := r.circuitBreaker.Execute(ctx, func() error {
			messageID, lastErr = r.notifier.Notify(ctx, alert)
			return lastErr
		})

		// Check if circuit breaker blocked the request
		if cbErr == resilience.ErrCircuitOpen {
			r.logger.Warn("circuit breaker open, skipping notification",
				"notifier", r.notifier.Name(),
				"alert_id", alert.ID,
				"cb_state", r.circuitBreaker.State(),
			)
			return "", cbErr
		}

		// Success - return immediately
		if lastErr == nil {
			success = true
			if attempt > 1 {
				r.logger.Info("notification succeeded after retry",
					"notifier", r.notifier.Name(),
					"alert_id", alert.ID,
					"attempt", attempt,
				)
			}
			return messageID, nil
		}

		// Check if error is retryable
		if !domainerrors.IsTransientError(lastErr) {
			// Permanent error - don't retry
			r.logger.Warn("notification failed with permanent error",
				"notifier", r.notifier.Name(),
				"alert_id", alert.ID,
				"error", lastErr,
			)
			return "", lastErr
		}

		// Last attempt failed - don't sleep
		if attempt == r.policy.MaxAttempts {
			r.logger.Error("notification failed after max retries",
				"notifier", r.notifier.Name(),
				"alert_id", alert.ID,
				"attempts", attempt,
				"error", lastErr,
			)
			break
		}

		// Calculate backoff with jitter
		backoff := r.calculateBackoff(attempt)
		r.logger.Warn("notification failed, retrying",
			"notifier", r.notifier.Name(),
			"alert_id", alert.ID,
			"attempt", attempt,
			"backoff", backoff,
			"error", lastErr,
		)

		// Wait before retry (check context cancellation)
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return "", lastErr
}

// UpdateMessage updates a notification with retry logic.
func (r *RetryableNotifier) UpdateMessage(ctx context.Context, messageID string, alert *entity.Alert) error {
	var lastErr error

	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		lastErr = r.notifier.UpdateMessage(ctx, messageID, alert)

		// Success
		if lastErr == nil {
			if attempt > 1 {
				r.logger.Info("update message succeeded after retry",
					"notifier", r.notifier.Name(),
					"message_id", messageID,
					"attempt", attempt,
				)
			}
			return nil
		}

		// Check if error is retryable
		if !domainerrors.IsTransientError(lastErr) {
			r.logger.Warn("update message failed with permanent error",
				"notifier", r.notifier.Name(),
				"message_id", messageID,
				"error", lastErr,
			)
			return lastErr
		}

		// Last attempt failed
		if attempt == r.policy.MaxAttempts {
			r.logger.Error("update message failed after max retries",
				"notifier", r.notifier.Name(),
				"message_id", messageID,
				"attempts", attempt,
				"error", lastErr,
			)
			break
		}

		// Calculate backoff with jitter
		backoff := r.calculateBackoff(attempt)
		r.logger.Warn("update message failed, retrying",
			"notifier", r.notifier.Name(),
			"message_id", messageID,
			"attempt", attempt,
			"backoff", backoff,
			"error", lastErr,
		)

		// Wait before retry
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

// Name returns the underlying notifier name.
func (r *RetryableNotifier) Name() string {
	return r.notifier.Name()
}

// calculateBackoff calculates the backoff duration with exponential growth and jitter.
// Formula: min(InitialInterval * Multiplier^(attempt-1) * (1 ± jitter), MaxInterval)
func (r *RetryableNotifier) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff
	backoff := float64(r.policy.InitialInterval) * math.Pow(r.policy.Multiplier, float64(attempt-1))

	// Apply jitter (-jitterFactor to +jitterFactor)
	jitter := 1.0 + (rand.Float64()*2.0-1.0)*r.policy.JitterFactor
	backoff *= jitter

	// Cap at max interval
	if backoff > float64(r.policy.MaxInterval) {
		backoff = float64(r.policy.MaxInterval)
	}

	return time.Duration(backoff)
}
