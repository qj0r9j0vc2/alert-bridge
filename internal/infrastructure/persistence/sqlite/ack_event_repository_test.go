package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAckEventTest(t *testing.T) (*DB, *AlertRepository, *AckEventRepository) {
	t.Helper()

	db, err := NewDB(":memory:")
	require.NoError(t, err)

	err = db.Migrate(context.Background())
	require.NoError(t, err)

	alertRepo := NewAlertRepository(db.DB)
	ackRepo := NewAckEventRepository(db.DB)

	return db, alertRepo, ackRepo
}

func createTestAlert(t *testing.T, alertRepo *AlertRepository) *entity.Alert {
	t.Helper()

	alert := &entity.Alert{
		ID:          "alert-1",
		Fingerprint: "fp-1",
		Name:        "Test Alert",
		Instance:    "localhost:9090",
		Target:      "test-target",
		Summary:     "Test summary",
		Description: "Test description",
		Severity:    entity.SeverityCritical,
		State:       entity.StateActive,
		Labels:      map[string]string{"env": "test"},
		Annotations: map[string]string{"runbook": "https://example.com"},
		FiredAt:     time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	err := alertRepo.Save(context.Background(), alert)
	require.NoError(t, err)

	return alert
}

func TestAckEventRepository_Save(t *testing.T) {
	db, alertRepo, repo := setupAckEventTest(t)
	defer db.Close()

	alert := createTestAlert(t, alertRepo)

	t.Run("save new ack event", func(t *testing.T) {
		event := &entity.AckEvent{
			ID:        "ack-1",
			AlertID:   alert.ID,
			Source:    entity.AckSourceSlack,
			UserID:    "U12345",
			UserEmail: "user@example.com",
			UserName:  "Test User",
			Note:      "Investigating",
			CreatedAt: time.Now().UTC(),
		}

		err := repo.Save(context.Background(), event)
		assert.NoError(t, err)

		// Verify saved
		saved, err := repo.FindByID(context.Background(), event.ID)
		require.NoError(t, err)
		require.NotNil(t, saved)

		assert.Equal(t, event.ID, saved.ID)
		assert.Equal(t, event.AlertID, saved.AlertID)
		assert.Equal(t, event.Source, saved.Source)
		assert.Equal(t, event.UserID, saved.UserID)
		assert.Equal(t, event.UserEmail, saved.UserEmail)
		assert.Equal(t, event.UserName, saved.UserName)
		assert.Equal(t, event.Note, saved.Note)
	})

	t.Run("save with duration", func(t *testing.T) {
		duration := 1 * time.Hour
		event := &entity.AckEvent{
			ID:        "ack-2",
			AlertID:   alert.ID,
			Source:    entity.AckSourceAPI,
			UserID:    "user-api",
			UserEmail: "api@example.com",
			UserName:  "API User",
			Duration:  &duration,
			CreatedAt: time.Now().UTC(),
		}

		err := repo.Save(context.Background(), event)
		assert.NoError(t, err)

		saved, err := repo.FindByID(context.Background(), event.ID)
		require.NoError(t, err)
		require.NotNil(t, saved)
		require.NotNil(t, saved.Duration)
		assert.Equal(t, int64(3600), int64(saved.Duration.Seconds()))
	})

	t.Run("foreign key constraint", func(t *testing.T) {
		event := &entity.AckEvent{
			ID:        "ack-invalid",
			AlertID:   "nonexistent-alert",
			Source:    entity.AckSourceSlack,
			UserID:    "U12345",
			UserEmail: "user@example.com",
			UserName:  "Test User",
			CreatedAt: time.Now().UTC(),
		}

		err := repo.Save(context.Background(), event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alert not found")
	})
}

func TestAckEventRepository_FindByID(t *testing.T) {
	db, alertRepo, repo := setupAckEventTest(t)
	defer db.Close()

	alert := createTestAlert(t, alertRepo)

	t.Run("find existing event", func(t *testing.T) {
		event := &entity.AckEvent{
			ID:        "ack-find-1",
			AlertID:   alert.ID,
			Source:    entity.AckSourcePagerDuty,
			UserID:    "PD123",
			UserEmail: "pd@example.com",
			UserName:  "PD User",
			CreatedAt: time.Now().UTC(),
		}

		err := repo.Save(context.Background(), event)
		require.NoError(t, err)

		found, err := repo.FindByID(context.Background(), event.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, event.ID, found.ID)
		assert.Equal(t, event.AlertID, found.AlertID)
		assert.Equal(t, event.Source, found.Source)
	})

	t.Run("find nonexistent event", func(t *testing.T) {
		found, err := repo.FindByID(context.Background(), "nonexistent")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestAckEventRepository_FindByAlertID(t *testing.T) {
	db, alertRepo, repo := setupAckEventTest(t)
	defer db.Close()

	alert := createTestAlert(t, alertRepo)

	t.Run("find multiple events ordered by time", func(t *testing.T) {
		baseTime := time.Now().UTC()

		// Create events with different timestamps
		events := []*entity.AckEvent{
			{
				ID:        "ack-order-1",
				AlertID:   alert.ID,
				Source:    entity.AckSourceSlack,
				UserID:    "U1",
				UserEmail: "u1@example.com",
				UserName:  "User 1",
				CreatedAt: baseTime,
			},
			{
				ID:        "ack-order-2",
				AlertID:   alert.ID,
				Source:    entity.AckSourceSlack,
				UserID:    "U2",
				UserEmail: "u2@example.com",
				UserName:  "User 2",
				CreatedAt: baseTime.Add(1 * time.Minute),
			},
			{
				ID:        "ack-order-3",
				AlertID:   alert.ID,
				Source:    entity.AckSourceAPI,
				UserID:    "U3",
				UserEmail: "u3@example.com",
				UserName:  "User 3",
				CreatedAt: baseTime.Add(2 * time.Minute),
			},
		}

		// Save in random order
		for _, event := range events {
			err := repo.Save(context.Background(), event)
			require.NoError(t, err)
		}

		// Retrieve and verify order
		found, err := repo.FindByAlertID(context.Background(), alert.ID)
		require.NoError(t, err)
		require.Len(t, found, 3)

		// Should be ordered oldest first
		assert.Equal(t, "ack-order-1", found[0].ID)
		assert.Equal(t, "ack-order-2", found[1].ID)
		assert.Equal(t, "ack-order-3", found[2].ID)
	})

	t.Run("find for nonexistent alert", func(t *testing.T) {
		found, err := repo.FindByAlertID(context.Background(), "nonexistent")
		assert.NoError(t, err)
		assert.Empty(t, found)
	})
}

func TestAckEventRepository_FindLatestByAlertID(t *testing.T) {
	db, alertRepo, repo := setupAckEventTest(t)
	defer db.Close()

	alert := createTestAlert(t, alertRepo)

	t.Run("find latest event", func(t *testing.T) {
		baseTime := time.Now().UTC()

		// Create multiple events
		events := []*entity.AckEvent{
			{
				ID:        "ack-latest-1",
				AlertID:   alert.ID,
				Source:    entity.AckSourceSlack,
				UserID:    "U1",
				UserEmail: "u1@example.com",
				UserName:  "User 1",
				CreatedAt: baseTime,
			},
			{
				ID:        "ack-latest-2",
				AlertID:   alert.ID,
				Source:    entity.AckSourceSlack,
				UserID:    "U2",
				UserEmail: "u2@example.com",
				UserName:  "User 2",
				CreatedAt: baseTime.Add(5 * time.Minute),
			},
			{
				ID:        "ack-latest-3",
				AlertID:   alert.ID,
				Source:    entity.AckSourceAPI,
				UserID:    "U3",
				UserEmail: "u3@example.com",
				UserName:  "User 3",
				CreatedAt: baseTime.Add(2 * time.Minute),
			},
		}

		for _, event := range events {
			err := repo.Save(context.Background(), event)
			require.NoError(t, err)
		}

		// Get latest
		latest, err := repo.FindLatestByAlertID(context.Background(), alert.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		// Should be the one with the latest timestamp
		assert.Equal(t, "ack-latest-2", latest.ID)
	})

	t.Run("find latest for nonexistent alert", func(t *testing.T) {
		latest, err := repo.FindLatestByAlertID(context.Background(), "nonexistent")
		assert.NoError(t, err)
		assert.Nil(t, latest)
	})

	t.Run("find latest when no events", func(t *testing.T) {
		// Create alert with no ack events
		alert2 := &entity.Alert{
			ID:          "alert-no-acks",
			Fingerprint: "fp-no-acks",
			Name:        "Alert Without Acks",
			Severity:    entity.SeverityCritical,
			State:       entity.StateActive,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
			FiredAt:     time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		err := alertRepo.Save(context.Background(), alert2)
		require.NoError(t, err)

		latest, err := repo.FindLatestByAlertID(context.Background(), alert2.ID)
		assert.NoError(t, err)
		assert.Nil(t, latest)
	})
}

func TestAckEventRepository_EmptyFields(t *testing.T) {
	db, alertRepo, repo := setupAckEventTest(t)
	defer db.Close()

	alert := createTestAlert(t, alertRepo)

	t.Run("empty note and no duration", func(t *testing.T) {
		event := &entity.AckEvent{
			ID:        "ack-empty",
			AlertID:   alert.ID,
			Source:    entity.AckSourceSlack,
			UserID:    "U12345",
			UserEmail: "user@example.com",
			UserName:  "Test User",
			Note:      "", // Empty note
			Duration:  nil,
			CreatedAt: time.Now().UTC(),
		}

		err := repo.Save(context.Background(), event)
		require.NoError(t, err)

		saved, err := repo.FindByID(context.Background(), event.ID)
		require.NoError(t, err)
		require.NotNil(t, saved)

		assert.Empty(t, saved.Note)
		assert.Nil(t, saved.Duration)
	})
}
