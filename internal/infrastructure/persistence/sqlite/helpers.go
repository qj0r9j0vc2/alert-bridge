package sqlite

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// nullString converts a string to sql.NullString.
// Empty strings are stored as NULL.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullableString converts a string pointer to sql.NullString.
func nullableString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// stringFromNull converts sql.NullString back to string.
// Returns empty string for NULL values.
func stringFromNull(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

// nullTime converts a *time.Time to RFC3339 string for storage.
// Returns sql.NullString with Valid=false for nil times.
func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}

// timeToString converts time.Time to RFC3339 string.
func timeToString(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// scanNullTime converts a sql.NullString back to *time.Time.
// Returns nil for NULL values.
func scanNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return nil
	}
	return &t
}

// parseTime parses an RFC3339 string to time.Time.
func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// marshalJSON converts a map to JSON string for storage.
func marshalJSON(m map[string]string) (string, error) {
	if m == nil {
		return "{}", nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "{}", err
	}
	return string(data), nil
}

// unmarshalJSON converts a JSON string back to map.
func unmarshalJSON(s string) (map[string]string, error) {
	if s == "" || s == "{}" {
		return make(map[string]string), nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return make(map[string]string), err
	}
	if m == nil {
		m = make(map[string]string)
	}
	return m, nil
}

// isUniqueConstraintError checks if the error is a SQLite unique constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// isForeignKeyError checks if the error is a SQLite foreign key constraint violation.
func isForeignKeyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

// nullInt64 converts an *int64 to sql.NullInt64.
func nullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

// int64FromNull converts sql.NullInt64 back to *int64.
func int64FromNull(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

// durationToSeconds converts *time.Duration to *int64 (seconds).
func durationToSeconds(d *time.Duration) sql.NullInt64 {
	if d == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(d.Seconds()), Valid: true}
}

// secondsToDuration converts sql.NullInt64 (seconds) to *time.Duration.
func secondsToDuration(ni sql.NullInt64) *time.Duration {
	if !ni.Valid {
		return nil
	}
	d := time.Duration(ni.Int64) * time.Second
	return &d
}
