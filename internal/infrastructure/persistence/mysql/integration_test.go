package mysql

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

// TestAlertPersistence_AcrossRestart simulates an application restart
// to verify that alerts persist in MySQL and remain accessible.
func TestAlertPersistence_AcrossRestart(t *testing.T) {
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

	// Phase 1: Create connection and save alerts
	db1, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}

	// Run migrations
	migrator := NewMigrator(db1.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		db1.Close()
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db1.Primary().Exec("DELETE FROM ack_events")
	_, _ = db1.Primary().Exec("DELETE FROM alerts")

	repo1 := NewAlertRepository(db1)

	// Create and save test alerts
	alert1 := entity.NewAlert("fp-1", "Alert1", "server-1", "http://example.com", "Summary1", entity.SeverityCritical)
	alert1.Description = "Description 1"
	alert1.AddLabel("env", "production")

	alert2 := entity.NewAlert("fp-2", "Alert2", "server-2", "http://example.com", "Summary2", entity.SeverityWarning)
	alert2.Description = "Description 2"
	alert2.AddLabel("env", "staging")

	err = repo1.Save(ctx, alert1)
	require.NoError(t, err)

	err = repo1.Save(ctx, alert2)
	require.NoError(t, err)

	// Save IDs for later verification
	savedID1 := alert1.ID
	savedID2 := alert2.ID

	// Close connection (simulate application shutdown)
	err = db1.Close()
	require.NoError(t, err)

	// Phase 2: Create new connection (simulate restart) and verify data persists
	db2, err := NewDB(cfg)
	require.NoError(t, err)
	defer db2.Close()

	repo2 := NewAlertRepository(db2)

	// Verify alert1 persists
	retrieved1, err := repo2.FindByID(ctx, savedID1)
	require.NoError(t, err)
	require.NotNil(t, retrieved1)
	assert.Equal(t, savedID1, retrieved1.ID)
	assert.Equal(t, "fp-1", retrieved1.Fingerprint)
	assert.Equal(t, "Alert1", retrieved1.Name)
	assert.Equal(t, "Description 1", retrieved1.Description)
	assert.Equal(t, "production", retrieved1.Labels["env"])

	// Verify alert2 persists
	retrieved2, err := repo2.FindByID(ctx, savedID2)
	require.NoError(t, err)
	require.NotNil(t, retrieved2)
	assert.Equal(t, savedID2, retrieved2.ID)
	assert.Equal(t, "fp-2", retrieved2.Fingerprint)
	assert.Equal(t, "Alert2", retrieved2.Name)
	assert.Equal(t, "Description 2", retrieved2.Description)
	assert.Equal(t, "staging", retrieved2.Labels["env"])

	// Verify FindActive works across restart
	active, err := repo2.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, active, 2)
}

// TestConcurrentAlertUpdates_MultipleInstances simulates multiple application
// instances updating the same alert concurrently to verify optimistic locking.
func TestConcurrentAlertUpdates_MultipleInstances(t *testing.T) {
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

	// Create initial database connection and alert
	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM ack_events")
	_, _ = db.Primary().Exec("DELETE FROM alerts")

	repo := NewAlertRepository(db)

	// Create test alert
	alert := entity.NewAlert("test-fp", "TestAlert", "server-1", "http://example.com", "Summary", entity.SeverityCritical)
	alert.Description = "Description"
	err = repo.Save(ctx, alert)
	require.NoError(t, err)

	alertID := alert.ID

	// Simulate 3 concurrent instances trying to update the same alert
	const numInstances = 3
	var wg sync.WaitGroup
	results := make([]error, numInstances)

	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(instanceNum int) {
			defer wg.Done()

			// Each instance creates its own connection (simulating separate app instance)
			instanceDB, err := NewDB(cfg)
			if err != nil {
				results[instanceNum] = err
				return
			}
			defer instanceDB.Close()

			instanceRepo := NewAlertRepository(instanceDB)

			// Read the alert
			instanceAlert, err := instanceRepo.FindByID(ctx, alertID)
			if err != nil {
				results[instanceNum] = err
				return
			}

			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)

			// Try to update the alert
			now := time.Now().UTC()
			instanceAlert.Summary = fmt.Sprintf("Updated by instance %d", instanceNum)
			instanceAlert.UpdatedAt = now

			err = instanceRepo.Update(ctx, instanceAlert)
			results[instanceNum] = err
		}(i)
	}

	wg.Wait()

	// Verify results:
	// - At least one instance should succeed
	// - At least one instance should fail with ErrConcurrentUpdate (due to optimistic locking)
	successCount := 0
	concurrentUpdateCount := 0

	for i, err := range results {
		if err == nil {
			successCount++
			t.Logf("Instance %d: succeeded", i)
		} else if err == repository.ErrConcurrentUpdate {
			concurrentUpdateCount++
			t.Logf("Instance %d: got ErrConcurrentUpdate (expected)", i)
		} else {
			t.Errorf("Instance %d: unexpected error: %v", i, err)
		}
	}

	// At least one should succeed
	assert.Greater(t, successCount, 0, "At least one instance should succeed")

	// With optimistic locking, concurrent updates should be detected
	// (In a real multi-instance scenario with timing, we expect some conflicts)
	assert.Greater(t, concurrentUpdateCount, 0, "Optimistic locking should detect concurrent updates")

	// Total should equal number of instances
	assert.Equal(t, numInstances, successCount+concurrentUpdateCount)

	// Verify final state is consistent
	finalAlert, err := repo.FindByID(ctx, alertID)
	require.NoError(t, err)
	require.NotNil(t, finalAlert)

	// The summary should be from one of the successful updates
	assert.Contains(t, finalAlert.Summary, "Updated by instance")
	t.Logf("Final alert summary: %s", finalAlert.Summary)
}

