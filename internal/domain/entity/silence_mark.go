package entity

import (
	"time"

	"github.com/google/uuid"
)

// SilenceMark represents a silence/snooze rule that suppresses alerts for a period.
type SilenceMark struct {
	// ID is the unique identifier for this silence.
	ID string

	// AlertID references a specific alert to silence (optional).
	// If empty, Instance-level silencing is used.
	AlertID string

	// Instance silences all alerts from this instance (optional).
	// Used when AlertID is empty for broader silencing.
	Instance string

	// Fingerprint silences alerts matching this Alertmanager fingerprint (optional).
	Fingerprint string

	// Labels matches alerts with specific labels (optional).
	// Supports partial matching - alert must have all specified labels.
	Labels map[string]string

	// StartAt is when the silence starts.
	StartAt time.Time

	// EndAt is when the silence expires.
	EndAt time.Time

	// CreatedBy identifies who created the silence.
	CreatedBy string

	// CreatedByEmail is the email of the creator for cross-platform correlation.
	CreatedByEmail string

	// Reason explains why the silence was created.
	Reason string

	// Source indicates where the silence was created (slack, pagerduty, api).
	Source AckSource

	// CreatedAt is when this record was created.
	CreatedAt time.Time
}

// NewSilenceMark creates a new silence with the given duration.
func NewSilenceMark(duration time.Duration, createdBy, createdByEmail string, source AckSource) (*SilenceMark, error) {
	if duration <= 0 {
		return nil, ErrInvalidSilenceDuration
	}

	now := time.Now().UTC()
	return &SilenceMark{
		ID:             uuid.New().String(),
		Labels:         make(map[string]string),
		StartAt:        now,
		EndAt:          now.Add(duration),
		CreatedBy:      createdBy,
		CreatedByEmail: createdByEmail,
		Source:         source,
		CreatedAt:      now,
	}, nil
}

// ForAlert sets the silence to target a specific alert.
func (s *SilenceMark) ForAlert(alertID string) *SilenceMark {
	s.AlertID = alertID
	return s
}

// ForInstance sets the silence to target all alerts from an instance.
func (s *SilenceMark) ForInstance(instance string) *SilenceMark {
	s.Instance = instance
	return s
}

// ForFingerprint sets the silence to target alerts with a specific fingerprint.
func (s *SilenceMark) ForFingerprint(fingerprint string) *SilenceMark {
	s.Fingerprint = fingerprint
	return s
}

// WithLabel adds a label matcher to the silence.
func (s *SilenceMark) WithLabel(key, value string) *SilenceMark {
	if s.Labels == nil {
		s.Labels = make(map[string]string)
	}
	s.Labels[key] = value
	return s
}

// WithMatchers adds multiple label matchers to the silence.
func (s *SilenceMark) WithMatchers(matchers map[string]string) *SilenceMark {
	if s.Labels == nil {
		s.Labels = make(map[string]string)
	}
	for key, value := range matchers {
		s.Labels[key] = value
	}
	return s
}

// WithReason sets the reason for the silence.
func (s *SilenceMark) WithReason(reason string) *SilenceMark {
	s.Reason = reason
	return s
}

// IsActive returns true if the silence is currently active.
func (s *SilenceMark) IsActive() bool {
	now := time.Now().UTC()
	return now.After(s.StartAt) && now.Before(s.EndAt)
}

// IsExpired returns true if the silence has expired.
func (s *SilenceMark) IsExpired() bool {
	return time.Now().UTC().After(s.EndAt)
}

// IsPending returns true if the silence hasn't started yet.
func (s *SilenceMark) IsPending() bool {
	return time.Now().UTC().Before(s.StartAt)
}

// RemainingDuration returns how much time is left on the silence.
// Returns 0 if the silence has expired.
func (s *SilenceMark) RemainingDuration() time.Duration {
	remaining := time.Until(s.EndAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Cancel immediately expires the silence.
func (s *SilenceMark) Cancel() {
	s.EndAt = time.Now().UTC()
}

// Extend extends the silence by the given duration.
func (s *SilenceMark) Extend(duration time.Duration) error {
	if duration <= 0 {
		return ErrInvalidSilenceDuration
	}
	s.EndAt = s.EndAt.Add(duration)
	return nil
}

// MatchesAlert checks if this silence applies to the given alert.
func (s *SilenceMark) MatchesAlert(alert *Alert) bool {
	// Check if silence is active
	if !s.IsActive() {
		return false
	}

	// Check specific alert ID match
	if s.AlertID != "" && s.AlertID == alert.ID {
		return true
	}

	// Check fingerprint match
	if s.Fingerprint != "" && s.Fingerprint == alert.Fingerprint {
		return true
	}

	// Check instance match
	if s.Instance != "" && s.Instance == alert.Instance {
		// If labels are specified, all must match
		if len(s.Labels) > 0 {
			return s.matchesLabels(alert.Labels)
		}
		return true
	}

	// Check label-only match (instance not specified)
	if s.AlertID == "" && s.Fingerprint == "" && s.Instance == "" && len(s.Labels) > 0 {
		return s.matchesLabels(alert.Labels)
	}

	return false
}

// matchesLabels checks if all silence labels are present in the alert labels.
func (s *SilenceMark) matchesLabels(alertLabels map[string]string) bool {
	for key, value := range s.Labels {
		if alertLabels[key] != value {
			return false
		}
	}
	return true
}
