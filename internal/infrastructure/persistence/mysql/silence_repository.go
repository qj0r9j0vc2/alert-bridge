package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
)

// SilenceRepository provides MySQL implementation of repository.SilenceRepository.
type SilenceRepository struct {
	db *DB
}

// NewSilenceRepository creates a new MySQL-backed silence repository.
func NewSilenceRepository(db *DB) *SilenceRepository {
	return &SilenceRepository{db: db}
}

// Save persists a new silence.
func (r *SilenceRepository) Save(ctx context.Context, silence *entity.SilenceMark) error {
	// Serialize JSON fields
	labelsJSON, err := marshalJSON(silence.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	query := `
		INSERT INTO silences (
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at,
			created_by, created_by_email, reason, source,
			version, created_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?,
			?, ?, ?, ?,
			1, ?
		)
	`

	_, err = r.db.Primary().ExecContext(ctx, query,
		silence.ID,
		nullString(silence.AlertID),
		nullString(silence.Instance),
		nullString(silence.Fingerprint),
		labelsJSON,
		timeToTimestamp(silence.StartAt),
		timeToTimestamp(silence.EndAt),
		nullString(silence.CreatedBy),
		nullString(silence.CreatedByEmail),
		silence.Reason,
		string(silence.Source),
		timeToTimestamp(silence.CreatedAt),
	)

	if err != nil {
		if isDuplicateError(err) {
			return repository.ErrAlreadyExists
		}
		return fmt.Errorf("inserting silence: %w", err)
	}

	return nil
}

// FindByID retrieves a silence by its ID.
// Returns nil, nil if not found.
func (r *SilenceRepository) FindByID(ctx context.Context, id string) (*entity.SilenceMark, error) {
	query := `
		SELECT
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at,
			created_by, created_by_email, reason, source,
			version, created_at
		FROM silences
		WHERE id = ?
	`

	var silence entity.SilenceMark
	var alertID, instance, fingerprint, createdBy, createdByEmail sql.NullString
	var labelsJSON string
	var version int

	err := r.db.Replica().QueryRowContext(ctx, query, id).Scan(
		&silence.ID,
		&alertID,
		&instance,
		&fingerprint,
		&labelsJSON,
		&silence.StartAt,
		&silence.EndAt,
		&createdBy,
		&createdByEmail,
		&silence.Reason,
		&silence.Source,
		&version,
		&silence.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying silence: %w", err)
	}

	// Deserialize JSON fields
	if err := unmarshalJSON(labelsJSON, &silence.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}

	// Set nullable fields
	silence.AlertID = stringValue(alertID)
	silence.Instance = stringValue(instance)
	silence.Fingerprint = stringValue(fingerprint)
	silence.CreatedBy = stringValue(createdBy)
	silence.CreatedByEmail = stringValue(createdByEmail)

	return &silence, nil
}

// FindActive returns all currently active silences.
// Active means: start_at <= NOW() AND end_at > NOW()
func (r *SilenceRepository) FindActive(ctx context.Context) ([]*entity.SilenceMark, error) {
	query := `
		SELECT
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at,
			created_by, created_by_email, reason, source,
			version, created_at
		FROM silences
		WHERE start_at <= NOW() AND end_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying active silences: %w", err)
	}
	defer rows.Close()

	return r.scanSilences(rows)
}

// FindByAlertID retrieves active silences for a specific alert.
func (r *SilenceRepository) FindByAlertID(ctx context.Context, alertID string) ([]*entity.SilenceMark, error) {
	query := `
		SELECT
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at,
			created_by, created_by_email, reason, source,
			version, created_at
		FROM silences
		WHERE alert_id = ?
		  AND start_at <= NOW() AND end_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query, alertID)
	if err != nil {
		return nil, fmt.Errorf("querying silences by alert ID: %w", err)
	}
	defer rows.Close()

	return r.scanSilences(rows)
}

// FindByInstance retrieves active silences for a specific instance.
func (r *SilenceRepository) FindByInstance(ctx context.Context, instance string) ([]*entity.SilenceMark, error) {
	query := `
		SELECT
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at,
			created_by, created_by_email, reason, source,
			version, created_at
		FROM silences
		WHERE instance = ?
		  AND start_at <= NOW() AND end_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query, instance)
	if err != nil {
		return nil, fmt.Errorf("querying silences by instance: %w", err)
	}
	defer rows.Close()

	return r.scanSilences(rows)
}

// FindByFingerprint retrieves active silences for a specific fingerprint.
func (r *SilenceRepository) FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.SilenceMark, error) {
	query := `
		SELECT
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at,
			created_by, created_by_email, reason, source,
			version, created_at
		FROM silences
		WHERE fingerprint = ?
		  AND start_at <= NOW() AND end_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("querying silences by fingerprint: %w", err)
	}
	defer rows.Close()

	return r.scanSilences(rows)
}

