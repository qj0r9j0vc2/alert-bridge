package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSilenceTest(t *testing.T) (*DB, *SilenceRepository) {
	t.Helper()

	db, err := NewDB(":memory:")
	require.NoError(t, err)

	err = db.Migrate(context.Background())
	require.NoError(t, err)

	repo := NewSilenceRepository(db.DB)

	return db, repo
}

func TestSilenceRepository_Save(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	t.Run("save new silence with alert ID", func(t *testing.T) {
		silence := &entity.SilenceMark{
			ID:             "silence-1",
			AlertID:        "alert-1",
			StartAt:        time.Now().UTC(),
			EndAt:          time.Now().UTC().Add(1 * time.Hour),
			CreatedBy:      "user@example.com",
			CreatedByEmail: "user@example.com",
			Reason:         "Maintenance window",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"env": "prod"},
			CreatedAt:      time.Now().UTC(),
		}

		err := repo.Save(context.Background(), silence)
		assert.NoError(t, err)

		// Verify saved
		saved, err := repo.FindByID(context.Background(), silence.ID)
		require.NoError(t, err)
		require.NotNil(t, saved)

		assert.Equal(t, silence.ID, saved.ID)
		assert.Equal(t, silence.AlertID, saved.AlertID)
		assert.Equal(t, silence.Reason, saved.Reason)
		assert.Equal(t, silence.Source, saved.Source)
		assert.Equal(t, silence.Labels["env"], saved.Labels["env"])
	})

	t.Run("save silence with instance", func(t *testing.T) {
		silence := &entity.SilenceMark{
			ID:             "silence-2",
			Instance:       "localhost:9090",
			StartAt:        time.Now().UTC(),
			EndAt:          time.Now().UTC().Add(1 * time.Hour),
			CreatedBy:      "admin",
			CreatedByEmail: "admin@example.com",
			Reason:         "Planned maintenance",
			Source:         entity.AckSourceAPI,
			Labels:         map[string]string{},
			CreatedAt:      time.Now().UTC(),
		}

		err := repo.Save(context.Background(), silence)
		assert.NoError(t, err)

		saved, err := repo.FindByID(context.Background(), silence.ID)
		require.NoError(t, err)
		require.NotNil(t, saved)
		assert.Equal(t, silence.Instance, saved.Instance)
	})

	t.Run("save silence with fingerprint", func(t *testing.T) {
		silence := &entity.SilenceMark{
			ID:             "silence-3",
			Fingerprint:    "fp123",
			StartAt:        time.Now().UTC(),
			EndAt:          time.Now().UTC().Add(1 * time.Hour),
			CreatedBy:      "ops",
			CreatedByEmail: "ops@example.com",
			Reason:         "Known issue",
			Source:         entity.AckSourcePagerDuty,
			Labels:         map[string]string{},
			CreatedAt:      time.Now().UTC(),
		}

		err := repo.Save(context.Background(), silence)
		assert.NoError(t, err)

		saved, err := repo.FindByID(context.Background(), silence.ID)
		require.NoError(t, err)
		require.NotNil(t, saved)
		assert.Equal(t, silence.Fingerprint, saved.Fingerprint)
	})
}

