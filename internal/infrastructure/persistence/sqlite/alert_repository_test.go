package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

func setupAlertRepo(t *testing.T) (*AlertRepository, func()) {
	t.Helper()

	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to migrate: %v", err)
	}

	return NewAlertRepository(db.DB), func() { db.Close() }
}

func TestAlertRepository_Save(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	alert := entity.NewAlert("fp1", "TestAlert", "instance1", "target1", "Test summary", entity.SeverityWarning)
	alert.AddLabel("env", "test")
	alert.AddAnnotation("description", "Test description")

	err := repo.Save(ctx, alert)
	if err != nil {
		t.Fatalf("failed to save alert: %v", err)
	}

	// Verify it was saved
	found, err := repo.FindByID(ctx, alert.ID)
	if err != nil {
		t.Fatalf("failed to find alert: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find alert, got nil")
	}
	if found.Name != alert.Name {
		t.Errorf("expected name %s, got %s", alert.Name, found.Name)
	}
	if found.Labels["env"] != "test" {
		t.Errorf("expected label env=test, got %v", found.Labels)
	}
}

func TestAlertRepository_Save_Duplicate(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	alert := entity.NewAlert("fp1", "TestAlert", "instance1", "target1", "Test summary", entity.SeverityWarning)

	err := repo.Save(ctx, alert)
	if err != nil {
		t.Fatalf("failed to save alert: %v", err)
	}

	// Try to save again
	err = repo.Save(ctx, alert)
	if err != entity.ErrDuplicateAlert {
		t.Errorf("expected ErrDuplicateAlert, got %v", err)
	}
}

func TestAlertRepository_FindByID_NotFound(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	found, err := repo.FindByID(ctx, "non-existent")
	if err != nil {
		t.Fatalf("expected nil error for not found, got %v", err)
	}
	if found != nil {
		t.Errorf("expected nil for not found, got %v", found)
	}
}

func TestAlertRepository_FindByFingerprint(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple alerts with same fingerprint
	alert1 := entity.NewAlert("fp-same", "Alert1", "instance1", "target1", "Summary1", entity.SeverityWarning)
	alert2 := entity.NewAlert("fp-same", "Alert2", "instance2", "target2", "Summary2", entity.SeverityCritical)
	alert3 := entity.NewAlert("fp-other", "Alert3", "instance3", "target3", "Summary3", entity.SeverityInfo)

	for _, a := range []*entity.Alert{alert1, alert2, alert3} {
		if err := repo.Save(ctx, a); err != nil {
			t.Fatalf("failed to save alert: %v", err)
		}
	}

	// Find by fingerprint
	found, err := repo.FindByFingerprint(ctx, "fp-same")
	if err != nil {
		t.Fatalf("failed to find by fingerprint: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(found))
	}
}

func TestAlertRepository_FindBySlackMessageID(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	alert := entity.NewAlert("fp1", "TestAlert", "instance1", "target1", "Summary", entity.SeverityWarning)
	alert.SetSlackMessageID("C123:1234567890.123456")

	if err := repo.Save(ctx, alert); err != nil {
		t.Fatalf("failed to save alert: %v", err)
	}

	found, err := repo.FindBySlackMessageID(ctx, "C123:1234567890.123456")
	if err != nil {
		t.Fatalf("failed to find by slack message ID: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find alert, got nil")
	}
	if found.ID != alert.ID {
		t.Errorf("expected ID %s, got %s", alert.ID, found.ID)
	}
}

func TestAlertRepository_FindByPagerDutyIncidentID(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	alert := entity.NewAlert("fp1", "TestAlert", "instance1", "target1", "Summary", entity.SeverityWarning)
	alert.SetPagerDutyIncidentID("PD123456")

	if err := repo.Save(ctx, alert); err != nil {
		t.Fatalf("failed to save alert: %v", err)
	}

	found, err := repo.FindByPagerDutyIncidentID(ctx, "PD123456")
	if err != nil {
		t.Fatalf("failed to find by PagerDuty incident ID: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find alert, got nil")
	}
	if found.ID != alert.ID {
		t.Errorf("expected ID %s, got %s", alert.ID, found.ID)
	}
}

func TestAlertRepository_Update(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	alert := entity.NewAlert("fp1", "TestAlert", "instance1", "target1", "Summary", entity.SeverityWarning)

	if err := repo.Save(ctx, alert); err != nil {
		t.Fatalf("failed to save alert: %v", err)
	}

	// Update the alert
	now := time.Now().UTC()
	alert.Acknowledge("user@example.com", now)

	if err := repo.Update(ctx, alert); err != nil {
		t.Fatalf("failed to update alert: %v", err)
	}

	// Verify update
	found, err := repo.FindByID(ctx, alert.ID)
	if err != nil {
		t.Fatalf("failed to find alert: %v", err)
	}
	if found.State != entity.StateAcked {
		t.Errorf("expected state %s, got %s", entity.StateAcked, found.State)
	}
	if found.AckedBy != "user@example.com" {
		t.Errorf("expected acked_by user@example.com, got %s", found.AckedBy)
	}
}

func TestAlertRepository_Update_NotFound(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	alert := entity.NewAlert("fp1", "TestAlert", "instance1", "target1", "Summary", entity.SeverityWarning)

	err := repo.Update(ctx, alert)
	if err != entity.ErrAlertNotFound {
		t.Errorf("expected ErrAlertNotFound, got %v", err)
	}
}

func TestAlertRepository_FindActive(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create alerts in different states
	active := entity.NewAlert("fp1", "Active", "instance1", "target1", "Summary", entity.SeverityWarning)
	acked := entity.NewAlert("fp2", "Acked", "instance2", "target2", "Summary", entity.SeverityWarning)
	acked.Acknowledge("user", time.Now())
	resolved := entity.NewAlert("fp3", "Resolved", "instance3", "target3", "Summary", entity.SeverityWarning)
	resolved.Resolve(time.Now())

	for _, a := range []*entity.Alert{active, acked, resolved} {
		if err := repo.Save(ctx, a); err != nil {
			t.Fatalf("failed to save alert: %v", err)
		}
	}

	// FindActive should return active and acked, but not resolved
	found, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("failed to find active alerts: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 active alerts, got %d", len(found))
	}
}

