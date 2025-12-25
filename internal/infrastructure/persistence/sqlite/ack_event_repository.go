package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// AckEventRepository provides SQLite implementation of repository.AckEventRepository.
type AckEventRepository struct {
	db *sql.DB
}

// NewAckEventRepository creates a new SQLite-backed ack event repository.
func NewAckEventRepository(db *sql.DB) *AckEventRepository {
	return &AckEventRepository{db: db}
}

// Save persists a new ack event.
// Returns error if the referenced alert doesn't exist (foreign key constraint).
func (r *AckEventRepository) Save(ctx context.Context, event *entity.AckEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ack_events (
			id, alert_id, source, user_id, user_email, user_name,
			note, duration_seconds, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		event.ID, event.AlertID, string(event.Source),
		event.UserID, event.UserEmail, event.UserName,
		nullString(event.Note), durationToSeconds(event.Duration),
		timeToString(event.CreatedAt),
	)

	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("alert not found: %w", err)
		}
		return fmt.Errorf("insert ack event: %w", err)
	}

	return nil
}

// FindByID retrieves an ack event by its unique identifier.
// Returns nil, nil if not found.
func (r *AckEventRepository) FindByID(ctx context.Context, id string) (*entity.AckEvent, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, alert_id, source, user_id, user_email, user_name,
			note, duration_seconds, created_at
		FROM ack_events WHERE id = ?
	`, id)

	return scanAckEvent(row)
}

// FindByAlertID retrieves all ack events for an alert, ordered by creation time (oldest first).
// Returns empty slice if none found.
func (r *AckEventRepository) FindByAlertID(ctx context.Context, alertID string) ([]*entity.AckEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, alert_id, source, user_id, user_email, user_name,
			note, duration_seconds, created_at
		FROM ack_events WHERE alert_id = ?
		ORDER BY created_at ASC
	`, alertID)
	if err != nil {
		return nil, fmt.Errorf("query ack events by alert ID: %w", err)
	}
	defer rows.Close()

	return scanAckEvents(rows)
}

// FindLatestByAlertID retrieves the most recent ack event for an alert.
// Returns nil, nil if not found.
func (r *AckEventRepository) FindLatestByAlertID(ctx context.Context, alertID string) (*entity.AckEvent, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, alert_id, source, user_id, user_email, user_name,
			note, duration_seconds, created_at
		FROM ack_events WHERE alert_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, alertID)

	return scanAckEvent(row)
}

// scanAckEvent scans a single row into an AckEvent entity.
func scanAckEvent(row *sql.Row) (*entity.AckEvent, error) {
	var (
		event           entity.AckEvent
		source          string
		note            sql.NullString
		durationSeconds sql.NullInt64
		createdAt       string
	)

	err := row.Scan(
		&event.ID, &event.AlertID, &source,
		&event.UserID, &event.UserEmail, &event.UserName,
		&note, &durationSeconds, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan ack event: %w", err)
	}

	// Convert string fields
	event.Source = entity.AckSource(source)
	event.Note = stringFromNull(note)
	event.Duration = secondsToDuration(durationSeconds)

	// Parse timestamp
	event.CreatedAt, _ = parseTime(createdAt)

	return &event, nil
}

// scanAckEvents scans multiple rows into AckEvent entities.
func scanAckEvents(rows *sql.Rows) ([]*entity.AckEvent, error) {
	var events []*entity.AckEvent

	for rows.Next() {
		var (
			event           entity.AckEvent
			source          string
			note            sql.NullString
			durationSeconds sql.NullInt64
			createdAt       string
		)

		err := rows.Scan(
			&event.ID, &event.AlertID, &source,
			&event.UserID, &event.UserEmail, &event.UserName,
			&note, &durationSeconds, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ack event row: %w", err)
		}

		// Convert string fields
		event.Source = entity.AckSource(source)
		event.Note = stringFromNull(note)
		event.Duration = secondsToDuration(durationSeconds)

		// Parse timestamp
		event.CreatedAt, _ = parseTime(createdAt)

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Return empty slice instead of nil
	if events == nil {
		events = []*entity.AckEvent{}
	}

	return events, nil
}
