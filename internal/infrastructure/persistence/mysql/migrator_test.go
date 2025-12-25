package mysql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

func TestMigrator_LoadMigrations(t *testing.T) {
	m := &Migrator{}

	migrations, err := m.loadMigrations()
	require.NoError(t, err)

	// Should have at least one migration (001_initial.sql)
	require.NotEmpty(t, migrations)

	// First migration should be version 1
	assert.Equal(t, 1, migrations[0].Version)
	assert.Equal(t, "initial", migrations[0].Name)
	assert.Contains(t, migrations[0].SQL, "CREATE TABLE")

	// Migrations should be sorted by version
	for i := 1; i < len(migrations); i++ {
		assert.Greater(t, migrations[i].Version, migrations[i-1].Version)
	}
}

func TestMigrator_Up(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.MySQLConfig{
		Primary: config.MySQLInstanceConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			Username: "root",
			Password: "password",
		},
		Pool: config.MySQLPoolConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
		},
		Timeout:   5,
		ParseTime: true,
		Charset:   "utf8mb4",
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Clean up schema_migrations table
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS schema_migrations")
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS silences")
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS ack_events")
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS alerts")

	// Create migrator
	m := NewMigrator(db.Primary())

	ctx := context.Background()

	// Initial version should be 0
	version, err := m.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, version)

	// Run migrations
	err = m.Up(ctx)
	require.NoError(t, err)

	// Version should now be 1 (or higher if more migrations exist)
	version, err = m.Version(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, version, 1)

	// Verify schema_migrations table exists
	var count int
	err = db.Primary().QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)

	// Verify alerts table exists
	var tableExists bool
	err = db.Primary().QueryRow(`
		SELECT COUNT(*) > 0
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		AND table_name = 'alerts'
	`).Scan(&tableExists)
	require.NoError(t, err)
	assert.True(t, tableExists)

	// Running Up again should be idempotent (no error)
	err = m.Up(ctx)
	require.NoError(t, err)

	// Version should remain the same
	newVersion, err := m.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, version, newVersion)
}

func TestMigrator_CurrentVersion_NoTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.MySQLConfig{
		Primary: config.MySQLInstanceConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			Username: "root",
			Password: "password",
		},
		Pool: config.MySQLPoolConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
		},
		Timeout:   5,
		ParseTime: true,
		Charset:   "utf8mb4",
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Drop schema_migrations table if exists
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS schema_migrations")

	m := NewMigrator(db.Primary())

	ctx := context.Background()
	version, err := m.currentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, version)
}

func TestMigrator_CurrentVersion_EmptyTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.MySQLConfig{
		Primary: config.MySQLInstanceConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			Username: "root",
			Password: "password",
		},
		Pool: config.MySQLPoolConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
		},
		Timeout:   5,
		ParseTime: true,
		Charset:   "utf8mb4",
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Create empty schema_migrations table
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS schema_migrations")
	_, err = db.Primary().Exec(`
		CREATE TABLE schema_migrations (
			version INT PRIMARY KEY NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	m := NewMigrator(db.Primary())

	ctx := context.Background()
	version, err := m.currentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, version)
}

func TestMigrator_ApplyMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.MySQLConfig{
		Primary: config.MySQLInstanceConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			Username: "root",
			Password: "password",
		},
		Pool: config.MySQLPoolConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
		},
		Timeout:   5,
		ParseTime: true,
		Charset:   "utf8mb4",
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Create schema_migrations table
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS test_table")
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS schema_migrations")
	_, err = db.Primary().Exec(`
		CREATE TABLE schema_migrations (
			version INT PRIMARY KEY NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	m := NewMigrator(db.Primary())

	migration := Migration{
		Version: 999,
		Name:    "test_migration",
		SQL:     "CREATE TABLE test_table (id INT PRIMARY KEY)",
	}

	ctx := context.Background()
	err = m.applyMigration(ctx, migration)
	require.NoError(t, err)

	// Verify test_table was created
	var tableExists bool
	err = db.Primary().QueryRow(`
		SELECT COUNT(*) > 0
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		AND table_name = 'test_table'
	`).Scan(&tableExists)
	require.NoError(t, err)
	assert.True(t, tableExists)

	// Verify migration was recorded
	var recordedVersion int
	err = db.Primary().QueryRow("SELECT version FROM schema_migrations WHERE version = 999").Scan(&recordedVersion)
	require.NoError(t, err)
	assert.Equal(t, 999, recordedVersion)

	// Cleanup
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS test_table")
}

func TestMigrator_Down(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.MySQLConfig{
		Primary: config.MySQLInstanceConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			Username: "root",
			Password: "password",
		},
		Pool: config.MySQLPoolConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
		},
		Timeout:   5,
		ParseTime: true,
		Charset:   "utf8mb4",
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Create schema_migrations table and insert a version
	_, _ = db.Primary().Exec("DROP TABLE IF EXISTS schema_migrations")
	_, err = db.Primary().Exec(`
		CREATE TABLE schema_migrations (
			version INT PRIMARY KEY NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	_, err = db.Primary().Exec("INSERT INTO schema_migrations (version) VALUES (1)")
	require.NoError(t, err)

	m := NewMigrator(db.Primary())

	ctx := context.Background()

	// Current version should be 1
	version, err := m.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, version)

	// Run down migration
	err = m.Down(ctx)
	require.NoError(t, err)

	// Version should now be 0
	version, err = m.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, version)

	// Running down again should be safe (no error)
	err = m.Down(ctx)
	require.NoError(t, err)
}

func TestNewMigrator(t *testing.T) {
	db := &sql.DB{}
	m := NewMigrator(db)

	assert.NotNil(t, m)
	assert.Equal(t, db, m.db)
}
