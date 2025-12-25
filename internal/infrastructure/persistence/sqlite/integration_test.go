package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPersistenceAcrossRestart verifies that data survives database restart.
func TestPersistenceAcrossRestart(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Phase 1: Create database, save data, close
	func() {
		db, err := NewDB(dbPath)
		require.NoError(t, err)
		defer db.Close()

		err = db.Migrate(context.Background())
		require.NoError(t, err)

		alertRepo := NewAlertRepository(db.DB)
		ackRepo := NewAckEventRepository(db.DB)
		silenceRepo := NewSilenceRepository(db.DB)

		// Create alert
		alert := &entity.Alert{
			ID:          "persist-alert",
			Fingerprint: "fp-persist",
			Name:        "Persistence Test Alert",
			Instance:    "localhost:9090",
			Severity:    entity.SeverityCritical,
			State:       entity.StateActive,
			Labels:      map[string]string{"env": "test"},
			Annotations: map[string]string{"runbook": "https://example.com"},
			FiredAt:     time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		err = alertRepo.Save(context.Background(), alert)
		require.NoError(t, err)

		// Create ack event
		ackEvent := &entity.AckEvent{
			ID:        "persist-ack",
			AlertID:   alert.ID,
			Source:    entity.AckSourceSlack,
			UserID:    "U123",
			UserEmail: "test@example.com",
			UserName:  "Test User",
			Note:      "Test acknowledgment",
			CreatedAt: time.Now().UTC(),
		}
		err = ackRepo.Save(context.Background(), ackEvent)
		require.NoError(t, err)

		// Create silence
		silence := &entity.SilenceMark{
			ID:             "persist-silence",
			AlertID:        alert.ID,
			StartAt:        time.Now().UTC(),
			EndAt:          time.Now().UTC().Add(1 * time.Hour),
			CreatedBy:      "test@example.com",
			CreatedByEmail: "test@example.com",
			Reason:         "Test silence",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"test": "true"},
			CreatedAt:      time.Now().UTC(),
		}
		err = silenceRepo.Save(context.Background(), silence)
		require.NoError(t, err)
	}()

	// Verify database file exists
	_, err := os.Stat(dbPath)
	require.NoError(t, err, "database file should exist")

	// Phase 2: Reopen database, verify data persisted
	func() {
		db, err := NewDB(dbPath)
		require.NoError(t, err)
		defer db.Close()

		err = db.Migrate(context.Background())
		require.NoError(t, err)

		alertRepo := NewAlertRepository(db.DB)
		ackRepo := NewAckEventRepository(db.DB)
		silenceRepo := NewSilenceRepository(db.DB)

		// Verify alert persisted
		alert, err := alertRepo.FindByID(context.Background(), "persist-alert")
		require.NoError(t, err)
		require.NotNil(t, alert)
		assert.Equal(t, "Persistence Test Alert", alert.Name)
		assert.Equal(t, "fp-persist", alert.Fingerprint)
		assert.Equal(t, entity.SeverityCritical, alert.Severity)
		assert.Equal(t, "test", alert.Labels["env"])

		// Verify ack event persisted
		ackEvent, err := ackRepo.FindByID(context.Background(), "persist-ack")
		require.NoError(t, err)
		require.NotNil(t, ackEvent)
		assert.Equal(t, "persist-alert", ackEvent.AlertID)
		assert.Equal(t, "Test acknowledgment", ackEvent.Note)

		// Verify silence persisted
		silence, err := silenceRepo.FindByID(context.Background(), "persist-silence")
		require.NoError(t, err)
		require.NotNil(t, silence)
		assert.Equal(t, "persist-alert", silence.AlertID)
		assert.Equal(t, "Test silence", silence.Reason)
		assert.Equal(t, "true", silence.Labels["test"])
	}()
}

