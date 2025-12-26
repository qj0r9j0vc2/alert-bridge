package repository

import (
	"context"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// AlertRepository defines the contract for alert persistence.
// Following ISP: focused on alert storage operations only.
type AlertRepository interface {
	// Save persists a new alert.
	// Returns ErrDuplicateAlert if an alert with the same ID already exists.
	Save(ctx context.Context, alert *entity.Alert) error

	// FindByID retrieves an alert by its unique identifier.
	// Returns nil, nil if not found.
	FindByID(ctx context.Context, id string) (*entity.Alert, error)

	// FindByFingerprint finds alerts matching the Alertmanager fingerprint.
	// Returns empty slice if none found.
	FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.Alert, error)

	// FindByExternalReference finds an alert by its external system reference.
	// System examples: "slack", "pagerduty", etc.
	// Returns nil, nil if not found.
	FindByExternalReference(ctx context.Context, system, referenceID string) (*entity.Alert, error)

	// Update modifies an existing alert.
	// Returns ErrAlertNotFound if the alert doesn't exist.
	Update(ctx context.Context, alert *entity.Alert) error

	// FindActive returns all currently active (non-resolved) alerts.
	FindActive(ctx context.Context) ([]*entity.Alert, error)

	// GetActiveAlerts returns active alerts, optionally filtered by severity.
	// If severity is empty, returns all active alerts.
	// Valid severity values: "critical", "warning", "info"
	GetActiveAlerts(ctx context.Context, severity string) ([]*entity.Alert, error)

	// FindFiring returns all firing alerts (active or acknowledged).
	FindFiring(ctx context.Context) ([]*entity.Alert, error)

	// Delete removes an alert by ID.
	// Returns ErrAlertNotFound if the alert doesn't exist.
	Delete(ctx context.Context, id string) error
}

// AckEventRepository stores acknowledgment events for audit trail.
type AckEventRepository interface {
	// Save persists a new ack event.
	Save(ctx context.Context, event *entity.AckEvent) error

	// FindByAlertID retrieves all ack events for an alert.
	// Returns empty slice if none found.
	FindByAlertID(ctx context.Context, alertID string) ([]*entity.AckEvent, error)

	// FindByID retrieves an ack event by its ID.
	// Returns nil, nil if not found.
	FindByID(ctx context.Context, id string) (*entity.AckEvent, error)

	// FindLatestByAlertID retrieves the most recent ack event for an alert.
	// Returns nil, nil if none found.
	FindLatestByAlertID(ctx context.Context, alertID string) (*entity.AckEvent, error)
}

// SilenceRepository stores silence/snooze rules.
type SilenceRepository interface {
	// Save persists a new silence.
	Save(ctx context.Context, silence *entity.SilenceMark) error

	// FindByID retrieves a silence by its ID.
	// Returns nil, nil if not found.
	FindByID(ctx context.Context, id string) (*entity.SilenceMark, error)

	// FindActive returns all currently active silences.
	FindActive(ctx context.Context) ([]*entity.SilenceMark, error)

	// FindByAlertID retrieves active silences for a specific alert.
	FindByAlertID(ctx context.Context, alertID string) ([]*entity.SilenceMark, error)

	// FindByInstance retrieves active silences for a specific instance.
	FindByInstance(ctx context.Context, instance string) ([]*entity.SilenceMark, error)

	// FindByFingerprint retrieves active silences for a specific fingerprint.
	FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.SilenceMark, error)

	// FindMatchingAlert returns all active silences that match the given alert.
	FindMatchingAlert(ctx context.Context, alert *entity.Alert) ([]*entity.SilenceMark, error)

	// Update modifies an existing silence.
	// Returns ErrSilenceNotFound if the silence doesn't exist.
	Update(ctx context.Context, silence *entity.SilenceMark) error

	// Delete removes a silence by ID.
	// Returns ErrSilenceNotFound if the silence doesn't exist.
	Delete(ctx context.Context, id string) error

	// DeleteExpired removes all expired silences.
	// Returns the number of deleted silences.
	DeleteExpired(ctx context.Context) (int, error)
}