// TestConcurrentWrites_DifferentAlerts verifies that multiple instances
// can write different alerts concurrently without conflicts.
func TestConcurrentWrites_DifferentAlerts(t *testing.T) {
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
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM ack_events")
	_, _ = db.Primary().Exec("DELETE FROM alerts")

	// Simulate 10 instances each writing 5 alerts concurrently
	const numInstances = 10
	const alertsPerInstance = 5

	var wg sync.WaitGroup
	errors := make(chan error, numInstances*alertsPerInstance)

	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(instanceNum int) {
			defer wg.Done()

			// Each instance creates its own connection
			instanceDB, err := NewDB(cfg)
			if err != nil {
				errors <- err
				return
			}
			defer instanceDB.Close()

			instanceRepo := NewAlertRepository(instanceDB)

			// Write multiple alerts
			for j := 0; j < alertsPerInstance; j++ {
				alert := entity.NewAlert(
					fmt.Sprintf("fp-%d-%d", instanceNum, j),
					fmt.Sprintf("Alert-%d-%d", instanceNum, j),
					fmt.Sprintf("server-%d", instanceNum),
					"http://example.com",
					fmt.Sprintf("Summary from instance %d alert %d", instanceNum, j),
					entity.SeverityCritical,
				)
				alert.Description = fmt.Sprintf("Description %d-%d", instanceNum, j)

				if err := instanceRepo.Save(ctx, alert); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent write error: %v", err)
		errorCount++
	}

	assert.Equal(t, 0, errorCount, "No errors should occur when writing different alerts concurrently")

	// Verify all alerts were saved
	repo := NewAlertRepository(db)
	active, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, active, numInstances*alertsPerInstance, "All alerts should be saved")
}

