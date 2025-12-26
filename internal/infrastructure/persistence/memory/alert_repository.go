package memory

import (
	"context"
	"sync"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// AlertRepository provides an in-memory implementation of repository.AlertRepository.
// Thread-safe for concurrent access.
type AlertRepository struct {
	mu            sync.RWMutex
	alerts        map[string]*entity.Alert     // id -> alert
	byFingerprint map[string][]string          // fingerprint -> alert IDs
	byExternalRef map[string]map[string]string // system -> (referenceID -> alert ID)
}

// NewAlertRepository creates a new in-memory alert repository.
func NewAlertRepository() *AlertRepository {
	return &AlertRepository{
		alerts:        make(map[string]*entity.Alert),
		byFingerprint: make(map[string][]string),
		byExternalRef: make(map[string]map[string]string),
	}
}

// Save persists a new alert.
func (r *AlertRepository) Save(ctx context.Context, alert *entity.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.alerts[alert.ID]; exists {
		return entity.ErrDuplicateAlert
	}

	// Store a copy to prevent external mutations
	alertCopy := *alert
	r.alerts[alert.ID] = &alertCopy

	// Index by fingerprint
	r.byFingerprint[alert.Fingerprint] = append(r.byFingerprint[alert.Fingerprint], alert.ID)

	// Index by external references
	for system, refID := range alert.ExternalReferences {
		if refID != "" {
			if r.byExternalRef[system] == nil {
				r.byExternalRef[system] = make(map[string]string)
			}
			r.byExternalRef[system][refID] = alert.ID
		}
	}

	return nil
}

// FindByID retrieves an alert by its unique identifier.
func (r *AlertRepository) FindByID(ctx context.Context, id string) (*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	alert, ok := r.alerts[id]
	if !ok {
		return nil, nil
	}

	// Return a copy to prevent external mutations
	alertCopy := *alert
	return &alertCopy, nil
}

// FindByFingerprint finds alerts matching the Alertmanager fingerprint.
func (r *AlertRepository) FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byFingerprint[fingerprint]
	alerts := make([]*entity.Alert, 0, len(ids))
	for _, id := range ids {
		if alert, ok := r.alerts[id]; ok {
			alertCopy := *alert
			alerts = append(alerts, &alertCopy)
		}
	}
	return alerts, nil
}

// FindByExternalReference finds an alert by its external system reference.
func (r *AlertRepository) FindByExternalReference(ctx context.Context, system, referenceID string) (*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	systemMap, ok := r.byExternalRef[system]
	if !ok {
		return nil, nil
	}

	id, ok := systemMap[referenceID]
	if !ok {
		return nil, nil
	}

	alert, ok := r.alerts[id]
	if !ok {
		return nil, nil
	}

	alertCopy := *alert
	return &alertCopy, nil
}

// Update modifies an existing alert.
func (r *AlertRepository) Update(ctx context.Context, alert *entity.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.alerts[alert.ID]
	if !exists {
		return entity.ErrAlertNotFound
	}

	// Update external reference indexes if changed
	// Remove old references
	for system, oldRefID := range existing.ExternalReferences {
		newRefID := alert.GetExternalReference(system)
		if oldRefID != newRefID && oldRefID != "" {
			if r.byExternalRef[system] != nil {
				delete(r.byExternalRef[system], oldRefID)
			}
		}
	}

	// Add new references
	for system, newRefID := range alert.ExternalReferences {
		oldRefID := existing.GetExternalReference(system)
		if newRefID != oldRefID && newRefID != "" {
			if r.byExternalRef[system] == nil {
				r.byExternalRef[system] = make(map[string]string)
			}
			r.byExternalRef[system][newRefID] = alert.ID
		}
	}

	// Store updated copy
	alertCopy := *alert
	r.alerts[alert.ID] = &alertCopy

	return nil
}

// FindActive returns all currently active (non-resolved) alerts.
func (r *AlertRepository) FindActive(ctx context.Context) ([]*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var active []*entity.Alert
	for _, alert := range r.alerts {
		if alert.IsActive() {
			alertCopy := *alert
			active = append(active, &alertCopy)
		}
	}
	return active, nil
}

// FindFiring returns all firing alerts (active or acknowledged).
func (r *AlertRepository) FindFiring(ctx context.Context) ([]*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var firing []*entity.Alert
	for _, alert := range r.alerts {
		if alert.IsFiring() {
			alertCopy := *alert
			firing = append(firing, &alertCopy)
		}
	}
	return firing, nil
}

// GetActiveAlerts returns active alerts, optionally filtered by severity.
// Pass empty string for severity to get all active alerts.
func (r *AlertRepository) GetActiveAlerts(ctx context.Context, severity string) ([]*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var active []*entity.Alert
	for _, alert := range r.alerts {
		if alert.IsActive() {
			// Filter by severity if specified
			if severity != "" && string(alert.Severity) != severity {
				continue
			}
			alertCopy := *alert
			active = append(active, &alertCopy)
		}
	}
	return active, nil
}

// Delete removes an alert by ID.
func (r *AlertRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	alert, exists := r.alerts[id]
	if !exists {
		return entity.ErrAlertNotFound
	}

	// Remove from external reference indexes
	for system, refID := range alert.ExternalReferences {
		if refID != "" && r.byExternalRef[system] != nil {
			delete(r.byExternalRef[system], refID)
		}
	}

	// Remove from fingerprint index
	fps := r.byFingerprint[alert.Fingerprint]
	for i, fpID := range fps {
		if fpID == id {
			r.byFingerprint[alert.Fingerprint] = append(fps[:i], fps[i+1:]...)
			break
		}
	}

	delete(r.alerts, id)
	return nil
}
