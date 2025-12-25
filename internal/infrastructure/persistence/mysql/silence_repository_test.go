package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
)

// Helper function to create a test silence
func createTestSilence(t *testing.T, duration time.Duration) *entity.SilenceMark {
	silence, err := entity.NewSilenceMark(
		duration,
		"user@example.com",
		"user@example.com",
		entity.AckSourceSlack,
	)
	require.NoError(t, err)
	silence.WithReason("Test silence")
	return silence
}

func TestSilenceRepository_Save(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	silence := createTestSilence(t, 1*time.Hour)
	silence.ForInstance("server-1")

	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Verify silence was saved
	saved, err := repo.FindByID(ctx, silence.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	assert.Equal(t, silence.ID, saved.ID)
	assert.Equal(t, silence.Instance, saved.Instance)
	assert.Equal(t, silence.Reason, saved.Reason)
}

func TestSilenceRepository_Save_WithLabels(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	silence := createTestSilence(t, 1*time.Hour)
	silence.WithLabel("env", "production")
	silence.WithLabel("team", "platform")

	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Verify labels were saved
	saved, err := repo.FindByID(ctx, silence.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	assert.Equal(t, "production", saved.Labels["env"])
	assert.Equal(t, "platform", saved.Labels["team"])
}

func TestSilenceRepository_FindByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	silence := createTestSilence(t, 1*time.Hour)
	silence.ForFingerprint("test-fingerprint")

	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Find by ID should return the silence
	found, err := repo.FindByID(ctx, silence.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, silence.ID, found.ID)
	assert.Equal(t, "test-fingerprint", found.Fingerprint)
}

func TestSilenceRepository_FindByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Finding non-existent silence should return nil
	found, err := repo.FindByID(ctx, "non-existent-id")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSilenceRepository_FindActive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	// Create active silence (started, not expired)
	activeSilence := createTestSilence(t, 1*time.Hour)
	activeSilence.StartAt = time.Now().UTC().Add(-10 * time.Minute) // Started 10 min ago
	activeSilence.EndAt = time.Now().UTC().Add(50 * time.Minute)    // Ends in 50 min

	err := repo.Save(ctx, activeSilence)
	require.NoError(t, err)

	// Create expired silence
	expiredSilence := createTestSilence(t, 1*time.Hour)
	expiredSilence.StartAt = time.Now().UTC().Add(-2 * time.Hour)
	expiredSilence.EndAt = time.Now().UTC().Add(-1 * time.Hour)

	err = repo.Save(ctx, expiredSilence)
	require.NoError(t, err)

	// Create pending silence (not started yet)
	pendingSilence := createTestSilence(t, 1*time.Hour)
	pendingSilence.StartAt = time.Now().UTC().Add(10 * time.Minute)
	pendingSilence.EndAt = time.Now().UTC().Add(70 * time.Minute)

	err = repo.Save(ctx, pendingSilence)
	require.NoError(t, err)

	// FindActive should return only the active silence
	active, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, activeSilence.ID, active[0].ID)
}

func TestSilenceRepository_FindByAlertID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test alert
	alert := createTestAlertForAck(t, db)

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Create active silence for the alert
	silence := createTestSilence(t, 1*time.Hour)
	silence.ForAlert(alert.ID)

	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Find by alert ID should return the silence
	silences, err := repo.FindByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	assert.Len(t, silences, 1)
	assert.Equal(t, silence.ID, silences[0].ID)
}

func TestSilenceRepository_FindByInstance(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	// Create active silence for instance
	silence := createTestSilence(t, 1*time.Hour)
	silence.ForInstance("server-1")

	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Find by instance should return the silence
	silences, err := repo.FindByInstance(ctx, "server-1")
	require.NoError(t, err)
	assert.Len(t, silences, 1)
	assert.Equal(t, silence.ID, silences[0].ID)
}

func TestSilenceRepository_FindByFingerprint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	// Create active silence for fingerprint
	silence := createTestSilence(t, 1*time.Hour)
	silence.ForFingerprint("test-fp")

	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Find by fingerprint should return the silence
	silences, err := repo.FindByFingerprint(ctx, "test-fp")
	require.NoError(t, err)
	assert.Len(t, silences, 1)
	assert.Equal(t, silence.ID, silences[0].ID)
}

func TestSilenceRepository_FindMatchingAlert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	// Create test alert
	alert := entity.NewAlert(
		"test-fp",
		"TestAlert",
		"server-1",
		"http://example.com",
		"Summary",
		entity.SeverityCritical,
	)
	alert.AddLabel("env", "production")
	alert.AddLabel("team", "platform")

	// Create silence matching by fingerprint
	silenceByFP := createTestSilence(t, 1*time.Hour)
	silenceByFP.ForFingerprint("test-fp")
	err := repo.Save(ctx, silenceByFP)
	require.NoError(t, err)

	// Create silence matching by instance
	silenceByInstance := createTestSilence(t, 1*time.Hour)
	silenceByInstance.ForInstance("server-1")
	err = repo.Save(ctx, silenceByInstance)
	require.NoError(t, err)

	// Create silence matching by labels
	silenceByLabels := createTestSilence(t, 1*time.Hour)
	silenceByLabels.WithLabel("env", "production")
	err = repo.Save(ctx, silenceByLabels)
	require.NoError(t, err)

	// Create silence that doesn't match
	nonMatchingSilence := createTestSilence(t, 1*time.Hour)
	nonMatchingSilence.ForInstance("server-2")
	err = repo.Save(ctx, nonMatchingSilence)
	require.NoError(t, err)

	// FindMatchingAlert should return all matching silences
	matching, err := repo.FindMatchingAlert(ctx, alert)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(matching), 3, "Should match at least 3 silences")

	// Verify the non-matching silence is not included
	for _, s := range matching {
		assert.NotEqual(t, nonMatchingSilence.ID, s.ID)
	}
}

func TestSilenceRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	silence := createTestSilence(t, 1*time.Hour)
	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Modify the silence
	silence.Reason = "Updated reason"
	silence.EndAt = silence.EndAt.Add(30 * time.Minute)

	// Update should succeed
	err = repo.Update(ctx, silence)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.FindByID(ctx, silence.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Updated reason", updated.Reason)
}

func TestSilenceRepository_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	silence := createTestSilence(t, 1*time.Hour)

	// Update non-existent silence should fail with ErrNotFound
	err := repo.Update(ctx, silence)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestSilenceRepository_Update_ConcurrentUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	silence := createTestSilence(t, 1*time.Hour)
	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Simulate concurrent update by updating version directly
	_, err = db.Primary().Exec("UPDATE silences SET version = version + 1 WHERE id = ?", silence.ID)
	require.NoError(t, err)

	// Now try to update with stale version - should fail with ErrConcurrentUpdate
	silence.Reason = "Modified reason"
	err = repo.Update(ctx, silence)
	assert.ErrorIs(t, err, repository.ErrConcurrentUpdate)
}

func TestSilenceRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	silence := createTestSilence(t, 1*time.Hour)
	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Delete should succeed
	err = repo.Delete(ctx, silence.ID)
	require.NoError(t, err)

	// Silence should not exist
	found, err := repo.FindByID(ctx, silence.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSilenceRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Delete non-existent silence should fail with ErrNotFound
	err := repo.Delete(ctx, "non-existent-id")
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestSilenceRepository_DeleteExpired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	// Create active silence
	activeSilence := createTestSilence(t, 1*time.Hour)
	err := repo.Save(ctx, activeSilence)
	require.NoError(t, err)

	// Create expired silences
	expiredSilence1 := createTestSilence(t, 1*time.Hour)
	expiredSilence1.StartAt = time.Now().UTC().Add(-2 * time.Hour)
	expiredSilence1.EndAt = time.Now().UTC().Add(-1 * time.Hour)
	err = repo.Save(ctx, expiredSilence1)
	require.NoError(t, err)

	expiredSilence2 := createTestSilence(t, 1*time.Hour)
	expiredSilence2.StartAt = time.Now().UTC().Add(-3 * time.Hour)
	expiredSilence2.EndAt = time.Now().UTC().Add(-2 * time.Hour)
	err = repo.Save(ctx, expiredSilence2)
	require.NoError(t, err)

	// DeleteExpired should remove 2 silences
	count, err := repo.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Active silence should still exist
	found, err := repo.FindByID(ctx, activeSilence.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	// Expired silences should be gone
	expired1, err := repo.FindByID(ctx, expiredSilence1.ID)
	require.NoError(t, err)
	assert.Nil(t, expired1)

	expired2, err := repo.FindByID(ctx, expiredSilence2.ID)
	require.NoError(t, err)
	assert.Nil(t, expired2)
}

func TestSilenceRepository_TimeBasedFiltering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	now := time.Now().UTC()

	// Create silences with different time windows
	active := createTestSilence(t, 1*time.Hour)
	active.StartAt = now.Add(-10 * time.Minute)
	active.EndAt = now.Add(50 * time.Minute)
	active.ForInstance("server-active")
	err := repo.Save(ctx, active)
	require.NoError(t, err)

	pending := createTestSilence(t, 1*time.Hour)
	pending.StartAt = now.Add(10 * time.Minute)
	pending.EndAt = now.Add(70 * time.Minute)
	pending.ForInstance("server-pending")
	err = repo.Save(ctx, pending)
	require.NoError(t, err)

	expired := createTestSilence(t, 1*time.Hour)
	expired.StartAt = now.Add(-2 * time.Hour)
	expired.EndAt = now.Add(-1 * time.Hour)
	expired.ForInstance("server-expired")
	err = repo.Save(ctx, expired)
	require.NoError(t, err)

	// FindActive should only return the active silence
	activeSilences, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, activeSilences, 1)
	assert.Equal(t, "server-active", activeSilences[0].Instance)

	// FindByInstance for pending should return nothing (not active yet)
	pendingSilences, err := repo.FindByInstance(ctx, "server-pending")
	require.NoError(t, err)
	assert.Empty(t, pendingSilences)

	// FindByInstance for expired should return nothing
	expiredSilences, err := repo.FindByInstance(ctx, "server-expired")
	require.NoError(t, err)
	assert.Empty(t, expiredSilences)
}

func TestSilenceRepository_NullableFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSilenceRepository(db)
	ctx := context.Background()

	// Create silence with minimal fields (no optional targeting)
	silence := createTestSilence(t, 1*time.Hour)
	// Don't set AlertID, Instance, or Fingerprint

	err := repo.Save(ctx, silence)
	require.NoError(t, err)

	// Retrieve and verify nullable fields are empty
	saved, err := repo.FindByID(ctx, silence.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	assert.Empty(t, saved.AlertID)
	assert.Empty(t, saved.Instance)
	assert.Empty(t, saved.Fingerprint)
}

func TestNewSilenceRepository(t *testing.T) {
	db := &DB{}
	repo := NewSilenceRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}