// TestConcurrentWrites verifies concurrent write safety.
func TestConcurrentWrites(t *testing.T) {
	db, err := NewDB(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = db.Migrate(context.Background())
	require.NoError(t, err)

	alertRepo := NewAlertRepository(db.DB)
	ackRepo := NewAckEventRepository(db.DB)

	// Create base alert
	alert := &entity.Alert{
		ID:          "concurrent-alert",
		Fingerprint: "fp-concurrent",
		Name:        "Concurrent Test Alert",
		Severity:    entity.SeverityCritical,
		State:       entity.StateActive,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
		FiredAt:     time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	err = alertRepo.Save(context.Background(), alert)
	require.NoError(t, err)

	// Concurrently write ack events
	const numWorkers = 10
	done := make(chan bool, numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			ackEvent := &entity.AckEvent{
				ID:        fmt.Sprintf("ack-%d", id),
				AlertID:   alert.ID,
				Source:    entity.AckSourceSlack,
				UserID:    fmt.Sprintf("U%d", id),
				UserEmail: fmt.Sprintf("user%d@example.com", id),
				UserName:  fmt.Sprintf("User %d", id),
				CreatedAt: time.Now().UTC(),
			}
			err := ackRepo.Save(context.Background(), ackEvent)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all workers
	for i := 0; i < numWorkers; i++ {
		<-done
	}

	// Verify all ack events were saved
	ackEvents, err := ackRepo.FindByAlertID(context.Background(), alert.ID)
	require.NoError(t, err)
	assert.Len(t, ackEvents, numWorkers)
}

// TestForeignKeyConstraints verifies foreign key enforcement.
func TestForeignKeyConstraints(t *testing.T) {
	db, err := NewDB(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = db.Migrate(context.Background())
	require.NoError(t, err)

	ackRepo := NewAckEventRepository(db.DB)

	// Try to create ack event for non-existent alert
	ackEvent := &entity.AckEvent{
		ID:        "orphan-ack",
		AlertID:   "nonexistent-alert",
		Source:    entity.AckSourceSlack,
		UserID:    "U123",
		UserEmail: "test@example.com",
		UserName:  "Test User",
		CreatedAt: time.Now().UTC(),
	}

	err = ackRepo.Save(context.Background(), ackEvent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert not found")
}

// TestCascadeDelete verifies cascade delete behavior.
func TestCascadeDelete(t *testing.T) {
	db, err := NewDB(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = db.Migrate(context.Background())
	require.NoError(t, err)

	alertRepo := NewAlertRepository(db.DB)
	ackRepo := NewAckEventRepository(db.DB)

	// Create alert with ack events
	alert := &entity.Alert{
		ID:          "cascade-alert",
		Fingerprint: "fp-cascade",
		Name:        "Cascade Test",
		Severity:    entity.SeverityCritical,
		State:       entity.StateActive,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
		FiredAt:     time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	err = alertRepo.Save(context.Background(), alert)
	require.NoError(t, err)

	// Create ack events
	for i := 0; i < 3; i++ {
		ackEvent := &entity.AckEvent{
			ID:        fmt.Sprintf("cascade-ack-%d", i),
			AlertID:   alert.ID,
			Source:    entity.AckSourceSlack,
			UserID:    "U123",
			UserEmail: "test@example.com",
			UserName:  "Test User",
			CreatedAt: time.Now().UTC(),
		}
		err = ackRepo.Save(context.Background(), ackEvent)
		require.NoError(t, err)
	}

	// Verify ack events exist
	ackEvents, err := ackRepo.FindByAlertID(context.Background(), alert.ID)
	require.NoError(t, err)
	require.Len(t, ackEvents, 3)

	// Delete alert
	err = alertRepo.Delete(context.Background(), alert.ID)
	require.NoError(t, err)

	// Verify ack events were cascade deleted
	ackEvents, err = ackRepo.FindByAlertID(context.Background(), alert.ID)
	require.NoError(t, err)
	assert.Empty(t, ackEvents)
}