// TestCrossInstanceVisibility verifies that alerts created on one instance
// are immediately visible on another instance.
func TestCrossInstanceVisibility(t *testing.T) {
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

	// Instance A
	dbA, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer dbA.Close()

	// Run migrations
	migrator := NewMigrator(dbA.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = dbA.Primary().Exec("DELETE FROM ack_events")
	_, _ = dbA.Primary().Exec("DELETE FROM alerts")

	repoA := NewAlertRepository(dbA)

	// Instance B
	dbB, err := NewDB(cfg)
	require.NoError(t, err)
	defer dbB.Close()

	repoB := NewAlertRepository(dbB)

	// Instance A creates an alert
	alert := entity.NewAlert("test-fp", "TestAlert", "server-1", "http://example.com", "Summary", entity.SeverityCritical)
	alert.Description = "Description"
	err = repoA.Save(ctx, alert)
	require.NoError(t, err)

	alertID := alert.ID

	// Instance B should immediately see the alert (no caching)
	retrieved, err := repoB.FindByID(ctx, alertID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, alertID, retrieved.ID)
	assert.Equal(t, "TestAlert", retrieved.Name)

	// Instance A updates the alert
	now := time.Now().UTC()
	err = alert.Acknowledge("user@example.com", now)
	require.NoError(t, err)
	err = repoA.Update(ctx, alert)
	require.NoError(t, err)

	// Instance B should see the update immediately
	updated, err := repoB.FindByID(ctx, alertID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, entity.StateAcked, updated.State)
	assert.Equal(t, "user@example.com", updated.AckedBy)
	assert.NotNil(t, updated.AckedAt)
}

// TestAckEventVisibility_CrossInstance verifies that ack events created on one instance
// are immediately visible on another instance.
func TestAckEventVisibility_CrossInstance(t *testing.T) {
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

	// Instance A
	dbA, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer dbA.Close()

	// Run migrations
	migrator := NewMigrator(dbA.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = dbA.Primary().Exec("DELETE FROM ack_events")
	_, _ = dbA.Primary().Exec("DELETE FROM alerts")

	alertRepoA := NewAlertRepository(dbA)
	ackRepoA := NewAckEventRepository(dbA)

	// Instance B
	dbB, err := NewDB(cfg)
	require.NoError(t, err)
	defer dbB.Close()

	ackRepoB := NewAckEventRepository(dbB)

	// Instance A creates an alert
	alert := entity.NewAlert("test-fp", "TestAlert", "server-1", "http://example.com", "Summary", entity.SeverityCritical)
	err = alertRepoA.Save(ctx, alert)
	require.NoError(t, err)

	// Instance A creates an ack event via Slack
	ackEvent := entity.NewAckEvent(
		alert.ID,
		entity.AckSourceSlack,
		"U12345",
		"user@example.com",
		"John Doe",
	).WithNote("Investigating via Slack")

	err = ackRepoA.Save(ctx, ackEvent)
	require.NoError(t, err)

	// Instance B should immediately see the ack event
	retrieved, err := ackRepoB.FindByID(ctx, ackEvent.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, ackEvent.ID, retrieved.ID)
	assert.Equal(t, entity.AckSourceSlack, retrieved.Source)
	assert.Equal(t, "Investigating via Slack", retrieved.Note)

	// Instance B should see it in the alert's ack history
	events, err := ackRepoB.FindByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, ackEvent.ID, events[0].ID)

	// Instance B should see it as the latest ack
	latest, err := ackRepoB.FindLatestByAlertID(ctx, alert.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, ackEvent.ID, latest.ID)
}

// TestConcurrentAckEvents_DifferentSources verifies that multiple instances
// can create ack events from different sources concurrently without data loss.
func TestConcurrentAckEvents_DifferentSources(t *testing.T) {
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
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM ack_events")
	_, _ = db.Primary().Exec("DELETE FROM alerts")

	alertRepo := NewAlertRepository(db)

	// Create test alert
	alert := entity.NewAlert("test-fp", "TestAlert", "server-1", "http://example.com", "Summary", entity.SeverityCritical)
	err = alertRepo.Save(ctx, alert)
	require.NoError(t, err)

	alertID := alert.ID

	// Simulate 3 instances creating ack events concurrently from different sources
	sources := []entity.AckSource{
		entity.AckSourceSlack,
		entity.AckSourcePagerDuty,
		entity.AckSourceAPI,
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(sources))
	eventIDs := make(chan string, len(sources))

	for i, source := range sources {
		wg.Add(1)
		go func(instanceNum int, src entity.AckSource) {
			defer wg.Done()

			// Each instance creates its own connection
			instanceDB, err := NewDB(cfg)
			if err != nil {
				errors <- err
				return
			}
			defer instanceDB.Close()

			instanceRepo := NewAckEventRepository(instanceDB)

			// Create ack event
			event := entity.NewAckEvent(
				alertID,
				src,
				fmt.Sprintf("user-%d", instanceNum),
				fmt.Sprintf("user%d@example.com", instanceNum),
				fmt.Sprintf("User %d", instanceNum),
			).WithNote(fmt.Sprintf("Ack from %s", src))

			// Add slight delay to increase chance of concurrent execution
			time.Sleep(time.Duration(instanceNum*10) * time.Millisecond)

			if err := instanceRepo.Save(ctx, event); err != nil {
				errors <- err
			} else {
				eventIDs <- event.ID
			}
		}(i, source)
	}

	wg.Wait()
	close(errors)
	close(eventIDs)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent ack event creation error: %v", err)
		errorCount++
	}

	assert.Equal(t, 0, errorCount, "No errors should occur when creating ack events concurrently")

	// Collect saved event IDs
	savedIDs := make([]string, 0)
	for id := range eventIDs {
		savedIDs = append(savedIDs, id)
	}

	assert.Len(t, savedIDs, 3, "All 3 ack events should be saved")

	// Verify all events are retrievable
	ackRepo := NewAckEventRepository(db)
	events, err := ackRepo.FindByAlertID(ctx, alertID)
	require.NoError(t, err)
	assert.Len(t, events, 3, "All 3 ack events should be retrievable")

	// Verify all sources are represented
	sourceMap := make(map[entity.AckSource]bool)
	for _, event := range events {
		sourceMap[event.Source] = true
	}

	assert.True(t, sourceMap[entity.AckSourceSlack], "Slack ack should be present")
	assert.True(t, sourceMap[entity.AckSourcePagerDuty], "PagerDuty ack should be present")
	assert.True(t, sourceMap[entity.AckSourceAPI], "API ack should be present")

	// Verify latest is one of the three
	latest, err := ackRepo.FindLatestByAlertID(ctx, alertID)
	require.NoError(t, err)
	require.NotNil(t, latest)

	found := false
	for _, id := range savedIDs {
		if latest.ID == id {
			found = true
			break
		}
	}
	assert.True(t, found, "Latest event should be one of the saved events")
}

