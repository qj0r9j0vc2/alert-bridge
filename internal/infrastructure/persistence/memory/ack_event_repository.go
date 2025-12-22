package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// AckEventRepository provides an in-memory implementation of repository.AckEventRepository.
// Thread-safe for concurrent access.
type AckEventRepository struct {
	mu         sync.RWMutex
	events     map[string]*entity.AckEvent // id -> event
	byAlertID  map[string][]string         // alertID -> event IDs
}

// NewAckEventRepository creates a new in-memory ack event repository.
func NewAckEventRepository() *AckEventRepository {
	return &AckEventRepository{
		events:    make(map[string]*entity.AckEvent),
		byAlertID: make(map[string][]string),
	}
}

// Save persists a new ack event.
func (r *AckEventRepository) Save(ctx context.Context, event *entity.AckEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a copy to prevent external mutations
	eventCopy := *event
	r.events[event.ID] = &eventCopy

	// Index by alert ID
	r.byAlertID[event.AlertID] = append(r.byAlertID[event.AlertID], event.ID)

	return nil
}

// FindByAlertID retrieves all ack events for an alert.
func (r *AckEventRepository) FindByAlertID(ctx context.Context, alertID string) ([]*entity.AckEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byAlertID[alertID]
	events := make([]*entity.AckEvent, 0, len(ids))
	for _, id := range ids {
		if event, ok := r.events[id]; ok {
			eventCopy := *event
			events = append(events, &eventCopy)
		}
	}

	// Sort by created time (oldest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})

	return events, nil
}

// FindByID retrieves an ack event by its ID.
func (r *AckEventRepository) FindByID(ctx context.Context, id string) (*entity.AckEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	event, ok := r.events[id]
	if !ok {
		return nil, nil
	}

	eventCopy := *event
	return &eventCopy, nil
}

// FindLatestByAlertID retrieves the most recent ack event for an alert.
func (r *AckEventRepository) FindLatestByAlertID(ctx context.Context, alertID string) (*entity.AckEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byAlertID[alertID]
	if len(ids) == 0 {
		return nil, nil
	}

	var latest *entity.AckEvent
	for _, id := range ids {
		if event, ok := r.events[id]; ok {
			if latest == nil || event.CreatedAt.After(latest.CreatedAt) {
				latest = event
			}
		}
	}

	if latest == nil {
		return nil, nil
	}

	eventCopy := *latest
	return &eventCopy, nil
}
