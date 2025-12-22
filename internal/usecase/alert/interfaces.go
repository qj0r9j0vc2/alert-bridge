package alert

import (
	"context"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// Notifier defines the contract for sending alert notifications.
// OCP: New notification channels implement this interface.
type Notifier interface {
	// Notify sends an alert to the notification channel.
	// Returns a channel-specific message ID for tracking.
	Notify(ctx context.Context, alert *entity.Alert) (messageID string, err error)

	// UpdateMessage updates an existing notification (e.g., after ack or resolve).
	UpdateMessage(ctx context.Context, messageID string, alert *entity.Alert) error

	// Name returns the notifier identifier (e.g., "slack", "pagerduty").
	Name() string
}

// Logger defines the contract for logging within use cases.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}
