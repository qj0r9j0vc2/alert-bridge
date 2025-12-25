package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
)

// AckEventRepository provides MySQL implementation of repository.AckEventRepository.
type AckEventRepository struct {
	db *DB
}

// NewAckEventRepository creates a new MySQL-backed ack event repository.
func NewAckEventRepository(db *DB) *AckEventRepository {
	return &AckEventRepository{db: db}
}

// Save persists a new ack event.
// Returns ErrNotFound if the referenced alert doesn't exist (FK constraint).
func (r *AckEventRepository) Save(ctx context.Context, event *entity.AckEvent) error {
	query := `
		INSERT INTO ack_events (
			id, alert_id, source,
			user_id, user_email, user_name,
			note, duration_seconds,
			created_at
		) VALUES (
			?, ?, ?,
			?, ?, ?,
			?, ?,
			?
		)
	`

	_, err := r.db.Primary().ExecContext(ctx, query,
		event.ID,
		event.AlertID,
		string(event.Source),
		nullString(event.UserID),
		nullString(event.UserEmail),
		nullString(event.UserName),
		nullString(event.Note),
		durationToSeconds(event.Duration),
		timeToTimestamp(event.CreatedAt),
	)

	if err != nil {
		if isForeignKeyError(err) {
			return repository.ErrNotFound // Alert doesn't exist
		}
		if isDuplicateError(err) {
			return repository.ErrAlreadyExists
		}
		return fmt.Errorf("inserting ack event: %w", err)
	}

	return nil
}

// FindByID retrieves an ack event by its ID.
// Returns nil, nil if not found.
func (r *AckEventRepository) FindByID(ctx context.Context, id string) (*entity.AckEvent, error) {
	query := `
		SELECT
			id, alert_id, source,
			user_id, user_email, user_name,
			note, duration_seconds,
			created_at
		FROM ack_events
		WHERE id = ?
	`

	var event entity.AckEvent
	var userID, userEmail, userName, note sql.NullString
	var durationSeconds sql.NullInt64

	err := r.db.Replica().QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.AlertID,
		&event.Source,
		&userID,
		&userEmail,
		&userName,
		&note,
		&durationSeconds,
		&event.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying ack event: %w", err)
	}

	// Set nullable fields
	event.UserID = stringValue(userID)
	event.UserEmail = stringValue(userEmail)
	event.UserName = stringValue(userName)
	event.Note = stringValue(note)
	event.Duration = secondsToDuration(durationSeconds)

	return &event, nil
}

// FindByAlertID retrieves all ack events for an alert.
// Returns empty slice if none found.
// Results are ordered chronologically (oldest first).
func (r *AckEventRepository) FindByAlertID(ctx context.Context, alertID string) ([]*entity.AckEvent, error) {
	query := `
		SELECT
			id, alert_id, source,
			user_id, user_email, user_name,
			note, duration_seconds,
			created_at
		FROM ack_events
		WHERE alert_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.Replica().QueryContext(ctx, query, alertID)
	if err != nil {
		return nil, fmt.Errorf("querying ack events by alert ID: %w", err)
	}
	defer rows.Close()

	return r.scanAckEvents(rows)
}

// FindLatestByAlertID retrieves the most recent ack event for an alert.
// Returns nil, nil if none found.
func (r *AckEventRepository) FindLatestByAlertID(ctx context.Context, alertID string) (*entity.AckEvent, error) {
	query := `
		SELECT
			id, alert_id, source,
			user_id, user_email, user_name,
			note, duration_seconds,
			created_at
		FROM ack_events
		WHERE alert_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	var event entity.AckEvent
	var userID, userEmail, userName, note sql.NullString
	var durationSeconds sql.NullInt64

	err := r.db.Replica().QueryRowContext(ctx, query, alertID).Scan(
		&event.ID,
		&event.AlertID,
		&event.Source,
		&userID,
		&userEmail,
		&userName,
		&note,
		&durationSeconds,
		&event.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying latest ack event: %w", err)
	}

	// Set nullable fields
	event.UserID = stringValue(userID)
	event.UserEmail = stringValue(userEmail)
	event.UserName = stringValue(userName)
	event.Note = stringValue(note)
	event.Duration = secondsToDuration(durationSeconds)

	return &event, nil
}

// scanAckEvents is a helper function to scan multiple ack events from query results.
func (r *AckEventRepository) scanAckEvents(rows *sql.Rows) ([]*entity.AckEvent, error) {
	events := make([]*entity.AckEvent, 0)

	for rows.Next() {
		var event entity.AckEvent
		var userID, userEmail, userName, note sql.NullString
		var durationSeconds sql.NullInt64

		err := rows.Scan(
			&event.ID,
			&event.AlertID,
			&event.Source,
			&userID,
			&userEmail,
			&userName,
			&note,
			&durationSeconds,
			&event.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scanning ack event row: %w", err)
		}

		// Set nullable fields
		event.UserID = stringValue(userID)
		event.UserEmail = stringValue(userEmail)
		event.UserName = stringValue(userName)
		event.Note = stringValue(note)
		event.Duration = secondsToDuration(durationSeconds)

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ack event rows: %w", err)
	}

	return events, nil
}
