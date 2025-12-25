package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
)

// BenchmarkAlertSave measures alert save performance.
func BenchmarkAlertSave(b *testing.B) {
	db, err := NewDB(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		b.Fatal(err)
	}

	repo := NewAlertRepository(db.DB)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alert := &entity.Alert{
			ID:          fmt.Sprintf("alert-%d", i),
			Fingerprint: fmt.Sprintf("fp-%d", i),
			Name:        "Benchmark Alert",
			Severity:    entity.SeverityCritical,
			State:       entity.StateActive,
			Labels:      map[string]string{"env": "test", "app": "bench"},
			Annotations: map[string]string{"summary": "test alert"},
			FiredAt:     time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := repo.Save(ctx, alert); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAlertFindByID measures alert read performance.
func BenchmarkAlertFindByID(b *testing.B) {
	db, err := NewDB(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		b.Fatal(err)
	}

	repo := NewAlertRepository(db.DB)
	ctx := context.Background()

	// Prepopulate with alerts
	const numAlerts = 10000
	for i := 0; i < numAlerts; i++ {
		alert := &entity.Alert{
			ID:          fmt.Sprintf("alert-%d", i),
			Fingerprint: fmt.Sprintf("fp-%d", i),
			Name:        "Benchmark Alert",
			Severity:    entity.SeverityCritical,
			State:       entity.StateActive,
			Labels:      map[string]string{"env": "test"},
			Annotations: map[string]string{},
			FiredAt:     time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := repo.Save(ctx, alert); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Random alert ID
		alertID := fmt.Sprintf("alert-%d", i%numAlerts)
		if _, err := repo.FindByID(ctx, alertID); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAlertFindActive measures active alert query performance.
func BenchmarkAlertFindActive(b *testing.B) {
	db, err := NewDB(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		b.Fatal(err)
	}

	repo := NewAlertRepository(db.DB)
	ctx := context.Background()

	// Prepopulate with alerts (80% active, 20% resolved)
	const numAlerts = 10000
	for i := 0; i < numAlerts; i++ {
		state := entity.StateActive
		if i%5 == 0 {
			state = entity.StateResolved
		}
		alert := &entity.Alert{
			ID:          fmt.Sprintf("alert-%d", i),
			Fingerprint: fmt.Sprintf("fp-%d", i),
			Name:        "Benchmark Alert",
			Severity:    entity.SeverityCritical,
			State:       state,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
			FiredAt:     time.Now().UTC(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if err := repo.Save(ctx, alert); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.FindActive(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAckEventSave measures ack event save performance.
func BenchmarkAckEventSave(b *testing.B) {
	db, err := NewDB(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		b.Fatal(err)
	}

	alertRepo := NewAlertRepository(db.DB)
	ackRepo := NewAckEventRepository(db.DB)
	ctx := context.Background()

	// Create base alert
	alert := &entity.Alert{
		ID:          "bench-alert",
		Fingerprint: "fp-bench",
		Name:        "Benchmark Alert",
		Severity:    entity.SeverityCritical,
		State:       entity.StateActive,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
		FiredAt:     time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := alertRepo.Save(ctx, alert); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ackEvent := &entity.AckEvent{
			ID:        fmt.Sprintf("ack-%d", i),
			AlertID:   alert.ID,
			Source:    entity.AckSourceSlack,
			UserID:    "U123",
			UserEmail: "bench@example.com",
			UserName:  "Bench User",
			CreatedAt: time.Now().UTC(),
		}
		if err := ackRepo.Save(ctx, ackEvent); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSilenceFindMatchingAlert measures silence matching performance.
func BenchmarkSilenceFindMatchingAlert(b *testing.B) {
	db, err := NewDB(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		b.Fatal(err)
	}

	repo := NewSilenceRepository(db.DB)
	ctx := context.Background()

	// Prepopulate with silences
	now := time.Now().UTC()
	for i := 0; i < 100; i++ {
		silence := &entity.SilenceMark{
			ID:             fmt.Sprintf("silence-%d", i),
			Fingerprint:    fmt.Sprintf("fp-%d", i%10),
			StartAt:        now.Add(-10 * time.Minute),
			EndAt:          now.Add(10 * time.Minute),
			CreatedBy:      "bench@example.com",
			CreatedByEmail: "bench@example.com",
			Source:         entity.AckSourceSlack,
			Labels:         map[string]string{"env": "test"},
			CreatedAt:      now,
		}
		if err := repo.Save(ctx, silence); err != nil {
			b.Fatal(err)
		}
	}

	alert := &entity.Alert{
		ID:          "bench-alert",
		Fingerprint: "fp-5",
		Instance:    "localhost:9090",
		Labels:      map[string]string{"env": "test", "app": "bench"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.FindMatchingAlert(ctx, alert); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAlertUpdate measures alert update performance.
func BenchmarkAlertUpdate(b *testing.B) {
	db, err := NewDB(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		b.Fatal(err)
	}

	repo := NewAlertRepository(db.DB)
	ctx := context.Background()

	// Create initial alert
	alert := &entity.Alert{
		ID:          "update-alert",
		Fingerprint: "fp-update",
		Name:        "Update Benchmark",
		Severity:    entity.SeverityCritical,
		State:       entity.StateActive,
		Labels:      map[string]string{"env": "test"},
		Annotations: map[string]string{},
		FiredAt:     time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := repo.Save(ctx, alert); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alert.State = entity.StateAcked
		alert.UpdatedAt = time.Now().UTC()
		if err := repo.Update(ctx, alert); err != nil {
			b.Fatal(err)
		}
	}
}
