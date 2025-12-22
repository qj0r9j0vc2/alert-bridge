package memory

import (
	"context"
	"sync"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// AlertRepository provides an in-memory implementation of repository.AlertRepository.
// Thread-safe for concurrent access.
type AlertRepository struct {
	mu                    sync.RWMutex
	alerts                map[string]*entity.Alert // id -> alert
	byFingerprint         map[string][]string      // fingerprint -> alert IDs
	bySlackMessageID      map[string]string        // slack message ID -> alert ID
	byPagerDutyIncidentID map[string]string        // pagerduty incident ID -> alert ID
}

// NewAlertRepository creates a new in-memory alert repository.
func NewAlertRepository() *AlertRepository {
	return &AlertRepository{
		alerts:                make(map[string]*entity.Alert),
		byFingerprint:         make(map[string][]string),
		bySlackMessageID:      make(map[string]string),
		byPagerDutyIncidentID: make(map[string]string),
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

	// Index by Slack message ID if set
	if alert.SlackMessageID != "" {
		r.bySlackMessageID[alert.SlackMessageID] = alert.ID
	}

	// Index by PagerDuty incident ID if set
	if alert.PagerDutyIncidentID != "" {
		r.byPagerDutyIncidentID[alert.PagerDutyIncidentID] = alert.ID
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

// FindBySlackMessageID finds an alert by its Slack message reference.
func (r *AlertRepository) FindBySlackMessageID(ctx context.Context, messageID string) (*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.bySlackMessageID[messageID]
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

// FindByPagerDutyIncidentID finds an alert by its PagerDuty incident reference.
func (r *AlertRepository) FindByPagerDutyIncidentID(ctx context.Context, incidentID string) (*entity.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byPagerDutyIncidentID[incidentID]
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

	// Update secondary indexes if changed
	if existing.SlackMessageID != alert.SlackMessageID {
		if existing.SlackMessageID != "" {
			delete(r.bySlackMessageID, existing.SlackMessageID)
		}
		if alert.SlackMessageID != "" {
			r.bySlackMessageID[alert.SlackMessageID] = alert.ID
		}
	}

	if existing.PagerDutyIncidentID != alert.PagerDutyIncidentID {
		if existing.PagerDutyIncidentID != "" {
			delete(r.byPagerDutyIncidentID, existing.PagerDutyIncidentID)
		}
		if alert.PagerDutyIncidentID != "" {
			r.byPagerDutyIncidentID[alert.PagerDutyIncidentID] = alert.ID
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

// Delete removes an alert by ID.
func (r *AlertRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	alert, exists := r.alerts[id]
	if !exists {
		return entity.ErrAlertNotFound
	}

	// Remove from secondary indexes
	if alert.SlackMessageID != "" {
		delete(r.bySlackMessageID, alert.SlackMessageID)
	}
	if alert.PagerDutyIncidentID != "" {
		delete(r.byPagerDutyIncidentID, alert.PagerDutyIncidentID)
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
