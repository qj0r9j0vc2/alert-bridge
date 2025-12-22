package memory

import (
	"context"
	"sync"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// SilenceRepository provides an in-memory implementation of repository.SilenceRepository.
// Thread-safe for concurrent access.
type SilenceRepository struct {
	mu            sync.RWMutex
	silences      map[string]*entity.SilenceMark // id -> silence
	byAlertID     map[string][]string            // alertID -> silence IDs
	byInstance    map[string][]string            // instance -> silence IDs
	byFingerprint map[string][]string            // fingerprint -> silence IDs
}

// NewSilenceRepository creates a new in-memory silence repository.
func NewSilenceRepository() *SilenceRepository {
	return &SilenceRepository{
		silences:      make(map[string]*entity.SilenceMark),
		byAlertID:     make(map[string][]string),
		byInstance:    make(map[string][]string),
		byFingerprint: make(map[string][]string),
	}
}

// Save persists a new silence.
func (r *SilenceRepository) Save(ctx context.Context, silence *entity.SilenceMark) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a copy to prevent external mutations
	silenceCopy := *silence
	if silence.Labels != nil {
		silenceCopy.Labels = make(map[string]string)
		for k, v := range silence.Labels {
			silenceCopy.Labels[k] = v
		}
	}
	r.silences[silence.ID] = &silenceCopy

	// Index by alert ID if set
	if silence.AlertID != "" {
		r.byAlertID[silence.AlertID] = append(r.byAlertID[silence.AlertID], silence.ID)
	}

	// Index by instance if set
	if silence.Instance != "" {
		r.byInstance[silence.Instance] = append(r.byInstance[silence.Instance], silence.ID)
	}

	// Index by fingerprint if set
	if silence.Fingerprint != "" {
		r.byFingerprint[silence.Fingerprint] = append(r.byFingerprint[silence.Fingerprint], silence.ID)
	}

	return nil
}

// FindByID retrieves a silence by its ID.
func (r *SilenceRepository) FindByID(ctx context.Context, id string) (*entity.SilenceMark, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	silence, ok := r.silences[id]
	if !ok {
		return nil, nil
	}

	return r.copySilence(silence), nil
}

// FindActive returns all currently active silences.
func (r *SilenceRepository) FindActive(ctx context.Context) ([]*entity.SilenceMark, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var active []*entity.SilenceMark
	for _, silence := range r.silences {
		if silence.IsActive() {
			active = append(active, r.copySilence(silence))
		}
	}
	return active, nil
}

// FindByAlertID retrieves active silences for a specific alert.
func (r *SilenceRepository) FindByAlertID(ctx context.Context, alertID string) ([]*entity.SilenceMark, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byAlertID[alertID]
	var active []*entity.SilenceMark
	for _, id := range ids {
		if silence, ok := r.silences[id]; ok && silence.IsActive() {
			active = append(active, r.copySilence(silence))
		}
	}
	return active, nil
}

// FindByInstance retrieves active silences for a specific instance.
func (r *SilenceRepository) FindByInstance(ctx context.Context, instance string) ([]*entity.SilenceMark, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byInstance[instance]
	var active []*entity.SilenceMark
	for _, id := range ids {
		if silence, ok := r.silences[id]; ok && silence.IsActive() {
			active = append(active, r.copySilence(silence))
		}
	}
	return active, nil
}

// FindByFingerprint retrieves active silences for a specific fingerprint.
func (r *SilenceRepository) FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.SilenceMark, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byFingerprint[fingerprint]
	var active []*entity.SilenceMark
	for _, id := range ids {
		if silence, ok := r.silences[id]; ok && silence.IsActive() {
			active = append(active, r.copySilence(silence))
		}
	}
	return active, nil
}

// FindMatchingAlert returns all active silences that match the given alert.
func (r *SilenceRepository) FindMatchingAlert(ctx context.Context, alert *entity.Alert) ([]*entity.SilenceMark, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*entity.SilenceMark
	seen := make(map[string]bool)

	// Check silences by alert ID
	for _, id := range r.byAlertID[alert.ID] {
		if silence, ok := r.silences[id]; ok && silence.MatchesAlert(alert) {
			if !seen[id] {
				matches = append(matches, r.copySilence(silence))
				seen[id] = true
			}
		}
	}

	// Check silences by fingerprint
	for _, id := range r.byFingerprint[alert.Fingerprint] {
		if silence, ok := r.silences[id]; ok && silence.MatchesAlert(alert) {
			if !seen[id] {
				matches = append(matches, r.copySilence(silence))
				seen[id] = true
			}
		}
	}

	// Check silences by instance
	for _, id := range r.byInstance[alert.Instance] {
		if silence, ok := r.silences[id]; ok && silence.MatchesAlert(alert) {
			if !seen[id] {
				matches = append(matches, r.copySilence(silence))
				seen[id] = true
			}
		}
	}

	// Check label-only silences (no specific alert/fingerprint/instance)
	for id, silence := range r.silences {
		if silence.AlertID == "" && silence.Fingerprint == "" && silence.Instance == "" {
			if silence.MatchesAlert(alert) && !seen[id] {
				matches = append(matches, r.copySilence(silence))
				seen[id] = true
			}
		}
	}

	return matches, nil
}

// Update modifies an existing silence.
func (r *SilenceRepository) Update(ctx context.Context, silence *entity.SilenceMark) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.silences[silence.ID]; !exists {
		return entity.ErrSilenceNotFound
	}

	silenceCopy := *silence
	if silence.Labels != nil {
		silenceCopy.Labels = make(map[string]string)
		for k, v := range silence.Labels {
			silenceCopy.Labels[k] = v
		}
	}
	r.silences[silence.ID] = &silenceCopy

	return nil
}

// Delete removes a silence by ID.
func (r *SilenceRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	silence, exists := r.silences[id]
	if !exists {
		return entity.ErrSilenceNotFound
	}

	// Remove from indexes
	r.removeFromIndex(r.byAlertID, silence.AlertID, id)
	r.removeFromIndex(r.byInstance, silence.Instance, id)
	r.removeFromIndex(r.byFingerprint, silence.Fingerprint, id)

	delete(r.silences, id)
	return nil
}

// DeleteExpired removes all expired silences.
func (r *SilenceRepository) DeleteExpired(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var expiredIDs []string
	for id, silence := range r.silences {
		if silence.IsExpired() {
			expiredIDs = append(expiredIDs, id)
		}
	}

	for _, id := range expiredIDs {
		silence := r.silences[id]
		r.removeFromIndex(r.byAlertID, silence.AlertID, id)
		r.removeFromIndex(r.byInstance, silence.Instance, id)
		r.removeFromIndex(r.byFingerprint, silence.Fingerprint, id)
		delete(r.silences, id)
	}

	return len(expiredIDs), nil
}

// copySilence creates a deep copy of a silence.
func (r *SilenceRepository) copySilence(silence *entity.SilenceMark) *entity.SilenceMark {
	silenceCopy := *silence
	if silence.Labels != nil {
		silenceCopy.Labels = make(map[string]string)
		for k, v := range silence.Labels {
			silenceCopy.Labels[k] = v
		}
	}
	return &silenceCopy
}

// removeFromIndex removes an ID from a slice index.
func (r *SilenceRepository) removeFromIndex(index map[string][]string, key, id string) {
	if key == "" {
		return
	}
	ids := index[key]
	for i, indexID := range ids {
		if indexID == id {
			index[key] = append(ids[:i], ids[i+1:]...)
			return
		}
	}
}