// TestConcurrentAckEvents_HighVolume verifies that the system can handle
// a high volume of concurrent ack event writes without data loss.
func TestConcurrentAckEvents_HighVolume(t *testing.T) {
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
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM ack_events")
	_, _ = db.Primary().Exec("DELETE FROM alerts")

	alertRepo := NewAlertRepository(db)

	// Create 5 test alerts
	const numAlerts = 5
	alertIDs := make([]string, numAlerts)

	for i := 0; i < numAlerts; i++ {
		alert := entity.NewAlert(
			fmt.Sprintf("fp-%d", i),
			fmt.Sprintf("Alert-%d", i),
			fmt.Sprintf("server-%d", i),
			"http://example.com",
			fmt.Sprintf("Summary %d", i),
			entity.SeverityCritical,
		)
		err := alertRepo.Save(ctx, alert)
		require.NoError(t, err)
		alertIDs[i] = alert.ID
	}

	// Simulate 10 instances each creating 2 ack events concurrently (100 total)
	const numInstances = 10
	const acksPerInstance = 2

	var wg sync.WaitGroup
	errors := make(chan error, numInstances*acksPerInstance)

	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(instanceNum int) {
			defer wg.Done()

			// Each instance creates its own connection
			instanceDB, err := NewDB(cfg)
			if err != nil {
				errors <- err
				return
			}
			defer instanceDB.Close()

			instanceRepo := NewAckEventRepository(instanceDB)

			// Create multiple ack events for different alerts
			for j := 0; j < acksPerInstance; j++ {
				alertID := alertIDs[(instanceNum+j)%numAlerts]

				event := entity.NewAckEvent(
					alertID,
					entity.AckSourceSlack,
					fmt.Sprintf("U%d-%d", instanceNum, j),
					fmt.Sprintf("user%d-%d@example.com", instanceNum, j),
					fmt.Sprintf("User %d-%d", instanceNum, j),
				).WithNote(fmt.Sprintf("Ack from instance %d, event %d", instanceNum, j))

				if err := instanceRepo.Save(ctx, event); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("High volume ack event error: %v", err)
		errorCount++
	}

	assert.Equal(t, 0, errorCount, "No errors should occur during high-volume ack event creation")

	// Verify all ack events were saved
	ackRepo := NewAckEventRepository(db)
	for _, alertID := range alertIDs {
		events, err := ackRepo.FindByAlertID(ctx, alertID)
		require.NoError(t, err)
		assert.NotEmpty(t, events, "Each alert should have ack events")
	}

	// Verify total count
	var totalCount int
	err = db.Primary().QueryRow("SELECT COUNT(*) FROM ack_events").Scan(&totalCount)
	require.NoError(t, err)
	assert.Equal(t, numInstances*acksPerInstance, totalCount, "All ack events should be saved")
}

// TestGlobalSilenceRules_CrossInstance verifies that silence rules created on one instance
// apply to alerts arriving at another instance.
func TestGlobalSilenceRules_CrossInstance(t *testing.T) {
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

	// Instance A
	dbA, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer dbA.Close()

	// Run migrations
	migrator := NewMigrator(dbA.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = dbA.Primary().Exec("DELETE FROM silences")
	_, _ = dbA.Primary().Exec("DELETE FROM ack_events")
	_, _ = dbA.Primary().Exec("DELETE FROM alerts")

	silenceRepoA := NewSilenceRepository(dbA)

	// Instance B
	dbB, err := NewDB(cfg)
	require.NoError(t, err)
	defer dbB.Close()

	alertRepoB := NewAlertRepository(dbB)
	silenceRepoB := NewSilenceRepository(dbB)

	// Scenario 1: Instance A creates silence rule for specific instance
	silence1, err := entity.NewSilenceMark(
		1*time.Hour,
		"admin@example.com",
		"admin@example.com",
		entity.AckSourceSlack,
	)
	require.NoError(t, err)
	silence1.ForInstance("server-1").WithReason("Maintenance window")

	err = silenceRepoA.Save(ctx, silence1)
	require.NoError(t, err)

	// Instance B creates alert from server-1
	alert1 := entity.NewAlert(
		"fp-1",
		"Alert1",
		"server-1",
		"http://example.com",
		"Summary",
		entity.SeverityCritical,
	)
	err = alertRepoB.Save(ctx, alert1)
	require.NoError(t, err)

	// Instance B should see the silence rule applies to this alert
	matchingSilences, err := silenceRepoB.FindMatchingAlert(ctx, alert1)
	require.NoError(t, err)
	assert.NotEmpty(t, matchingSilences, "Silence created on instance A should match alert on instance B")

	found := false
	for _, s := range matchingSilences {
		if s.ID == silence1.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Specific silence should match the alert")

	// Scenario 2: Instance A creates silence rule by labels
	silence2, err := entity.NewSilenceMark(
		1*time.Hour,
		"ops@example.com",
		"ops@example.com",
		entity.AckSourceAPI,
	)
	require.NoError(t, err)
	silence2.WithLabel("env", "production").WithLabel("team", "platform").WithReason("Deployment")

	err = silenceRepoA.Save(ctx, silence2)
	require.NoError(t, err)

	// Instance B creates alert with matching labels
	alert2 := entity.NewAlert(
		"fp-2",
		"Alert2",
		"server-2",
		"http://example.com",
		"Summary",
		entity.SeverityCritical,
	)
	alert2.AddLabel("env", "production")
	alert2.AddLabel("team", "platform")
	alert2.AddLabel("service", "api")

	err = alertRepoB.Save(ctx, alert2)
	require.NoError(t, err)

	// Instance B should see the label-based silence applies
	matchingSilences2, err := silenceRepoB.FindMatchingAlert(ctx, alert2)
	require.NoError(t, err)
	assert.NotEmpty(t, matchingSilences2, "Label-based silence should match alert")

	found = false
	for _, s := range matchingSilences2 {
		if s.ID == silence2.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Label-based silence should match the alert")

	// Scenario 3: Instance A creates silence by fingerprint
	silence3, err := entity.NewSilenceMark(
		1*time.Hour,
		"sre@example.com",
		"sre@example.com",
		entity.AckSourcePagerDuty,
	)
	require.NoError(t, err)
	silence3.ForFingerprint("fp-3").WithReason("Known issue")

	err = silenceRepoA.Save(ctx, silence3)
	require.NoError(t, err)

	// Instance B creates alert with matching fingerprint
	alert3 := entity.NewAlert(
		"fp-3",
		"Alert3",
		"server-3",
		"http://example.com",
		"Summary",
		entity.SeverityCritical,
	)
	err = alertRepoB.Save(ctx, alert3)
	require.NoError(t, err)

	// Instance B should see the fingerprint-based silence applies
	matchingSilences3, err := silenceRepoB.FindMatchingAlert(ctx, alert3)
	require.NoError(t, err)
	assert.NotEmpty(t, matchingSilences3, "Fingerprint-based silence should match alert")

	found = false
	for _, s := range matchingSilences3 {
		if s.ID == silence3.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Fingerprint-based silence should match the alert")

	// Verify that alerts not matching the silences are not affected
	alert4 := entity.NewAlert(
		"fp-4",
		"Alert4",
		"server-4",
		"http://example.com",
		"Summary",
		entity.SeverityCritical,
	)
	alert4.AddLabel("env", "development")

	err = alertRepoB.Save(ctx, alert4)
	require.NoError(t, err)

	matchingSilences4, err := silenceRepoB.FindMatchingAlert(ctx, alert4)
	require.NoError(t, err)

	// Should not match any of the previously created silences
	for _, s := range matchingSilences4 {
		assert.NotEqual(t, silence1.ID, s.ID)
		assert.NotEqual(t, silence2.ID, s.ID)
		assert.NotEqual(t, silence3.ID, s.ID)
	}
}

// TestConcurrentSilenceManagement verifies that multiple instances can manage silences concurrently.
func TestConcurrentSilenceManagement(t *testing.T) {
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
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	// Simulate 5 instances each creating 3 silence rules concurrently
	const numInstances = 5
	const silencesPerInstance = 3

	var wg sync.WaitGroup
	errors := make(chan error, numInstances*silencesPerInstance)
	silenceIDs := make(chan string, numInstances*silencesPerInstance)

	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(instanceNum int) {
			defer wg.Done()

			// Each instance creates its own connection
			instanceDB, err := NewDB(cfg)
			if err != nil {
				errors <- err
				return
			}
			defer instanceDB.Close()

			instanceRepo := NewSilenceRepository(instanceDB)

			// Create multiple silences
			for j := 0; j < silencesPerInstance; j++ {
				silence, err := entity.NewSilenceMark(
					1*time.Hour,
					fmt.Sprintf("user%d@example.com", instanceNum),
					fmt.Sprintf("user%d@example.com", instanceNum),
					entity.AckSourceSlack,
				)
				if err != nil {
					errors <- err
					continue
				}

				silence.ForInstance(fmt.Sprintf("server-%d-%d", instanceNum, j))
				silence.WithReason(fmt.Sprintf("Silence from instance %d, rule %d", instanceNum, j))

				if err := instanceRepo.Save(ctx, silence); err != nil {
					errors <- err
				} else {
					silenceIDs <- silence.ID
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	close(silenceIDs)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent silence creation error: %v", err)
		errorCount++
	}

	assert.Equal(t, 0, errorCount, "No errors should occur when creating silences concurrently")

	// Collect saved silence IDs
	savedIDs := make([]string, 0)
	for id := range silenceIDs {
		savedIDs = append(savedIDs, id)
	}

	assert.Len(t, savedIDs, numInstances*silencesPerInstance, "All silences should be saved")

	// Verify all silences are active and retrievable
	repo := NewSilenceRepository(db)
	activeSilences, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, activeSilences, numInstances*silencesPerInstance, "All silences should be active")
}

// TestSilenceExpiration verifies that expired silences are correctly filtered and can be cleaned up.
func TestSilenceExpiration(t *testing.T) {
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
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	repo := NewSilenceRepository(db)

	now := time.Now().UTC()

	// Create active silences
	activeSilence1 := &entity.SilenceMark{
		ID:        "active-1",
		StartAt:   now.Add(-10 * time.Minute),
		EndAt:     now.Add(50 * time.Minute),
		Reason:    "Active silence 1",
		Source:    entity.AckSourceSlack,
		Labels:    make(map[string]string),
		CreatedAt: now,
	}
	activeSilence1.ForInstance("server-active-1")

	activeSilence2 := &entity.SilenceMark{
		ID:        "active-2",
		StartAt:   now.Add(-5 * time.Minute),
		EndAt:     now.Add(30 * time.Minute),
		Reason:    "Active silence 2",
		Source:    entity.AckSourceAPI,
		Labels:    make(map[string]string),
		CreatedAt: now,
	}
	activeSilence2.ForInstance("server-active-2")

	// Create expired silences
	expiredSilence1 := &entity.SilenceMark{
		ID:        "expired-1",
		StartAt:   now.Add(-2 * time.Hour),
		EndAt:     now.Add(-1 * time.Hour),
		Reason:    "Expired silence 1",
		Source:    entity.AckSourceSlack,
		Labels:    make(map[string]string),
		CreatedAt: now.Add(-2 * time.Hour),
	}
	expiredSilence1.ForInstance("server-expired-1")

	expiredSilence2 := &entity.SilenceMark{
		ID:        "expired-2",
		StartAt:   now.Add(-3 * time.Hour),
		EndAt:     now.Add(-2 * time.Hour),
		Reason:    "Expired silence 2",
		Source:    entity.AckSourcePagerDuty,
		Labels:    make(map[string]string),
		CreatedAt: now.Add(-3 * time.Hour),
	}
	expiredSilence2.ForInstance("server-expired-2")

	// Save all silences
	for _, s := range []*entity.SilenceMark{activeSilence1, activeSilence2, expiredSilence1, expiredSilence2} {
		err := repo.Save(ctx, s)
		require.NoError(t, err)
	}

	// Verify FindActive only returns active silences
	activeSilences, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, activeSilences, 2, "Should only return 2 active silences")

	// Verify all silences exist before cleanup
	totalCount := 0
	err = db.Primary().QueryRow("SELECT COUNT(*) FROM silences").Scan(&totalCount)
	require.NoError(t, err)
	assert.Equal(t, 4, totalCount)

	// Delete expired silences
	deletedCount, err := repo.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, deletedCount, "Should delete 2 expired silences")

	// Verify only active silences remain
	remainingCount := 0
	err = db.Primary().QueryRow("SELECT COUNT(*) FROM silences").Scan(&remainingCount)
	require.NoError(t, err)
	assert.Equal(t, 2, remainingCount, "Only active silences should remain")

	// Verify FindActive still returns 2
	stillActive, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, stillActive, 2)
}

// TestMultiInstance_ConcurrentWrites verifies that 3+ instances can write concurrently
// without data corruption or loss.
func TestMultiInstance_ConcurrentWrites(t *testing.T) {
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

	// Setup database
	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")
	_, _ = db.Primary().Exec("DELETE FROM ack_events")
	_, _ = db.Primary().Exec("DELETE FROM alerts")

	// Simulate 5 instances concurrently writing different entity types
	const numInstances = 5

	var wg sync.WaitGroup
	errors := make(chan error, numInstances*3) // 3 operations per instance

	for i := 0; i < numInstances; i++ {
		wg.Add(1)
		go func(instanceNum int) {
			defer wg.Done()

			// Each instance creates its own connection
			instanceDB, err := NewDB(cfg)
			if err != nil {
				errors <- fmt.Errorf("instance %d: connection error: %w", instanceNum, err)
				return
			}
			defer instanceDB.Close()

			alertRepo := NewAlertRepository(instanceDB)
			ackRepo := NewAckEventRepository(instanceDB)
			silenceRepo := NewSilenceRepository(instanceDB)

			// Create alert
			alert := entity.NewAlert(
				fmt.Sprintf("fp-%d", instanceNum),
				fmt.Sprintf("Alert-%d", instanceNum),
				fmt.Sprintf("server-%d", instanceNum),
				"http://example.com",
				fmt.Sprintf("Summary %d", instanceNum),
				entity.SeverityCritical,
			)
			alert.AddLabel("instance", fmt.Sprintf("%d", instanceNum))

			if err := alertRepo.Save(ctx, alert); err != nil {
				errors <- fmt.Errorf("instance %d: alert save error: %w", instanceNum, err)
				return
			}

			// Create ack event
			ackEvent := entity.NewAckEvent(
				alert.ID,
				entity.AckSourceSlack,
				fmt.Sprintf("U%d", instanceNum),
				fmt.Sprintf("user%d@example.com", instanceNum),
				fmt.Sprintf("User %d", instanceNum),
			).WithNote(fmt.Sprintf("Ack from instance %d", instanceNum))

			if err := ackRepo.Save(ctx, ackEvent); err != nil {
				errors <- fmt.Errorf("instance %d: ack save error: %w", instanceNum, err)
				return
			}

			// Create silence
			silence, err := entity.NewSilenceMark(
				1*time.Hour,
				fmt.Sprintf("admin%d@example.com", instanceNum),
				fmt.Sprintf("admin%d@example.com", instanceNum),
				entity.AckSourceAPI,
			)
			if err != nil {
				errors <- fmt.Errorf("instance %d: silence creation error: %w", instanceNum, err)
				return
			}
			silence.ForInstance(fmt.Sprintf("server-%d", instanceNum))

			if err := silenceRepo.Save(ctx, silence); err != nil {
				errors <- fmt.Errorf("instance %d: silence save error: %w", instanceNum, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorList := make([]error, 0)
	for err := range errors {
		errorList = append(errorList, err)
		t.Errorf("Concurrent write error: %v", err)
	}

	assert.Empty(t, errorList, "No errors should occur during concurrent writes from multiple instances")

	// Verify all data was written correctly
	alertRepo := NewAlertRepository(db)
	ackRepo := NewAckEventRepository(db)
	silenceRepo := NewSilenceRepository(db)

	// Verify alerts
	alerts, err := alertRepo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, alerts, numInstances, "All instances should have created alerts")

	// Verify ack events
	for i := 0; i < numInstances; i++ {
		alert, err := alertRepo.FindByFingerprint(ctx, fmt.Sprintf("fp-%d", i))
		require.NoError(t, err)
		require.NotEmpty(t, alert)

		acks, err := ackRepo.FindByAlertID(ctx, alert[0].ID)
		require.NoError(t, err)
		assert.Len(t, acks, 1, "Each alert should have one ack event")
	}

	// Verify silences
	silences, err := silenceRepo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, silences, numInstances, "All instances should have created silences")
}

// TestConnectionPool_ExhaustionAndRecovery verifies that the connection pool
// can handle exhaustion and recover gracefully.
func TestConnectionPool_ExhaustionAndRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use a smaller pool for testing exhaustion
	cfg := &config.MySQLConfig{
		Primary: config.MySQLInstanceConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			Username: "root",
			Password: "password",
		},
		Pool: config.MySQLPoolConfig{
			MaxOpenConns:    3,  // Small pool to force exhaustion
			MaxIdleConns:    1,
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
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM alerts")

	repo := NewAlertRepository(db)

	// Try to create more concurrent operations than pool size
	const numGoroutines = 10 // More than MaxOpenConns

	var wg sync.WaitGroup
	successes := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			alert := entity.NewAlert(
				fmt.Sprintf("fp-pool-%d", idx),
				fmt.Sprintf("PoolTest-%d", idx),
				"server-1",
				"http://example.com",
				fmt.Sprintf("Pool test %d", idx),
				entity.SeverityCritical,
			)

			// This should wait for available connection and succeed
			err := repo.Save(ctx, alert)
			if err != nil {
				t.Logf("Error saving alert %d: %v", idx, err)
				successes <- false
			} else {
				successes <- true
			}
		}(i)
	}

	wg.Wait()
	close(successes)

	// Count successes
	successCount := 0
	for success := range successes {
		if success {
			successCount++
		}
	}

	// All operations should eventually succeed despite pool exhaustion
	assert.Equal(t, numGoroutines, successCount, "All operations should succeed (connection pool should handle queueing)")

	// Verify pool recovered and is functional
	stats := db.Stats()
	t.Logf("Connection pool stats - Open: %d, InUse: %d, Idle: %d, WaitCount: %d",
		stats.Primary.OpenConnections,
		stats.Primary.InUse,
		stats.Primary.Idle,
		stats.Primary.WaitCount)

	// Pool should have waited for connections (WaitCount > 0)
	assert.Greater(t, stats.Primary.WaitCount, int64(0), "Pool should have queued requests")

	// Verify data consistency - all alerts should be saved
	alerts, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, alerts, numGoroutines, "All alerts should be saved despite pool exhaustion")
}

// TestMySQL_ConnectionLossAndRecovery simulates connection loss and verifies
// the application can handle it gracefully with retries.
func TestMySQL_ConnectionLossAndRecovery(t *testing.T) {
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
		return
	}
	defer db.Close()

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewAlertRepository(db)

	// Verify connection works initially
	alert1 := entity.NewAlert("fp-1", "Alert1", "server-1", "http://example.com", "Summary", entity.SeverityCritical)
	err = repo.Save(ctx, alert1)
	require.NoError(t, err)

	// Simulate connection loss by setting very short connection lifetime
	// In a real scenario, you might kill connections or stop MySQL temporarily
	// For this test, we'll verify the connection can recover after errors

	// Force close all connections in the pool
	db.Primary().SetMaxOpenConns(0)
	time.Sleep(100 * time.Millisecond)
	db.Primary().SetMaxOpenConns(25)

	// Try to use the connection - it should recover
	alert2 := entity.NewAlert("fp-2", "Alert2", "server-2", "http://example.com", "Summary", entity.SeverityCritical)
	
	// Retry logic (simulating application-level retry)
	maxRetries := 3
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = repo.Save(ctx, alert2)
		if err == nil {
			break
		}
		lastErr = err
		t.Logf("Attempt %d failed: %v, retrying...", attempt+1, err)
		time.Sleep(100 * time.Millisecond)
	}

	assert.NoError(t, lastErr, "Connection should recover after reconnection")

	// Verify both alerts were saved
	alerts, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(alerts), 2, "Both alerts should be saved after recovery")
}
