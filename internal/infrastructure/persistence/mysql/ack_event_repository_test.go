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

// Helper function to create a test alert for ack events
func createTestAlertForAck(t *testing.T, db *DB) *entity.Alert {
	alert := entity.NewAlert(
		"test-fingerprint",
		"TestAlert",
		"server-1",
		"http://example.com",
		"Test alert for ack events",
		entity.SeverityCritical,
	)

	repo := NewAlertRepository(db)
	ctx := context.Background()
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	return alert
}

func TestAckEventRepository_Save(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test alert first
	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Create ack event
	event := entity.NewAckEvent(
		alert.ID,
		entity.AckSourceSlack,
		"U12345",
		"user@example.com",
		"John Doe",
	)
	event.WithNote("Investigating the issue")

	err := repo.Save(ctx, event)
	require.NoError(t, err)

	// Verify event was saved
	saved, err := repo.FindByID(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	assert.Equal(t, event.ID, saved.ID)
	assert.Equal(t, event.AlertID, saved.AlertID)
	assert.Equal(t, event.Source, saved.Source)
	assert.Equal(t, event.UserID, saved.UserID)
	assert.Equal(t, event.UserEmail, saved.UserEmail)
	assert.Equal(t, event.UserName, saved.UserName)
	assert.Equal(t, event.Note, saved.Note)
}

func TestAckEventRepository_Save_WithDuration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Create ack event with duration
	duration := 15 * time.Minute
	event := entity.NewAckEvent(
		alert.ID,
		entity.AckSourcePagerDuty,
		"PD12345",
		"user@example.com",
		"Jane Smith",
	).WithDuration(duration)

	err := repo.Save(ctx, event)
	require.NoError(t, err)

	// Verify duration was saved correctly
	saved, err := repo.FindByID(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	require.NotNil(t, saved.Duration)
	assert.Equal(t, duration, *saved.Duration)
}

func TestAckEventRepository_Save_ForeignKeyConstraint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Try to save ack event for non-existent alert
	event := entity.NewAckEvent(
		"non-existent-alert-id",
		entity.AckSourceSlack,
		"U12345",
		"user@example.com",
		"John Doe",
	)

	err := repo.Save(ctx, event)
	assert.ErrorIs(t, err, repository.ErrNotFound, "Should fail with ErrNotFound due to FK constraint")
}

func TestAckEventRepository_FindByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	event := entity.NewAckEvent(
		alert.ID,
		entity.AckSourceAPI,
		"api-user",
		"api@example.com",
		"API User",
	).WithNote("Acknowledged via API")

	err := repo.Save(ctx, event)
	require.NoError(t, err)

	// Find by ID should return the event
	found, err := repo.FindByID(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, event.ID, found.ID)
	assert.Equal(t, "Acknowledged via API", found.Note)
}

func TestAckEventRepository_FindByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Finding non-existent event should return nil
	found, err := repo.FindByID(ctx, "non-existent-id")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestAckEventRepository_FindByAlertID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Create multiple ack events for the same alert with different timestamps
	event1 := entity.NewAckEvent(alert.ID, entity.AckSourceSlack, "U1", "user1@example.com", "User 1")
	event1.CreatedAt = time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	event2 := entity.NewAckEvent(alert.ID, entity.AckSourcePagerDuty, "PD1", "user2@example.com", "User 2")
	event2.CreatedAt = time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)

	event3 := entity.NewAckEvent(alert.ID, entity.AckSourceAPI, "API1", "user3@example.com", "User 3")
	event3.CreatedAt = time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)

	// Save in random order
	err := repo.Save(ctx, event2)
	require.NoError(t, err)
	err = repo.Save(ctx, event1)
	require.NoError(t, err)
	err = repo.Save(ctx, event3)
	require.NoError(t, err)

	// Find by alert ID should return all events in chronological order (oldest first)
	events, err := repo.FindByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	assert.Len(t, events, 3)

	// Verify chronological order
	assert.Equal(t, event3.ID, events[0].ID, "First should be event3 (9:00)")
	assert.Equal(t, event1.ID, events[1].ID, "Second should be event1 (10:00)")
	assert.Equal(t, event2.ID, events[2].ID, "Third should be event2 (11:00)")
}

