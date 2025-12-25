package mysql

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrator handles database schema migrations with version tracking.
type Migrator struct {
	db *sql.DB
}

// NewMigrator creates a new migrator for the given database connection.
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// Migration represents a single database migration.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Up runs all pending migrations to bring the database schema up to date.
// It tracks applied migrations in the schema_migrations table.
func (m *Migrator) Up(ctx context.Context) error {
	// Get current version
	currentVersion, err := m.currentVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting current version: %w", err)
	}

	// Load all migrations
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	// Filter pending migrations
	pending := make([]Migration, 0)
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			pending = append(pending, migration)
		}
	}

	if len(pending) == 0 {
		return nil // No pending migrations
	}

	// Apply each pending migration in a transaction
	for _, migration := range pending {
		if err := m.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("applying migration %d (%s): %w", migration.Version, migration.Name, err)
		}
	}

	return nil
}

// Down rolls back the last applied migration.
// This is not commonly used in production but useful for development.
func (m *Migrator) Down(ctx context.Context) error {
	// Get current version
	currentVersion, err := m.currentVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting current version: %w", err)
	}

	if currentVersion == 0 {
		return nil // No migrations to roll back
	}

	// For down migrations, we would need separate down migration files
	// For now, we'll just remove the version record
	// In production, down migrations are rarely used
	query := `DELETE FROM schema_migrations WHERE version = ?`
	if _, err := m.db.ExecContext(ctx, query, currentVersion); err != nil {
		return fmt.Errorf("removing migration version: %w", err)
	}

	return nil
}

// currentVersion returns the latest applied migration version.
// Returns 0 if no migrations have been applied.
func (m *Migrator) currentVersion(ctx context.Context) (int, error) {
	// First, check if schema_migrations table exists
	var tableExists bool
	checkTableQuery := `
		SELECT COUNT(*) > 0
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		AND table_name = 'schema_migrations'
	`
	if err := m.db.QueryRowContext(ctx, checkTableQuery).Scan(&tableExists); err != nil {
		return 0, fmt.Errorf("checking schema_migrations table: %w", err)
	}

	if !tableExists {
		return 0, nil // No migrations applied yet
	}

	// Get the latest version
	var version sql.NullInt64
	query := `SELECT MAX(version) FROM schema_migrations`
	if err := m.db.QueryRowContext(ctx, query).Scan(&version); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("querying current version: %w", err)
	}

	if !version.Valid {
		return 0, nil
	}

	return int(version.Int64), nil
}

// applyMigration applies a single migration in a transaction.
func (m *Migrator) applyMigration(ctx context.Context, migration Migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration SQL
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("executing migration SQL: %w", err)
	}

	// Record migration version (if not already recorded by the migration itself)
	// Note: 001_initial.sql includes its own INSERT, so we use ON DUPLICATE KEY UPDATE
	recordQuery := `
		INSERT INTO schema_migrations (version, applied_at)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE applied_at = VALUES(applied_at)
	`
	if _, err := tx.ExecContext(ctx, recordQuery, migration.Version, time.Now()); err != nil {
		return fmt.Errorf("recording migration version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// loadMigrations loads all migration files from the embedded filesystem.
// It parses filenames like "001_initial.sql" to extract version numbers.
func (m *Migrator) loadMigrations() ([]Migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		// Parse version from filename (e.g., "001_initial.sql" -> version 1)
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename format: %s (expected NNN_name.sql)", filename)
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("parsing version from filename %s: %w", filename, err)
		}

		// Extract name (remove .sql extension)
		name := strings.TrimSuffix(parts[1], ".sql")

		// Read migration SQL
		content, err := migrationFiles.ReadFile("migrations/" + filename)
		if err != nil {
			return nil, fmt.Errorf("reading migration file %s: %w", filename, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			SQL:     string(content),
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Version returns the current migration version.
func (m *Migrator) Version(ctx context.Context) (int, error) {
	return m.currentVersion(ctx)
}
