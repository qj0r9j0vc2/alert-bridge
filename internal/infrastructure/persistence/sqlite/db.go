package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB wraps a sql.DB connection with SQLite-specific functionality.
type DB struct {
	*sql.DB
	path string
}

// NewDB creates a new SQLite database connection.
// Use ":memory:" for an in-memory database.
func NewDB(path string) (*DB, error) {
	// Ensure directory exists for file-based database
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			// Directory creation is handled by the caller or config
		}
	}

	// Build connection string with pragmas
	dsn := path
	if path != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", path)
	} else {
		dsn = "file::memory:?cache=shared&_pragma=foreign_keys(ON)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite works best with single connection for writes
	db.SetMaxOpenConns(1)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{DB: db, path: path}, nil
}

// Migrate runs all pending database migrations.
func (db *DB) Migrate(ctx context.Context) error {
	// Check current schema version
	var currentVersion int
	err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		// Table doesn't exist yet, that's fine
		currentVersion = 0
	}

	// Read and execute migration SQL
	data, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	// Only run if not already applied
	if currentVersion < 1 {
		_, err = db.ExecContext(ctx, string(data))
		if err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}

// Close closes the database connection with proper cleanup.
func (db *DB) Close() error {
	// Force WAL checkpoint before close (only for file-based databases)
	if db.path != ":memory:" {
		_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	}
	return db.DB.Close()
}

// Ping verifies the database connection is alive.
func (db *DB) Ping(ctx context.Context) error {
	return db.DB.PingContext(ctx)
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}
