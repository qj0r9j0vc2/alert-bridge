package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// SilenceRepository provides SQLite implementation of repository.SilenceRepository.
type SilenceRepository struct {
	db *DB
}

// NewSilenceRepository creates a new SQLite-backed silence repository.
func NewSilenceRepository(db *DB) *SilenceRepository {
	return &SilenceRepository{db: db}
}

// Save persists a new silence.
func (r *SilenceRepository) Save(ctx context.Context, silence *entity.SilenceMark) error {
	labels, err := marshalJSON(silence.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	_, err = r.db.getExecutor(ctx).ExecContext(ctx, `
		INSERT INTO silences (
			id, alert_id, instance, fingerprint, labels,
			start_at, end_at, created_by, created_by_email, reason, source, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		silence.ID,
		nullString(silence.AlertID),
		nullString(silence.Instance),
		nullString(silence.Fingerprint),
		labels,
		timeToString(silence.StartAt),
		timeToString(silence.EndAt),
		silence.CreatedBy,
		silence.CreatedByEmail,
		silence.Reason,
		string(silence.Source),
		timeToString(silence.CreatedAt),
	)

	if err != nil {
		return fmt.Errorf("insert silence: %w", err)
	}

	return nil
}

// FindByID retrieves a silence by its unique identifier.
// Returns nil, nil if not found.
func (r *SilenceRepository) FindByID(ctx context.Context, id string) (*entity.SilenceMark, error) {
	row := r.db.getExecutor(ctx).QueryRowContext(ctx, `
		SELECT id, alert_id, instance, fingerprint, labels,
			start_at, end_at, created_by, created_by_email, reason, source, created_at
		FROM silences WHERE id = ?
	`, id)

	return scanSilence(row)
}

// FindActive returns all currently active silences (start_at <= now < end_at).
// Returns empty slice if none found.
func (r *SilenceRepository) FindActive(ctx context.Context) ([]*entity.SilenceMark, error) {
	now := timeToString(time.Now().UTC())

	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, alert_id, instance, fingerprint, labels,
			start_at, end_at, created_by, created_by_email, reason, source, created_at
		FROM silences
		WHERE start_at <= ? AND end_at > ?
	`, now, now)
	if err != nil {
		return nil, fmt.Errorf("query active silences: %w", err)
	}
	defer rows.Close()

	return scanSilences(rows)
}

// FindByAlertID retrieves active silences for a specific alert.
// Returns empty slice if none found.
func (r *SilenceRepository) FindByAlertID(ctx context.Context, alertID string) ([]*entity.SilenceMark, error) {
	now := timeToString(time.Now().UTC())

	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, alert_id, instance, fingerprint, labels,
			start_at, end_at, created_by, created_by_email, reason, source, created_at
		FROM silences
		WHERE alert_id = ? AND start_at <= ? AND end_at > ?
	`, alertID, now, now)
	if err != nil {
		return nil, fmt.Errorf("query silences by alert ID: %w", err)
	}
	defer rows.Close()

	return scanSilences(rows)
}

// FindByInstance retrieves active silences for a specific instance.
// Returns empty slice if none found.
func (r *SilenceRepository) FindByInstance(ctx context.Context, instance string) ([]*entity.SilenceMark, error) {
	now := timeToString(time.Now().UTC())

	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, alert_id, instance, fingerprint, labels,
			start_at, end_at, created_by, created_by_email, reason, source, created_at
		FROM silences
		WHERE instance = ? AND start_at <= ? AND end_at > ?
	`, instance, now, now)
	if err != nil {
		return nil, fmt.Errorf("query silences by instance: %w", err)
	}
	defer rows.Close()

	return scanSilences(rows)
}

