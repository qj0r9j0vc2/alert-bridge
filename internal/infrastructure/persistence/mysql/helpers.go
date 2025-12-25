package mysql

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
)

// nullString converts a string to sql.NullString.
// Returns NULL if the string is empty.
func nullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

// nullTime converts a *time.Time to sql.NullTime.
// Returns NULL if the pointer is nil.
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{
		Time:  *t,
		Valid: true,
	}
}

// timePtr converts sql.NullTime to *time.Time.
// Returns nil if the value is NULL.
func timePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

// stringValue converts sql.NullString to string.
// Returns empty string if the value is NULL.
func stringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

// timeToTimestamp converts time.Time to MySQL TIMESTAMP format.
// MySQL TIMESTAMP is stored in UTC and converted to local timezone on retrieval.
func timeToTimestamp(t time.Time) time.Time {
	return t.UTC()
}

// durationToSeconds converts time.Duration to seconds (int).
// Returns 0 if duration is nil or zero.
func durationToSeconds(d *time.Duration) sql.NullInt64 {
	if d == nil || *d == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{
		Int64: int64(d.Seconds()),
		Valid: true,
	}
}

// secondsToDuration converts seconds (int) to time.Duration.
// Returns nil if the value is NULL.
func secondsToDuration(seconds sql.NullInt64) *time.Duration {
	if !seconds.Valid {
		return nil
	}
	d := time.Duration(seconds.Int64) * time.Second
	return &d
}

// marshalJSON converts a Go value to JSON string for storage.
// Returns error if marshaling fails.
func marshalJSON(v interface{}) (string, error) {
	if v == nil {
		return "{}", nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshaling JSON: %w", err)
	}

	return string(data), nil
}

// unmarshalJSON converts a JSON string to a Go value.
// Returns error if unmarshaling fails.
func unmarshalJSON(data string, v interface{}) error {
	if data == "" {
		data = "{}"
	}

	if err := json.Unmarshal([]byte(data), v); err != nil {
		return fmt.Errorf("unmarshaling JSON: %w", err)
	}

	return nil
}

// mapError maps MySQL errors to domain repository errors.
// This provides a consistent error interface across different storage implementations.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	// Check for sql.ErrNoRows (not found)
	if err == sql.ErrNoRows {
		return repository.ErrNotFound
	}

	// Check for MySQL-specific errors
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		switch mysqlErr.Number {
		case 1062: // ER_DUP_ENTRY - Duplicate entry
			return repository.ErrAlreadyExists
		case 1452: // ER_NO_REFERENCED_ROW_2 - Foreign key constraint fails
			return repository.ErrNotFound // Referenced entity doesn't exist
		case 1213: // ER_LOCK_DEADLOCK - Deadlock found
			return repository.ErrConcurrentUpdate // Retry needed
		case 1205: // ER_LOCK_WAIT_TIMEOUT - Lock wait timeout exceeded
			return repository.ErrConcurrentUpdate // Retry needed
		}
	}

	// Return original error if no mapping applies
	return err
}

// isRetryable checks if an error is retryable (transient failure).
// Returns true for deadlocks, lock timeouts, and connection errors.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for MySQL-specific retryable errors
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		switch mysqlErr.Number {
		case 1213: // ER_LOCK_DEADLOCK
			return true
		case 1205: // ER_LOCK_WAIT_TIMEOUT
			return true
		case 2006: // ER_SERVER_GONE_ERROR - MySQL server has gone away
			return true
		case 2013: // ER_SERVER_LOST - Lost connection to MySQL server during query
			return true
		}
	}

	// Check for connection errors
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") {
		return true
	}

	return false
}

// isForeignKeyError checks if an error is a foreign key constraint violation.
func isForeignKeyError(err error) bool {
	if err == nil {
		return false
	}

	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		switch mysqlErr.Number {
		case 1452: // ER_NO_REFERENCED_ROW_2 - Cannot add or update a child row
			return true
		case 1451: // ER_ROW_IS_REFERENCED_2 - Cannot delete or update a parent row
			return true
		}
	}

	return false
}

// isDuplicateError checks if an error is a duplicate key constraint violation.
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}

	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		return mysqlErr.Number == 1062 // ER_DUP_ENTRY
	}

	return false
}