// FindMatchingAlert returns all active silences that match the given alert.
// This implements the complex matching logic:
// 1. Match by alert_id (exact match)
// 2. Match by fingerprint (exact match)
// 3. Match by instance (with optional label matching)
// 4. Match by labels only (all silence labels must be present in alert)
func (r *SilenceRepository) FindMatchingAlert(ctx context.Context, alert *entity.Alert) ([]*entity.SilenceMark, error) {
	// Get all active silences
	query := `
		SELECT
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at,
			created_by, created_by_email, reason, source,
			version, created_at
		FROM silences
		WHERE start_at <= NOW() AND end_at > NOW()
	`

	rows, err := r.db.Replica().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying active silences: %w", err)
	}
	defer rows.Close()

	allSilences, err := r.scanSilences(rows)
	if err != nil {
		return nil, err
	}

	// Filter silences that match the alert using entity business logic
	matching := make([]*entity.SilenceMark, 0)
	for _, silence := range allSilences {
		if silence.MatchesAlert(alert) {
			matching = append(matching, silence)
		}
	}

	return matching, nil
}

// Update modifies an existing silence with optimistic locking.
// Returns ErrNotFound if the silence doesn't exist.
// Returns ErrConcurrentUpdate if the silence was modified by another instance.
func (r *SilenceRepository) Update(ctx context.Context, silence *entity.SilenceMark) error {
	// First, get the current version
	var currentVersion int
	versionQuery := `SELECT version FROM silences WHERE id = ?`
	err := r.db.Primary().QueryRowContext(ctx, versionQuery, silence.ID).Scan(&currentVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return fmt.Errorf("checking silence version: %w", err)
	}

	// Serialize JSON fields
	labelsJSON, err := marshalJSON(silence.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	// Update with optimistic locking (increment version)
	query := `
		UPDATE silences SET
			alert_id = ?,
			instance = ?,
			fingerprint = ?,
			labels = ?,
			start_at = ?,
			end_at = ?,
			created_by = ?,
			created_by_email = ?,
			reason = ?,
			source = ?,
			version = version + 1
		WHERE id = ? AND version = ?
	`

	result, err := r.db.Primary().ExecContext(ctx, query,
		nullString(silence.AlertID),
		nullString(silence.Instance),
		nullString(silence.Fingerprint),
		labelsJSON,
		timeToTimestamp(silence.StartAt),
		timeToTimestamp(silence.EndAt),
		nullString(silence.CreatedBy),
		nullString(silence.CreatedByEmail),
		silence.Reason,
		string(silence.Source),
		silence.ID,
		currentVersion,
	)

	if err != nil {
		return fmt.Errorf("updating silence: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Either the silence doesn't exist or version mismatch
		var exists bool
		existsQuery := `SELECT COUNT(*) > 0 FROM silences WHERE id = ?`
		err := r.db.Primary().QueryRowContext(ctx, existsQuery, silence.ID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking silence existence: %w", err)
		}

		if !exists {
			return repository.ErrNotFound
		}

		// Silence exists but version mismatch - concurrent update detected
		return repository.ErrConcurrentUpdate
	}

	return nil
}

// Delete removes a silence by ID.
// Returns ErrNotFound if the silence doesn't exist.
func (r *SilenceRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM silences WHERE id = ?`

	result, err := r.db.Primary().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting silence: %w", err)
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

// DeleteExpired removes all expired silences.
// Returns the number of deleted silences.
func (r *SilenceRepository) DeleteExpired(ctx context.Context) (int, error) {
	query := `DELETE FROM silences WHERE end_at < NOW()`

	result, err := r.db.Primary().ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("deleting expired silences: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("checking rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// scanSilences is a helper function to scan multiple silences from query results.
func (r *SilenceRepository) scanSilences(rows *sql.Rows) ([]*entity.SilenceMark, error) {
	silences := make([]*entity.SilenceMark, 0)

	for rows.Next() {
		var silence entity.SilenceMark
		var alertID, instance, fingerprint, createdBy, createdByEmail sql.NullString
		var labelsJSON string
		var version int

		err := rows.Scan(
			&silence.ID,
			&alertID,
			&instance,
			&fingerprint,
			&labelsJSON,
			&silence.StartAt,
			&silence.EndAt,
			&createdBy,
			&createdByEmail,
			&silence.Reason,
			&silence.Source,
			&version,
			&silence.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scanning silence row: %w", err)
		}

		// Deserialize JSON fields
		if err := unmarshalJSON(labelsJSON, &silence.Labels); err != nil {
			return nil, fmt.Errorf("unmarshaling labels: %w", err)
		}

		// Set nullable fields
		silence.AlertID = stringValue(alertID)
		silence.Instance = stringValue(instance)
		silence.Fingerprint = stringValue(fingerprint)
		silence.CreatedBy = stringValue(createdBy)
		silence.CreatedByEmail = stringValue(createdByEmail)

		silences = append(silences, &silence)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating silence rows: %w", err)
	}

	return silences, nil
}
