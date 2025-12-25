package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDB_InMemory(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory database: %v", err)
	}
	defer db.Close()

	if db.Path() != ":memory:" {
		t.Errorf("expected path :memory:, got %s", db.Path())
	}
}

func TestNewDB_File(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("failed to create file database: %v", err)
	}
	defer db.Close()

	if db.Path() != dbPath {
		t.Errorf("expected path %s, got %s", dbPath, db.Path())
	}

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestDB_Migrate(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Run migration
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}

	// Verify tables were created
	tables := []string{"schema_version", "alerts", "ack_events", "silences"}
	for _, table := range tables {
		var name string
		err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s was not created: %v", table, err)
		}
	}

	// Verify schema version
	var version int
	err = db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 1 {
		t.Errorf("expected schema version 1, got %d", version)
	}
}

func TestDB_MigrateIdempotent(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Run migration twice
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("failed to run first migration: %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("failed to run second migration: %v", err)
	}

	// Verify schema version is still 1
	var version int
	err = db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 1 {
		t.Errorf("expected schema version 1, got %d", version)
	}
}

func TestDB_Ping(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Ping(ctx); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestDB_Close(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Verify connection is closed
	ctx := context.Background()
	if err := db.Ping(ctx); err == nil {
		t.Error("expected ping to fail after close")
	}
}

func TestDB_ForeignKeysEnabled(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Run migration to create tables
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}

	// Try to insert an ack_event with non-existent alert_id
	_, err = db.ExecContext(ctx, `
		INSERT INTO ack_events (id, alert_id, source, created_at)
		VALUES ('test-ack', 'non-existent-alert', 'api', datetime('now'))
	`)

	if err == nil {
		t.Error("expected foreign key constraint error, got nil")
	}
}