func TestAckEventRepository_FindByAlertID_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Finding events for alert with no acks should return empty slice
	events, err := repo.FindByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestAckEventRepository_FindLatestByAlertID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Create multiple ack events
	event1 := entity.NewAckEvent(alert.ID, entity.AckSourceSlack, "U1", "user1@example.com", "User 1")
	event1.CreatedAt = time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	event2 := entity.NewAckEvent(alert.ID, entity.AckSourcePagerDuty, "PD1", "user2@example.com", "User 2")
	event2.CreatedAt = time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)

	event3 := entity.NewAckEvent(alert.ID, entity.AckSourceAPI, "API1", "user3@example.com", "User 3")
	event3.CreatedAt = time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)

	err := repo.Save(ctx, event1)
	require.NoError(t, err)
	err = repo.Save(ctx, event2)
	require.NoError(t, err)
	err = repo.Save(ctx, event3)
	require.NoError(t, err)

	// FindLatestByAlertID should return event2 (most recent)
	latest, err := repo.FindLatestByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, event2.ID, latest.ID)
	assert.Equal(t, "User 2", latest.UserName)
}

func TestAckEventRepository_FindLatestByAlertID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Finding latest for alert with no acks should return nil
	latest, err := repo.FindLatestByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	assert.Nil(t, latest)
}

func TestAckEventRepository_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	ackRepo := NewAckEventRepository(db)
	alertRepo := NewAlertRepository(db)
	ctx := context.Background()

	// Create ack event
	event := entity.NewAckEvent(
		alert.ID,
		entity.AckSourceSlack,
		"U12345",
		"user@example.com",
		"John Doe",
	)
	err := ackRepo.Save(ctx, event)
	require.NoError(t, err)

	// Verify ack event exists
	saved, err := ackRepo.FindByID(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	// Delete the alert
	err = alertRepo.Delete(ctx, alert.ID)
	require.NoError(t, err)

	// Ack event should be cascade deleted
	deleted, err := ackRepo.FindByID(ctx, event.ID)
	require.NoError(t, err)
	assert.Nil(t, deleted, "Ack event should be cascade deleted when alert is deleted")
}

func TestAckEventRepository_NullableFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Create event with minimal fields (no note, no duration)
	event := entity.NewAckEvent(
		alert.ID,
		entity.AckSourceSlack,
		"", // empty user ID
		"", // empty email
		"", // empty name
	)

	err := repo.Save(ctx, event)
	require.NoError(t, err)

	// Retrieve and verify nullable fields are empty
	saved, err := repo.FindByID(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	assert.Empty(t, saved.UserID)
	assert.Empty(t, saved.UserEmail)
	assert.Empty(t, saved.UserName)
	assert.Empty(t, saved.Note)
	assert.Nil(t, saved.Duration)
}

func TestAckEventRepository_DifferentSources(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	alert := createTestAlertForAck(t, db)

	repo := NewAckEventRepository(db)
	ctx := context.Background()

	// Create events from different sources
	slackEvent := entity.NewAckEvent(alert.ID, entity.AckSourceSlack, "U1", "slack@example.com", "Slack User")
	pdEvent := entity.NewAckEvent(alert.ID, entity.AckSourcePagerDuty, "PD1", "pd@example.com", "PD User")
	apiEvent := entity.NewAckEvent(alert.ID, entity.AckSourceAPI, "API1", "api@example.com", "API User")

	err := repo.Save(ctx, slackEvent)
	require.NoError(t, err)
	err = repo.Save(ctx, pdEvent)
	require.NoError(t, err)
	err = repo.Save(ctx, apiEvent)
	require.NoError(t, err)

	// Retrieve all events
	events, err := repo.FindByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	assert.Len(t, events, 3)

	// Verify sources are preserved
	sources := make(map[entity.AckSource]bool)
	for _, e := range events {
		sources[e.Source] = true
	}

	assert.True(t, sources[entity.AckSourceSlack])
	assert.True(t, sources[entity.AckSourcePagerDuty])
	assert.True(t, sources[entity.AckSourceAPI])
}

func TestNewAckEventRepository(t *testing.T) {
	db := &DB{}
	repo := NewAckEventRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}
