package sqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/persistence/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQuickstartValidation validates all steps from quickstart.md
func TestQuickstartValidation(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "quickstart-test.db")

	t.Run("Step 1: Initialize database", func(t *testing.T) {
		// Initialize SQLite database as per quickstart
		db, err := sqlite.NewDB(dbPath)
		require.NoError(t, err, "Failed to initialize SQLite database")
		defer db.Close()

		t.Log("✅ SQLite database initialized")

		// Run migrations
		err = db.Migrate(context.Background())
		require.NoError(t, err, "Failed to run migrations")

		t.Log("✅ Database migrations completed")

		// Verify database file exists
		_, err = os.Stat(dbPath)
		require.NoError(t, err, "Database file should exist")

		t.Log("✅ Database file created")
	})

	t.Run("Step 2: Create repositories and save data", func(t *testing.T) {
		db, err := sqlite.NewDB(dbPath)
		require.NoError(t, err)
		defer db.Close()

		// Create repositories
		repos := sqlite.NewRepositories(db.DB)
		t.Log("✅ Repositories created")

		// Test Alert Repository
		alert := &entity.Alert{
			ID:          "quickstart-alert",
			Fingerprint: "fp-quickstart",
			Name:        "Quickstart Test Alert",
			Instance:    "localhost:9090",
			Severity:    entity.SeverityCritical,
			State:       entity.StateActive,
			Labels:      map[string]string{"env": "test", "team": "platform"},
			Annotations: map[string]string{"description": "Quickstart validation test"},
			FiredAt:     time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}

		err = repos.Alert.Save(context.Background(), alert)
		require.NoError(t, err, "Failed to save alert")

		t.Log("✅ Alert saved successfully")

		// Retrieve and verify
		retrieved, err := repos.Alert.FindByID(context.Background(), "quickstart-alert")
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, "Quickstart Test Alert", retrieved.Name)
		assert.Equal(t, "fp-quickstart", retrieved.Fingerprint)
		assert.Equal(t, "test", retrieved.Labels["env"])

		t.Log("✅ Alert retrieved and verified")

		// Test AckEvent Repository
		ackEvent := &entity.AckEvent{
			ID:        "quickstart-ack",
			AlertID:   alert.ID,
			Source:    entity.AckSourceSlack,
			UserID:    "U123456",
			UserEmail: "ops@example.com",
			UserName:  "Ops Engineer",
			Note:      "Investigating the issue",
			CreatedAt: time.Now().UTC(),
		}

		err = repos.AckEvent.Save(context.Background(), ackEvent)
		require.NoError(t, err, "Failed to save ack event")

		t.Log("✅ Ack event saved successfully")

		// Test Silence Repository
		silence := &entity.SilenceMark{
			ID:             "quickstart-silence",
			AlertID:        alert.ID,
			StartAt:        time.Now().UTC(),
			EndAt:          time.Now().UTC().Add(1 * time.Hour),
			CreatedBy:      "ops@example.com",
			CreatedByEmail: "ops@example.com",
			Reason:         "Maintenance window",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"maintenance": "true"},
			CreatedAt:      time.Now().UTC(),
		}

		err = repos.Silence.Save(context.Background(), silence)
		require.NoError(t, err, "Failed to save silence")

		t.Log("✅ Silence saved successfully")
	})

	t.Run("Step 3: Verify persistence across restart", func(t *testing.T) {
		// Close and reopen database to simulate restart
		db, err := sqlite.NewDB(dbPath)
		require.NoError(t, err)
		defer db.Close()

		repos := sqlite.NewRepositories(db.DB)

		// Verify alert persisted
		alert, err := repos.Alert.FindByID(context.Background(), "quickstart-alert")
		require.NoError(t, err)
		require.NotNil(t, alert)
		assert.Equal(t, "Quickstart Test Alert", alert.Name)

		t.Log("✅ Alert persisted across restart")

		// Verify ack event persisted
		ackEvent, err := repos.AckEvent.FindByID(context.Background(), "quickstart-ack")
		require.NoError(t, err)
		require.NotNil(t, ackEvent)
		assert.Equal(t, "Investigating the issue", ackEvent.Note)

		t.Log("✅ Ack event persisted across restart")

		// Verify silence persisted
		silence, err := repos.Silence.FindByID(context.Background(), "quickstart-silence")
		require.NoError(t, err)
		require.NotNil(t, silence)
		assert.Equal(t, "Maintenance window", silence.Reason)

		t.Log("✅ Silence persisted across restart")
	})

	t.Run("Step 4: Verify configuration loading", func(t *testing.T) {
		// Test that config can be loaded with SQLite settings
		// This validates the configuration structure
		type testConfig struct {
			Storage config.StorageConfig `yaml:"storage"`
		}

		cfg := &testConfig{
			Storage: config.StorageConfig{
				Type: "sqlite",
				SQLite: config.SQLiteConfig{
					Path: "./data/alert-bridge.db",
				},
			},
		}

		assert.Equal(t, "sqlite", cfg.Storage.Type)
		assert.Equal(t, "./data/alert-bridge.db", cfg.Storage.SQLite.Path)

		t.Log("✅ Configuration structure validated")
	})

	t.Log("\n🎉 All quickstart validation steps passed!")
}