func TestSilenceRepository_FindByID(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	t.Run("find existing silence", func(t *testing.T) {
		silence := &entity.SilenceMark{
			ID:             "silence-find",
			AlertID:        "alert-1",
			StartAt:        time.Now().UTC(),
			EndAt:          time.Now().UTC().Add(1 * time.Hour),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Reason:         "Testing",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"test": "true"},
			CreatedAt:      time.Now().UTC(),
		}

		err := repo.Save(context.Background(), silence)
		require.NoError(t, err)

		found, err := repo.FindByID(context.Background(), silence.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, silence.ID, found.ID)
	})

	t.Run("find nonexistent silence", func(t *testing.T) {
		found, err := repo.FindByID(context.Background(), "nonexistent")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSilenceRepository_FindActive(t *testing.T) {
	now := time.Now().UTC()

	t.Run("find active silences", func(t *testing.T) {
		db, repo := setupSilenceTest(t)
		defer db.Close()
		// Create active silence
		active := &entity.SilenceMark{
			ID:             "active-1",
			AlertID:        "alert-1",
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		// Create expired silence
		expired := &entity.SilenceMark{
			ID:             "expired-1",
			AlertID:        "alert-2",
			StartAt:        now.Add(-2 * time.Hour),
			EndAt:          now.Add(-1 * time.Hour),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		// Create pending silence
		pending := &entity.SilenceMark{
			ID:             "pending-1",
			AlertID:        "alert-3",
			StartAt:        now.Add(1 * time.Hour),
			EndAt:          now.Add(2 * time.Hour),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		require.NoError(t, repo.Save(context.Background(), active))
		require.NoError(t, repo.Save(context.Background(), expired))
		require.NoError(t, repo.Save(context.Background(), pending))

		// Query active silences
		actives, err := repo.FindActive(context.Background())
		require.NoError(t, err)
		require.Len(t, actives, 1)
		assert.Equal(t, "active-1", actives[0].ID)
	})

	t.Run("find active when none exist", func(t *testing.T) {
		db2, repo2 := setupSilenceTest(t)
		defer db2.Close()

		actives, err := repo2.FindActive(context.Background())
		assert.NoError(t, err)
		assert.Empty(t, actives)
	})
}

func TestSilenceRepository_FindByAlertID(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	now := time.Now().UTC()
	alertID := "test-alert"

	t.Run("find active silences for alert", func(t *testing.T) {
		// Create active silence for alert
		active := &entity.SilenceMark{
			ID:             "alert-silence-1",
			AlertID:        alertID,
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		// Create expired silence for same alert
		expired := &entity.SilenceMark{
			ID:             "alert-silence-2",
			AlertID:        alertID,
			StartAt:        now.Add(-2 * time.Hour),
			EndAt:          now.Add(-1 * time.Hour),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		require.NoError(t, repo.Save(context.Background(), active))
		require.NoError(t, repo.Save(context.Background(), expired))

		// Query active silences
		silences, err := repo.FindByAlertID(context.Background(), alertID)
		require.NoError(t, err)
		require.Len(t, silences, 1)
		assert.Equal(t, "alert-silence-1", silences[0].ID)
	})
}

func TestSilenceRepository_FindByInstance(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	now := time.Now().UTC()
	instance := "localhost:9090"

	t.Run("find active silences for instance", func(t *testing.T) {
		active := &entity.SilenceMark{
			ID:             "instance-silence-1",
			Instance:       instance,
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		require.NoError(t, repo.Save(context.Background(), active))

		silences, err := repo.FindByInstance(context.Background(), instance)
		require.NoError(t, err)
		require.Len(t, silences, 1)
		assert.Equal(t, instance, silences[0].Instance)
	})
}

func TestSilenceRepository_FindByFingerprint(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	now := time.Now().UTC()
	fingerprint := "fp-test-123"

	t.Run("find active silences for fingerprint", func(t *testing.T) {
		active := &entity.SilenceMark{
			ID:             "fp-silence-1",
			Fingerprint:    fingerprint,
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		require.NoError(t, repo.Save(context.Background(), active))

		silences, err := repo.FindByFingerprint(context.Background(), fingerprint)
		require.NoError(t, err)
		require.Len(t, silences, 1)
		assert.Equal(t, fingerprint, silences[0].Fingerprint)
	})
}

func TestSilenceRepository_FindMatchingAlert(t *testing.T) {
	now := time.Now().UTC()

	alert := &entity.Alert{
		ID:          "alert-match",
		Fingerprint: "fp-match",
		Instance:    "instance-match",
		Labels:      map[string]string{"env": "prod", "app": "web"},
	}

	t.Run("match by alert ID", func(t *testing.T) {
		db, repo := setupSilenceTest(t)
		defer db.Close()
		silence := &entity.SilenceMark{
			ID:             "match-alert-id",
			AlertID:        alert.ID,
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		require.NoError(t, repo.Save(context.Background(), silence))

		matches, err := repo.FindMatchingAlert(context.Background(), alert)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, silence.ID, matches[0].ID)
	})

	t.Run("match by fingerprint", func(t *testing.T) {
		db2, repo2 := setupSilenceTest(t)
		defer db2.Close()

		silence := &entity.SilenceMark{
			ID:             "match-fingerprint",
			Fingerprint:    alert.Fingerprint,
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		require.NoError(t, repo2.Save(context.Background(), silence))

		matches, err := repo2.FindMatchingAlert(context.Background(), alert)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, silence.ID, matches[0].ID)
	})

	t.Run("match by instance with labels", func(t *testing.T) {
		db2, repo2 := setupSilenceTest(t)
		defer db2.Close()

		silence := &entity.SilenceMark{
			ID:             "match-instance-labels",
			Instance:       alert.Instance,
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"env": "prod"},
			CreatedAt:      now,
		}

		require.NoError(t, repo2.Save(context.Background(), silence))

		matches, err := repo2.FindMatchingAlert(context.Background(), alert)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, silence.ID, matches[0].ID)
	})

	t.Run("match by labels only", func(t *testing.T) {
		db2, repo2 := setupSilenceTest(t)
		defer db2.Close()

		silence := &entity.SilenceMark{
			ID:             "match-labels-only",
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"env": "prod"},
			CreatedAt:      now,
		}

		require.NoError(t, repo2.Save(context.Background(), silence))

		matches, err := repo2.FindMatchingAlert(context.Background(), alert)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, silence.ID, matches[0].ID)
	})

	t.Run("no match when labels don't match", func(t *testing.T) {
		db, repo := setupSilenceTest(t)
		defer db.Close()

		silence := &entity.SilenceMark{
			ID:             "no-match-labels",
			Instance:       alert.Instance,
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"env": "staging"}, // Different value
			CreatedAt:      now,
		}

		require.NoError(t, repo.Save(context.Background(), silence))

		matches, err := repo.FindMatchingAlert(context.Background(), alert)
		require.NoError(t, err)
		assert.Empty(t, matches)
	})
}

func TestSilenceRepository_Update(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	now := time.Now().UTC()

	t.Run("update existing silence", func(t *testing.T) {
		silence := &entity.SilenceMark{
			ID:             "update-1",
			AlertID:        "alert-1",
			StartAt:        now,
			EndAt:          now.Add(1 * time.Hour),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Reason:         "Original reason",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"env": "prod"},
			CreatedAt:      now,
		}

		err := repo.Save(context.Background(), silence)
		require.NoError(t, err)

		// Update silence
		silence.Reason = "Updated reason"
		silence.EndAt = now.Add(2 * time.Hour)

		err = repo.Update(context.Background(), silence)
		assert.NoError(t, err)

		// Verify update
		updated, err := repo.FindByID(context.Background(), silence.ID)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, "Updated reason", updated.Reason)
	})

	t.Run("update nonexistent silence", func(t *testing.T) {
		silence := &entity.SilenceMark{
			ID:             "nonexistent",
			StartAt:        now,
			EndAt:          now.Add(1 * time.Hour),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		err := repo.Update(context.Background(), silence)
		assert.ErrorIs(t, err, entity.ErrSilenceNotFound)
	})
}

func TestSilenceRepository_Delete(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	now := time.Now().UTC()

	t.Run("delete existing silence", func(t *testing.T) {
		silence := &entity.SilenceMark{
			ID:             "delete-1",
			AlertID:        "alert-1",
			StartAt:        now,
			EndAt:          now.Add(1 * time.Hour),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}

		err := repo.Save(context.Background(), silence)
		require.NoError(t, err)

		// Delete
		err = repo.Delete(context.Background(), silence.ID)
		assert.NoError(t, err)

		// Verify deleted
		found, err := repo.FindByID(context.Background(), silence.ID)
		assert.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("delete nonexistent silence", func(t *testing.T) {
		err := repo.Delete(context.Background(), "nonexistent")
		assert.ErrorIs(t, err, entity.ErrSilenceNotFound)
	})
}

func TestSilenceRepository_DeleteExpired(t *testing.T) {
	db, repo := setupSilenceTest(t)
	defer db.Close()

	now := time.Now().UTC()

	t.Run("delete expired silences", func(t *testing.T) {
		// Create expired silences
		for i := 0; i < 3; i++ {
			silence := &entity.SilenceMark{
				ID:             fmt.Sprintf("expired-%d", i),
				AlertID:        fmt.Sprintf("alert-%d", i),
				StartAt:        now.Add(-2 * time.Hour),
				EndAt:          now.Add(-1 * time.Hour),
				CreatedBy:      "user",
				CreatedByEmail: "user@example.com",
				Source:         entity.AckSourceSlack,
				Labels:         map[string]string{},
				CreatedAt:      now,
			}
			require.NoError(t, repo.Save(context.Background(), silence))
		}

		// Create active silence
		active := &entity.SilenceMark{
			ID:             "active",
			AlertID:        "alert-active",
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "user",
			CreatedByEmail: "user@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{},
			CreatedAt:      now,
		}
		require.NoError(t, repo.Save(context.Background(), active))

		// Delete expired
		count, err := repo.DeleteExpired(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify active still exists
		found, err := repo.FindByID(context.Background(), "active")
		require.NoError(t, err)
		require.NotNil(t, found)

		// Verify expired are deleted
		for i := 0; i < 3; i++ {
			found, err := repo.FindByID(context.Background(), fmt.Sprintf("expired-%d", i))
			assert.NoError(t, err)
			assert.Nil(t, found)
		}
	})

	t.Run("delete expired when none exist", func(t *testing.T) {
		db2, repo2 := setupSilenceTest(t)
		defer db2.Close()

		count, err := repo2.DeleteExpired(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
