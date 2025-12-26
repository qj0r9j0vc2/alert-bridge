package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// AlertRepository provides SQLite implementation of repository.AlertRepository.
type AlertRepository struct {
	db *DB
}

// NewAlertRepository creates a new SQLite-backed alert repository.
func NewAlertRepository(db *DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// Save persists a new alert.
// Returns ErrDuplicateAlert if an alert with the same ID already exists.
func (r *AlertRepository) Save(ctx context.Context, alert *entity.Alert) error {
	labels, err := marshalJSON(alert.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	annotations, err := marshalJSON(alert.Annotations)
	if err != nil {
		return fmt.Errorf("marshal annotations: %w", err)
	}

	externalRefs, err := marshalJSON(alert.ExternalReferences)
	if err != nil {
		return fmt.Errorf("marshal external references: %w", err)
	}

	_, err = r.db.getExecutor(ctx).ExecContext(ctx, `
		INSERT INTO alerts (
			id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			external_references,
			fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		alert.ID, alert.Fingerprint, alert.Name, alert.Instance, alert.Target,
		alert.Summary, alert.Description, string(alert.Severity), string(alert.State),
		labels, annotations,
		externalRefs,
		timeToString(alert.FiredAt),
		nullTime(alert.AckedAt), nullString(alert.AckedBy), nullTime(alert.ResolvedAt),
		timeToString(alert.CreatedAt), timeToString(alert.UpdatedAt),
	)

	if err != nil {
		if isUniqueConstraintError(err) {
			return entity.ErrDuplicateAlert
		}
		return fmt.Errorf("insert alert: %w", err)
	}

	return nil
}

// FindByID retrieves an alert by its unique identifier.
// Returns nil, nil if not found.
func (r *AlertRepository) FindByID(ctx context.Context, id string) (*entity.Alert, error) {
	row := r.db.getExecutor(ctx).QueryRowContext(ctx, `
		SELECT id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			external_references,
			fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
		FROM alerts WHERE id = ?
	`, id)

	return scanAlert(row)
}

// FindByFingerprint finds alerts matching the Alertmanager fingerprint.
// Returns empty slice if none found.
func (r *AlertRepository) FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.Alert, error) {
	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			external_references,
			fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
		FROM alerts WHERE fingerprint = ?
	`, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("query by fingerprint: %w", err)
	}
	defer rows.Close()

	return scanAlerts(rows)
}

// FindByExternalReference finds an alert by its external integration reference.
// Returns nil, nil if not found.
func (r *AlertRepository) FindByExternalReference(ctx context.Context, system, referenceID string) (*entity.Alert, error) {
	row := r.db.getExecutor(ctx).QueryRowContext(ctx, `
		SELECT id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			external_references,
			fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
		FROM alerts
		WHERE json_extract(external_references, '$.' || ?) = ?
	`, system, referenceID)

	return scanAlert(row)
}

// Update modifies an existing alert.
// Returns ErrAlertNotFound if the alert doesn't exist.
func (r *AlertRepository) Update(ctx context.Context, alert *entity.Alert) error {
	labels, err := marshalJSON(alert.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	annotations, err := marshalJSON(alert.Annotations)
	if err != nil {
		return fmt.Errorf("marshal annotations: %w", err)
	}

	externalRefs, err := marshalJSON(alert.ExternalReferences)
	if err != nil {
		return fmt.Errorf("marshal external references: %w", err)
	}

	result, err := r.db.getExecutor(ctx).ExecContext(ctx, `
		UPDATE alerts SET
			fingerprint = ?, name = ?, instance = ?, target = ?, summary = ?, description = ?,
			severity = ?, state = ?, labels = ?, annotations = ?,
			external_references = ?,
			fired_at = ?, acked_at = ?, acked_by = ?, resolved_at = ?, updated_at = ?
		WHERE id = ?
	`,
		alert.Fingerprint, alert.Name, alert.Instance, alert.Target,
		alert.Summary, alert.Description, string(alert.Severity), string(alert.State),
		labels, annotations,
		externalRefs,
		timeToString(alert.FiredAt),
		nullTime(alert.AckedAt), nullString(alert.AckedBy), nullTime(alert.ResolvedAt),
		timeToString(alert.UpdatedAt),
		alert.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return entity.ErrAlertNotFound
	}

	return nil
}

// FindActive returns all currently active (non-resolved) alerts.
func (r *AlertRepository) FindActive(ctx context.Context) ([]*entity.Alert, error) {
	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			external_references,
			fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
		FROM alerts WHERE state != 'resolved'
	`)
	if err != nil {
		return nil, fmt.Errorf("query active alerts: %w", err)
	}
	defer rows.Close()

	return scanAlerts(rows)
}

// FindFiring returns all firing alerts (active or acknowledged).
func (r *AlertRepository) FindFiring(ctx context.Context) ([]*entity.Alert, error) {
	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, fingerprint, name, instance, target, summary, description,
			severity, state, labels, annotations,
			external_references,
			fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
		FROM alerts WHERE state IN ('active', 'acknowledged')
	`)
	if err != nil {
		return nil, fmt.Errorf("query firing alerts: %w", err)
	}
	defer rows.Close()

	return scanAlerts(rows)
}

// GetActiveAlerts returns active alerts, optionally filtered by severity.
// Pass empty string for severity to get all active alerts.
func (r *AlertRepository) GetActiveAlerts(ctx context.Context, severity string) ([]*entity.Alert, error) {
	var query string
	var args []interface{}

	if severity == "" {
		query = `
			SELECT id, fingerprint, name, instance, target, summary, description,
				severity, state, labels, annotations,
				external_references,
				fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
			FROM alerts WHERE state != 'resolved'
			ORDER BY fired_at DESC
		`
	} else {
		query = `
			SELECT id, fingerprint, name, instance, target, summary, description,
				severity, state, labels, annotations,
				external_references,
				fired_at, acked_at, acked_by, resolved_at, created_at, updated_at
			FROM alerts WHERE state != 'resolved' AND severity = ?
			ORDER BY fired_at DESC
		`
		args = append(args, severity)
	}

	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query active alerts: %w", err)
	}
	defer rows.Close()

	return scanAlerts(rows)
}

// Delete removes an alert by ID.
// Returns ErrAlertNotFound if the alert doesn't exist.
func (r *AlertRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.getExecutor(ctx).ExecContext(ctx, `DELETE FROM alerts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return entity.ErrAlertNotFound
	}

	return nil
}

// scanAlert scans a single row into an Alert entity.
func scanAlert(row *sql.Row) (*entity.Alert, error) {
	var (
		alert        entity.Alert
		severity     string
		state        string
		labels       string
		annotations  string
		externalRefs string
		firedAt      string
		ackedAt      sql.NullString
		ackedBy      sql.NullString
		resolvedAt   sql.NullString
		createdAt    string
		updatedAt    string
	)

	err := row.Scan(
		&alert.ID, &alert.Fingerprint, &alert.Name, &alert.Instance, &alert.Target,
		&alert.Summary, &alert.Description, &severity, &state, &labels, &annotations,
		&externalRefs, &firedAt, &ackedAt, &ackedBy, &resolvedAt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan alert: %w", err)
	}

	// Convert string fields
	alert.Severity = entity.AlertSeverity(severity)
	alert.State = entity.AlertState(state)
	alert.AckedBy = stringFromNull(ackedBy)

	// Parse JSON fields
	alert.Labels, _ = unmarshalJSON(labels)
	alert.Annotations, _ = unmarshalJSON(annotations)
	alert.ExternalReferences, _ = unmarshalJSON(externalRefs)

	// Parse timestamps
	alert.FiredAt, _ = parseTime(firedAt)
	alert.AckedAt = scanNullTime(ackedAt)
	alert.ResolvedAt = scanNullTime(resolvedAt)
	alert.CreatedAt, _ = parseTime(createdAt)
	alert.UpdatedAt, _ = parseTime(updatedAt)

	return &alert, nil
}

// scanAlerts scans multiple rows into Alert entities.
func scanAlerts(rows *sql.Rows) ([]*entity.Alert, error) {
	var alerts []*entity.Alert

	for rows.Next() {
		var (
			alert        entity.Alert
			severity     string
			state        string
			labels       string
			annotations  string
			externalRefs string
			firedAt      string
			ackedAt      sql.NullString
			ackedBy      sql.NullString
			resolvedAt   sql.NullString
			createdAt    string
			updatedAt    string
		)

		err := rows.Scan(
			&alert.ID, &alert.Fingerprint, &alert.Name, &alert.Instance, &alert.Target,
			&alert.Summary, &alert.Description, &severity, &state, &labels, &annotations,
			&externalRefs, &firedAt, &ackedAt, &ackedBy, &resolvedAt, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan alert row: %w", err)
		}

		// Convert string fields
		alert.Severity = entity.AlertSeverity(severity)
		alert.State = entity.AlertState(state)
		alert.AckedBy = stringFromNull(ackedBy)

		// Parse JSON fields
		alert.Labels, _ = unmarshalJSON(labels)
		alert.Annotations, _ = unmarshalJSON(annotations)
		alert.ExternalReferences, _ = unmarshalJSON(externalRefs)

		// Parse timestamps
		alert.FiredAt, _ = parseTime(firedAt)
		alert.AckedAt = scanNullTime(ackedAt)
		alert.ResolvedAt = scanNullTime(resolvedAt)
		alert.CreatedAt, _ = parseTime(createdAt)
		alert.UpdatedAt, _ = parseTime(updatedAt)

		alerts = append(alerts, &alert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Return empty slice instead of nil
	if alerts == nil {
		alerts = []*entity.Alert{}
	}

	return alerts, nil
}