// FindByFingerprint retrieves active silences for a specific fingerprint.
// Returns empty slice if none found.
func (r *SilenceRepository) FindByFingerprint(ctx context.Context, fingerprint string) ([]*entity.SilenceMark, error) {
	now := timeToString(time.Now().UTC())

	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, alert_id, instance, fingerprint, labels,
			start_at, end_at, created_by, created_by_email, reason, source, created_at
		FROM silences
		WHERE fingerprint = ? AND start_at <= ? AND end_at > ?
	`, fingerprint, now, now)
	if err != nil {
		return nil, fmt.Errorf("query silences by fingerprint: %w", err)
	}
	defer rows.Close()

	return scanSilences(rows)
}

// FindMatchingAlert returns all active silences that match the given alert.
// Uses complex matching logic: alert_id, fingerprint, instance, or label matching.
// Returns empty slice if none found.
func (r *SilenceRepository) FindMatchingAlert(ctx context.Context, alert *entity.Alert) ([]*entity.SilenceMark, error) {
	now := timeToString(time.Now().UTC())

	// Query all active silences
	rows, err := r.db.getExecutor(ctx).QueryContext(ctx, `
		SELECT id, alert_id, instance, fingerprint, labels,
			start_at, end_at, created_by, created_by_email, reason, source, created_at
		FROM silences
		WHERE start_at <= ? AND end_at > ?
	`, now, now)
	if err != nil {
		return nil, fmt.Errorf("query active silences: %w", err)
	}
	defer rows.Close()

	silences, err := scanSilences(rows)
	if err != nil {
		return nil, err
	}

	// Filter silences that match the alert
	var matches []*entity.SilenceMark
	for _, silence := range silences {
		if silence.MatchesAlert(alert) {
			matches = append(matches, silence)
		}
	}

	return matches, nil
}

// Update modifies an existing silence.
// Returns ErrSilenceNotFound if the silence doesn't exist.
func (r *SilenceRepository) Update(ctx context.Context, silence *entity.SilenceMark) error {
	labels, err := marshalJSON(silence.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	result, err := r.db.getExecutor(ctx).ExecContext(ctx, `
		UPDATE silences SET
			alert_id = ?, instance = ?, fingerprint = ?, labels = ?,
			start_at = ?, end_at = ?, created_by = ?, created_by_email = ?,
			reason = ?, source = ?
		WHERE id = ?
	`,
		nullString(silence.AlertID),
		nullString(silence.Instance),
		nullString(silence.Fingerprint),
		labels,
		timeToString(silence.StartAt),
		timeToString(silence.EndAt),
		silence.CreatedBy,
		silence.CreatedByEmail,
		silence.Reason,
		string(silence.Source),
		silence.ID,
	)
	if err != nil {
		return fmt.Errorf("update silence: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return entity.ErrSilenceNotFound
	}

	return nil
}

// Delete removes a silence by ID.
// Returns ErrSilenceNotFound if the silence doesn't exist.
func (r *SilenceRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.getExecutor(ctx).ExecContext(ctx, `DELETE FROM silences WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete silence: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return entity.ErrSilenceNotFound
	}

	return nil
}

// DeleteExpired removes all expired silences (end_at < now).
// Returns the number of silences deleted.
func (r *SilenceRepository) DeleteExpired(ctx context.Context) (int, error) {
	now := timeToString(time.Now().UTC())

	result, err := r.db.getExecutor(ctx).ExecContext(ctx, `DELETE FROM silences WHERE end_at < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired silences: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// scanSilence scans a single row into a SilenceMark entity.
func scanSilence(row *sql.Row) (*entity.SilenceMark, error) {
	var (
		silence     entity.SilenceMark
		alertID     sql.NullString
		instance    sql.NullString
		fingerprint sql.NullString
		labels      string
		startAt     string
		endAt       string
		source      string
		createdAt   string
	)

	err := row.Scan(
		&silence.ID, &alertID, &instance, &fingerprint, &labels,
		&startAt, &endAt, &silence.CreatedBy, &silence.CreatedByEmail,
		&silence.Reason, &source, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan silence: %w", err)
	}

	// Convert nullable fields
	silence.AlertID = stringFromNull(alertID)
	silence.Instance = stringFromNull(instance)
	silence.Fingerprint = stringFromNull(fingerprint)
	silence.Source = entity.AckSource(source)

	// Parse JSON labels
	silence.Labels, _ = unmarshalJSON(labels)

	// Parse timestamps
	silence.StartAt, _ = parseTime(startAt)
	silence.EndAt, _ = parseTime(endAt)
	silence.CreatedAt, _ = parseTime(createdAt)

	return &silence, nil
}

// scanSilences scans multiple rows into SilenceMark entities.
func scanSilences(rows *sql.Rows) ([]*entity.SilenceMark, error) {
	var silences []*entity.SilenceMark

	for rows.Next() {
		var (
			silence     entity.SilenceMark
			alertID     sql.NullString
			instance    sql.NullString
			fingerprint sql.NullString
			labels      string
			startAt     string
			endAt       string
			source      string
			createdAt   string
		)

		err := rows.Scan(
			&silence.ID, &alertID, &instance, &fingerprint, &labels,
			&startAt, &endAt, &silence.CreatedBy, &silence.CreatedByEmail,
			&silence.Reason, &source, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan silence row: %w", err)
		}

		// Convert nullable fields
		silence.AlertID = stringFromNull(alertID)
		silence.Instance = stringFromNull(instance)
		silence.Fingerprint = stringFromNull(fingerprint)
		silence.Source = entity.AckSource(source)

		// Parse JSON labels
		silence.Labels, _ = unmarshalJSON(labels)

		// Parse timestamps
		silence.StartAt, _ = parseTime(startAt)
		silence.EndAt, _ = parseTime(endAt)
		silence.CreatedAt, _ = parseTime(createdAt)

		silences = append(silences, &silence)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Return empty slice instead of nil
	if silences == nil {
		silences = []*entity.SilenceMark{}
	}

	return silences, nil
}
