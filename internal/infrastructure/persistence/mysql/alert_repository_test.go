package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

// Helper function to create a test database connection
func setupTestDB(t *testing.T) *DB {
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
			ConnMaxLifetime: 3 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
		},
		Timeout:   5 * time.Second,
		ParseTime: true,
		Charset:   "utf8mb4",
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		db.Close()
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up existing data
	_, _ = db.Primary().Exec("DELETE FROM ack_events")
	_, _ = db.Primary().Exec("DELETE FROM alerts")
	_, _ = db.Primary().Exec("DELETE FROM silences")

	return db
}

// Helper function to create a test alert
func createTestAlert() *entity.Alert {
	alert := entity.NewAlert(
		"test-fingerprint",
		"TestAlert",
		"server-1",
		"http://example.com",
		"Test alert summary",
		entity.SeverityCritical,
	)
	alert.Description = "Test alert description"
	alert.AddLabel("env", "production")
	alert.AddLabel("team", "platform")
	alert.AddAnnotation("runbook", "https://runbook.example.com")
	return alert
}

func TestAlertRepository_Save(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()

	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Verify alert was saved
	saved, err := repo.FindByID(ctx, alert.ID)
	require.NoError(t, err)
	require.NotNil(t, saved)

	assert.Equal(t, alert.ID, saved.ID)
	assert.Equal(t, alert.Fingerprint, saved.Fingerprint)
	assert.Equal(t, alert.Name, saved.Name)
	assert.Equal(t, alert.Summary, saved.Summary)
	assert.Equal(t, alert.Description, saved.Description)
	assert.Equal(t, alert.Severity, saved.Severity)
	assert.Equal(t, alert.State, saved.State)
	assert.Equal(t, alert.Labels, saved.Labels)
	assert.Equal(t, alert.Annotations, saved.Annotations)
}

func TestAlertRepository_Save_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()

	// First save should succeed
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Second save should fail with ErrAlreadyExists
	err = repo.Save(ctx, alert)
	assert.ErrorIs(t, err, repository.ErrAlreadyExists)
}

func TestAlertRepository_FindByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Find by ID should return the alert
	found, err := repo.FindByID(ctx, alert.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, alert.ID, found.ID)
	assert.Equal(t, alert.Labels["env"], found.Labels["env"])
}

func TestAlertRepository_FindByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Finding non-existent alert should return nil
	found, err := repo.FindByID(ctx, "non-existent-id")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestAlertRepository_FindByFingerprint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Create multiple alerts with same fingerprint
	alert1 := createTestAlert()
	alert2 := createTestAlert()
	alert2.ID = "different-id"

	err := repo.Save(ctx, alert1)
	require.NoError(t, err)
	err = repo.Save(ctx, alert2)
	require.NoError(t, err)

	// Find by fingerprint should return both
	alerts, err := repo.FindByFingerprint(ctx, "test-fingerprint")
	require.NoError(t, err)
	assert.Len(t, alerts, 2)
}

func TestAlertRepository_FindBySlackMessageID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()
	alert.SetSlackMessageID("C123456:1234567890.123456")
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Find by Slack message ID
	found, err := repo.FindBySlackMessageID(ctx, "C123456:1234567890.123456")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, alert.ID, found.ID)
	assert.Equal(t, "C123456:1234567890.123456", found.SlackMessageID)
}

func TestAlertRepository_FindBySlackMessageID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	found, err := repo.FindBySlackMessageID(ctx, "non-existent")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestAlertRepository_FindByPagerDutyIncidentID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()
	alert.SetPagerDutyIncidentID("PD-12345")
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Find by PagerDuty incident ID
	found, err := repo.FindByPagerDutyIncidentID(ctx, "PD-12345")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, alert.ID, found.ID)
	assert.Equal(t, "PD-12345", found.PagerDutyIncidentID)
}

func TestAlertRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Modify the alert
	now := time.Now().UTC()
	err = alert.Acknowledge("user@example.com", now)
	require.NoError(t, err)

	// Update should succeed
	err = repo.Update(ctx, alert)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.FindByID(ctx, alert.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, entity.StateAcked, updated.State)
	assert.Equal(t, "user@example.com", updated.AckedBy)
	assert.NotNil(t, updated.AckedAt)
}

func TestAlertRepository_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()

	// Update non-existent alert should fail with ErrNotFound
	err := repo.Update(ctx, alert)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestAlertRepository_Update_ConcurrentUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Simulate concurrent update by updating directly in database
	_, err = db.Primary().Exec("UPDATE alerts SET version = version + 1 WHERE id = ?", alert.ID)
	require.NoError(t, err)

	// Now try to update with stale version - should fail with ErrConcurrentUpdate
	alert.Summary = "Modified summary"
	err = repo.Update(ctx, alert)
	assert.ErrorIs(t, err, repository.ErrConcurrentUpdate)
}