func TestAlertRepository_FindFiring(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create alerts in different states
	active := entity.NewAlert("fp1", "Active", "instance1", "target1", "Summary", entity.SeverityWarning)
	acked := entity.NewAlert("fp2", "Acked", "instance2", "target2", "Summary", entity.SeverityWarning)
	acked.Acknowledge("user", time.Now())
	resolved := entity.NewAlert("fp3", "Resolved", "instance3", "target3", "Summary", entity.SeverityWarning)
	resolved.Resolve(time.Now())

	for _, a := range []*entity.Alert{active, acked, resolved} {
		if err := repo.Save(ctx, a); err != nil {
			t.Fatalf("failed to save alert: %v", err)
		}
	}

	// FindFiring should return active and acked
	found, err := repo.FindFiring(ctx)
	if err != nil {
		t.Fatalf("failed to find firing alerts: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 firing alerts, got %d", len(found))
	}
}

func TestAlertRepository_Delete(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	alert := entity.NewAlert("fp1", "TestAlert", "instance1", "target1", "Summary", entity.SeverityWarning)

	if err := repo.Save(ctx, alert); err != nil {
		t.Fatalf("failed to save alert: %v", err)
	}

	// Delete
	if err := repo.Delete(ctx, alert.ID); err != nil {
		t.Fatalf("failed to delete alert: %v", err)
	}

	// Verify deleted
	found, err := repo.FindByID(ctx, alert.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Error("expected alert to be deleted")
	}
}

func TestAlertRepository_Delete_NotFound(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()
	err := repo.Delete(ctx, "non-existent")
	if err != entity.ErrAlertNotFound {
		t.Errorf("expected ErrAlertNotFound, got %v", err)
	}
}

func TestAlertRepository_EmptySliceNotNil(t *testing.T) {
	repo, cleanup := setupAlertRepo(t)
	defer cleanup()

	ctx := context.Background()

	// All Find* methods returning slices should return empty slice, not nil
	found, err := repo.FindByFingerprint(ctx, "non-existent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(found) != 0 {
		t.Errorf("expected empty slice, got %d items", len(found))
	}

	active, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active == nil {
		t.Error("expected empty slice, got nil")
	}

	firing, err := repo.FindFiring(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if firing == nil {
		t.Error("expected empty slice, got nil")
	}
}
