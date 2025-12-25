package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
)

// AlertRepository provides MySQL implementation of repository.AlertRepository.
type AlertRepository struct {
	db *DB
}

// NewAlertRepository creates a new MySQL-backed alert repository.
func NewAlertRepository(db *DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// Save persists a new alert.
// Returns ErrAlreadyExists if an alert with the same ID already exists.
func (r *AlertRepository) Save(ctx context.Context, alert *entity.Alert) error {
	// Serialize JSON fields
	labelsJSON, err := marshalJSON(alert.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	annotationsJSON, err := marshalJSON(alert.Annotations)
	if err != nil {
		return fmt.Errorf("marshaling annotations: %w", err)
	}

	query := `
		INSERT INTO alerts (
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			slack_message_id, pagerduty_incident_id,
			fired_at, acked_at, acked_by, resolved_at,
			version, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?,
			?, ?, ?, ?,
			1, ?, ?
		)
	`

	_, err = r.db.Primary().ExecContext(ctx, query,
		alert.ID,
		alert.Fingerprint,
		alert.Name,
		alert.Instance,
		alert.Target,
		alert.Summary,
		alert.Description,
		string(alert.Severity),
		string(alert.State),
		labelsJSON,
		annotationsJSON,
		nullString(alert.SlackMessageID),
		nullString(alert.PagerDutyIncidentID),
		timeToTimestamp(alert.FiredAt),
		nullTime(alert.AckedAt),
		nullString(alert.AckedBy),
		nullTime(alert.ResolvedAt),
		timeToTimestamp(alert.CreatedAt),
		timeToTimestamp(alert.UpdatedAt),
	)

	if err != nil {
		if isDuplicateError(err) {
			return repository.ErrAlreadyExists
		}
		return fmt.Errorf("inserting alert: %w", err)
	}

	return nil
}

// FindByID retrieves an alert by its unique identifier.
// Returns nil, nil if not found.
func (r *AlertRepository) FindByID(ctx context.Context, id string) (*entity.Alert, error) {
	query := `
		SELECT
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			slack_message_id, pagerduty_incident_id,
			fired_at, acked_at, acked_by, resolved_at,
			version, created_at, updated_at
		FROM alerts
		WHERE id = ?
	`

	var alert entity.Alert
	var labelsJSON, annotationsJSON string
	var slackMessageID, pagerDutyIncidentID, ackedBy sql.NullString
	var ackedAt, resolvedAt sql.NullTime
	var version int

	err := r.db.Replica().QueryRowContext(ctx, query, id).Scan(
		&alert.ID,
		&alert.Fingerprint,
		&alert.Name,
		&alert.Instance,
		&alert.Target,
		&alert.Summary,
		&alert.Description,
		&alert.Severity,
		&alert.State,
		&labelsJSON,
		&annotationsJSON,
		&slackMessageID,
		&pagerDutyIncidentID,
		&alert.FiredAt,
		&ackedAt,
		&ackedBy,
		&resolvedAt,
		&version,
		&alert.CreatedAt,
		&alert.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying alert: %w", err)
	}

	// Deserialize JSON fields
	if err := unmarshalJSON(labelsJSON, &alert.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}
	if err := unmarshalJSON(annotationsJSON, &alert.Annotations); err != nil {
		return nil, fmt.Errorf("unmarshaling annotations: %w", err)
	}

	// Set nullable fields
	alert.SlackMessageID = stringValue(slackMessageID)
	alert.PagerDutyIncidentID = stringValue(pagerDutyIncidentID)
	alert.AckedBy = stringValue(ackedBy)
	alert.AckedAt = timePtr(ackedAt)
	alert.ResolvedAt = timePtr(resolvedAt)

	return &alert, nil
}

// FindByFingerprint finds alerts matching the Alertmanager fingerprint.
// Returns empty slice if none found.
func (r *AlertRepository) FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.Alert, error) {
	query := `
		SELECT
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			slack_message_id, pagerduty_incident_id,
			fired_at, acked_at, acked_by, resolved_at,
			version, created_at, updated_at
		FROM alerts
		WHERE fingerprint = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("querying alerts by fingerprint: %w", err)
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// FindBySlackMessageID finds an alert by its Slack message reference.
// Returns nil, nil if not found.
func (r *AlertRepository) FindBySlackMessageID(ctx context.Context, messageID string) (*entity.Alert, error) {
	query := `
		SELECT
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			slack_message_id, pagerduty_incident_id,
			fired_at, acked_at, acked_by, resolved_at,
			version, created_at, updated_at
		FROM alerts
		WHERE slack_message_id = ?
	`

	var alert entity.Alert
	var labelsJSON, annotationsJSON string
	var slackMessageID, pagerDutyIncidentID, ackedBy sql.NullString
	var ackedAt, resolvedAt sql.NullTime
	var version int

	err := r.db.Replica().QueryRowContext(ctx, query, messageID).Scan(
		&alert.ID,
		&alert.Fingerprint,
		&alert.Name,
		&alert.Instance,
		&alert.Target,
		&alert.Summary,
		&alert.Description,
		&alert.Severity,
		&alert.State,
		&labelsJSON,
		&annotationsJSON,
		&slackMessageID,
		&pagerDutyIncidentID,
		&alert.FiredAt,
		&ackedAt,
		&ackedBy,
		&resolvedAt,
		&version,
		&alert.CreatedAt,
		&alert.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying alert by slack message ID: %w", err)
	}

	// Deserialize JSON fields
	if err := unmarshalJSON(labelsJSON, &alert.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}
	if err := unmarshalJSON(annotationsJSON, &alert.Annotations); err != nil {
		return nil, fmt.Errorf("unmarshaling annotations: %w", err)
	}

	// Set nullable fields
	alert.SlackMessageID = stringValue(slackMessageID)
	alert.PagerDutyIncidentID = stringValue(pagerDutyIncidentID)
	alert.AckedBy = stringValue(ackedBy)
	alert.AckedAt = timePtr(ackedAt)
	alert.ResolvedAt = timePtr(resolvedAt)

	return &alert, nil
}

// FindByPagerDutyIncidentID finds an alert by its PagerDuty incident reference.
// Returns nil, nil if not found.
func (r *AlertRepository) FindByPagerDutyIncidentID(ctx context.Context, incidentID string) (*entity.Alert, error) {
	query := `
		SELECT
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			slack_message_id, pagerduty_incident_id,
			fired_at, acked_at, acked_by, resolved_at,
			version, created_at, updated_at
		FROM alerts
		WHERE pagerduty_incident_id = ?
	`

	var alert entity.Alert
	var labelsJSON, annotationsJSON string
	var slackMessageID, pagerDutyIncidentID, ackedBy sql.NullString
	var ackedAt, resolvedAt sql.NullTime
	var version int

	err := r.db.Replica().QueryRowContext(ctx, query, incidentID).Scan(
		&alert.ID,
		&alert.Fingerprint,
		&alert.Name,
		&alert.Instance,
		&alert.Target,
		&alert.Summary,
		&alert.Description,
		&alert.Severity,
		&alert.State,
		&labelsJSON,
		&annotationsJSON,
		&slackMessageID,
		&pagerDutyIncidentID,
		&alert.FiredAt,
		&ackedAt,
		&ackedBy,
		&resolvedAt,
		&version,
		&alert.CreatedAt,
		&alert.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying alert by pagerduty incident ID: %w", err)
	}

	// Deserialize JSON fields
	if err := unmarshalJSON(labelsJSON, &alert.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}
	if err := unmarshalJSON(annotationsJSON, &alert.Annotations); err != nil {
		return nil, fmt.Errorf("unmarshaling annotations: %w", err)
	}

	// Set nullable fields
	alert.SlackMessageID = stringValue(slackMessageID)
	alert.PagerDutyIncidentID = stringValue(pagerDutyIncidentID)
	alert.AckedBy = stringValue(ackedBy)
	alert.AckedAt = timePtr(ackedAt)
	alert.ResolvedAt = timePtr(resolvedAt)

	return &alert, nil
}

// Update modifies an existing alert with optimistic locking.
// Returns ErrNotFound if the alert doesn't exist.
// Returns ErrConcurrentUpdate if the alert was modified by another instance.
func (r *AlertRepository) Update(ctx context.Context, alert *entity.Alert) error {
	// First, get the current version
	var currentVersion int
	versionQuery := `SELECT version FROM alerts WHERE id = ?`
	err := r.db.Primary().QueryRowContext(ctx, versionQuery, alert.ID).Scan(&currentVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return fmt.Errorf("checking alert version: %w", err)
	}

	// Serialize JSON fields
	labelsJSON, err := marshalJSON(alert.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	annotationsJSON, err := marshalJSON(alert.Annotations)
	if err != nil {
		return fmt.Errorf("marshaling annotations: %w", err)
	}

	// Update with optimistic locking (increment version)
	query := `
		UPDATE alerts SET
			fingerprint = ?,
			name = ?,
			instance = ?,
			target = ?,
			summary = ?,
			description = ?,
			severity = ?,
			state = ?,
			labels = ?,
			annotations = ?,
			slack_message_id = ?,
			pagerduty_incident_id = ?,
			fired_at = ?,
			acked_at = ?,
			acked_by = ?,
			resolved_at = ?,
			updated_at = ?,
			version = version + 1
		WHERE id = ? AND version = ?
	`

	result, err := r.db.Primary().ExecContext(ctx, query,
		alert.Fingerprint,
		alert.Name,
		alert.Instance,
		alert.Target,
		alert.Summary,
		alert.Description,
		string(alert.Severity),
		string(alert.State),
		labelsJSON,
		annotationsJSON,
		nullString(alert.SlackMessageID),
		nullString(alert.PagerDutyIncidentID),
		timeToTimestamp(alert.FiredAt),
		nullTime(alert.AckedAt),
		nullString(alert.AckedBy),
		nullTime(alert.ResolvedAt),
		timeToTimestamp(alert.UpdatedAt),
		alert.ID,
		currentVersion,
	)

	if err != nil {
		return fmt.Errorf("updating alert: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Either the alert doesn't exist or version mismatch (concurrent update)
		// Check if alert exists
		var exists bool
		existsQuery := `SELECT COUNT(*) > 0 FROM alerts WHERE id = ?`
		err := r.db.Primary().QueryRowContext(ctx, existsQuery, alert.ID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking alert existence: %w", err)
		}

		if !exists {
			return repository.ErrNotFound
		}

		// Alert exists but version mismatch - concurrent update detected
		return repository.ErrConcurrentUpdate
	}

	return nil
}

// FindActive returns all currently active (non-resolved) alerts.
func (r *AlertRepository) FindActive(ctx context.Context) ([]*entity.Alert, error) {
	query := `
		SELECT
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			slack_message_id, pagerduty_incident_id,
			fired_at, acked_at, acked_by, resolved_at,
			version, created_at, updated_at
		FROM alerts
		WHERE state != 'resolved'
		ORDER BY fired_at DESC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying active alerts: %w", err)
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// FindFiring returns all firing alerts (active or acknowledged).
func (r *AlertRepository) FindFiring(ctx context.Context) ([]*entity.Alert, error) {
	query := `
		SELECT
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			slack_message_id, pagerduty_incident_id,
			fired_at, acked_at, acked_by, resolved_at,
			version, created_at, updated_at
		FROM alerts
		WHERE state IN ('active', 'acknowledged')
		ORDER BY fired_at DESC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying firing alerts: %w", err)
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// Delete removes an alert by ID.
// Returns ErrNotFound if the alert doesn't exist.
func (r *AlertRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM alerts WHERE id = ?`

	result, err := r.db.Primary().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting alert: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}

// scanAlerts is a helper function to scan multiple alerts from query results.
func (r *AlertRepository) scanAlerts(rows *sql.Rows) ([]*entity.Alert, error) {
	alerts := make([]*entity.Alert, 0)

	for rows.Next() {
		var alert entity.Alert
		var labelsJSON, annotationsJSON string
		var slackMessageID, pagerDutyIncidentID, ackedBy sql.NullString
		var ackedAt, resolvedAt sql.NullTime
		var version int

		err := rows.Scan(
			&alert.ID,
			&alert.Fingerprint,
			&alert.Name,
			&alert.Instance,
			&alert.Target,
			&alert.Summary,
			&alert.Description,
			&alert.Severity,
			&alert.State,
			&labelsJSON,
			&annotationsJSON,
			&slackMessageID,
			&pagerDutyIncidentID,
			&alert.FiredAt,
			&ackedAt,
			&ackedBy,
			&resolvedAt,
			&version,
			&alert.CreatedAt,
			&alert.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scanning alert row: %w", err)
		}

		// Deserialize JSON fields
		if err := unmarshalJSON(labelsJSON, &alert.Labels); err != nil {
			return nil, fmt.Errorf("unmarshaling labels: %w", err)
		}
		if err := unmarshalJSON(annotationsJSON, &alert.Annotations); err != nil {
			return nil, fmt.Errorf("unmarshaling annotations: %w", err)
		}

		// Set nullable fields
		alert.SlackMessageID = stringValue(slackMessageID)
		alert.PagerDutyIncidentID = stringValue(pagerDutyIncidentID)
		alert.AckedBy = stringValue(ackedBy)
		alert.AckedAt = timePtr(ackedAt)
		alert.ResolvedAt = timePtr(resolvedAt)

		alerts = append(alerts, &alert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating alert rows: %w", err)
	}

	return alerts, nil
}