func TestAlertRepository_FindActive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Create active alert
	activeAlert := createTestAlert()
	err := repo.Save(ctx, activeAlert)
	require.NoError(t, err)

	// Create acknowledged alert
	ackedAlert := createTestAlert()
	ackedAlert.ID = "acked-alert"
	now := time.Now().UTC()
	_ = ackedAlert.Acknowledge("user@example.com", now)
	err = repo.Save(ctx, ackedAlert)
	require.NoError(t, err)

	// Create resolved alert
	resolvedAlert := createTestAlert()
	resolvedAlert.ID = "resolved-alert"
	resolvedAlert.Resolve(now)
	err = repo.Save(ctx, resolvedAlert)
	require.NoError(t, err)

	// FindActive should return only active and acknowledged (not resolved)
	active, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, active, 2)

	// Verify it doesn't include resolved alert
	for _, a := range active {
		assert.NotEqual(t, "resolved-alert", a.ID)
	}
}

func TestAlertRepository_FindFiring(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Create active alert
	activeAlert := createTestAlert()
	err := repo.Save(ctx, activeAlert)
	require.NoError(t, err)

	// Create acknowledged alert
	ackedAlert := createTestAlert()
	ackedAlert.ID = "acked-alert"
	now := time.Now().UTC()
	_ = ackedAlert.Acknowledge("user@example.com", now)
	err = repo.Save(ctx, ackedAlert)
	require.NoError(t, err)

	// Create resolved alert
	resolvedAlert := createTestAlert()
	resolvedAlert.ID = "resolved-alert"
	resolvedAlert.Resolve(now)
	err = repo.Save(ctx, resolvedAlert)
	require.NoError(t, err)

	// FindFiring should return only active and acknowledged (not resolved)
	firing, err := repo.FindFiring(ctx)
	require.NoError(t, err)
	assert.Len(t, firing, 2)

	// Verify it includes active and acknowledged
	ids := make(map[string]bool)
	for _, a := range firing {
		ids[a.ID] = true
	}
	assert.True(t, ids[activeAlert.ID])
	assert.True(t, ids[ackedAlert.ID])
	assert.False(t, ids["resolved-alert"])
}

func TestAlertRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()
	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Delete should succeed
	err = repo.Delete(ctx, alert.ID)
	require.NoError(t, err)

	// Alert should not exist
	found, err := repo.FindByID(ctx, alert.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestAlertRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Delete non-existent alert should fail with ErrNotFound
	err := repo.Delete(ctx, "non-existent-id")
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestAlertRepository_JSONFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	alert := createTestAlert()
	alert.AddLabel("key1", "value1")
	alert.AddLabel("key2", "value2")
	alert.AddAnnotation("anno1", "annotation1")
	alert.AddAnnotation("anno2", "annotation2")

	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Retrieve and verify JSON fields
	found, err := repo.FindByID(ctx, alert.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, "value1", found.Labels["key1"])
	assert.Equal(t, "value2", found.Labels["key2"])
	assert.Equal(t, "annotation1", found.Annotations["anno1"])
	assert.Equal(t, "annotation2", found.Annotations["anno2"])
}

func TestAlertRepository_NullableFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Create alert without nullable fields set
	alert := createTestAlert()

	err := repo.Save(ctx, alert)
	require.NoError(t, err)

	// Retrieve and verify nullable fields are nil/empty
	found, err := repo.FindByID(ctx, alert.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Empty(t, found.SlackMessageID)
	assert.Empty(t, found.PagerDutyIncidentID)
	assert.Empty(t, found.AckedBy)
	assert.Nil(t, found.AckedAt)
	assert.Nil(t, found.ResolvedAt)

	// Update with nullable fields
	now := time.Now().UTC()
	_ = found.Acknowledge("user@example.com", now)
	found.SetSlackMessageID("C123:456")
	found.SetPagerDutyIncidentID("PD-789")

	err = repo.Update(ctx, found)
	require.NoError(t, err)

	// Retrieve and verify nullable fields are set
	updated, err := repo.FindByID(ctx, found.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "C123:456", updated.SlackMessageID)
	assert.Equal(t, "PD-789", updated.PagerDutyIncidentID)
	assert.Equal(t, "user@example.com", updated.AckedBy)
	assert.NotNil(t, updated.AckedAt)
}

func TestNewAlertRepository(t *testing.T) {
	db := &DB{}
	repo := NewAlertRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}
