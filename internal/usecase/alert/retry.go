package alert

import (
	"context"
	"math"
	"math/rand"
	"time"

	domainerrors "github.com/qj0r9j0vc2/alert-bridge/internal/domain/errors"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
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

// RetryableNotifier wraps a Notifier with retry logic for transient failures.
// It implements the Decorator pattern to add retry capabilities.
type RetryableNotifier struct {
	notifier Notifier
	policy   RetryPolicy
	logger   Logger
}

// NewRetryableNotifier creates a new RetryableNotifier with the given policy.
func NewRetryableNotifier(notifier Notifier, policy RetryPolicy, logger Logger) *RetryableNotifier {
	return &RetryableNotifier{
		notifier: notifier,
		policy:   policy,
		logger:   logger,
	}
}

// Notify sends a notification with retry logic for transient failures.
func (r *RetryableNotifier) Notify(ctx context.Context, alert *entity.Alert) (string, error) {
	var lastErr error
	var messageID string

	for attempt := 1; attempt <= r.policy.MaxAttempts; attempt++ {
		// Attempt notification
		messageID, lastErr = r.notifier.Notify(ctx, alert)

		// Success - return immediately
		if lastErr == nil {
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
