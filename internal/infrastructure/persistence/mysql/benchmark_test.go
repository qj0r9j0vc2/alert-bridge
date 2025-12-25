package mysql

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/entity"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

// setupBenchmarkDB creates a database connection for benchmarking.
func setupBenchmarkDB(b *testing.B) (*DB, func()) {
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
		b.Skipf("Skipping benchmark: MySQL not available: %v", err)
	}

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		db.Close()
		b.Fatalf("Failed to run migrations: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// BenchmarkAlert_Read measures alert read performance (target: < 100ms).
func BenchmarkAlert_Read(b *testing.B) {
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Create test alert
	alert := entity.NewAlert(
		"bench-fp",
		"BenchAlert",
		"server-1",
		"http://example.com",
		"Benchmark alert",
		entity.SeverityCritical,
	)
	alert.AddLabel("env", "production")
	alert.AddLabel("team", "platform")

	err := repo.Save(ctx, alert)
	if err != nil {
		b.Fatalf("Failed to save alert: %v", err)
	}

	// Reset timer before benchmark
	b.ResetTimer()

	// Benchmark read operation
	for i := 0; i < b.N; i++ {
		_, err := repo.FindByID(ctx, alert.ID)
		if err != nil {
			b.Fatalf("Failed to read alert: %v", err)
		}
	}

	b.StopTimer()

	// Report operations per second
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N) / 1e6 // Convert to ms

	b.ReportMetric(float64(avgLatency), "ms/op")
	b.ReportMetric(opsPerSec, "ops/sec")

	if avgLatency > 100 {
		b.Logf("WARNING: Average read latency %dms exceeds target of 100ms", avgLatency)
	}
}

// BenchmarkAlert_Write measures alert write performance (target: < 200ms).
func BenchmarkAlert_Write(b *testing.B) {
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Reset timer before benchmark
	b.ResetTimer()

	// Benchmark write operation
	for i := 0; i < b.N; i++ {
		alert := entity.NewAlert(
			fmt.Sprintf("bench-fp-%d", i),
			fmt.Sprintf("BenchAlert-%d", i),
			"server-1",
			"http://example.com",
			fmt.Sprintf("Benchmark alert %d", i),
			entity.SeverityCritical,
		)
		alert.AddLabel("env", "production")
		alert.AddLabel("iteration", fmt.Sprintf("%d", i))

		err := repo.Save(ctx, alert)
		if err != nil {
			b.Fatalf("Failed to save alert: %v", err)
		}
	}

	b.StopTimer()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM ack_events")
	_, _ = db.Primary().Exec("DELETE FROM alerts WHERE fingerprint LIKE 'bench-fp-%'")

	// Report operations per second
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N) / 1e6 // Convert to ms

	b.ReportMetric(float64(avgLatency), "ms/op")
	b.ReportMetric(opsPerSec, "ops/sec")

	if avgLatency > 200 {
		b.Logf("WARNING: Average write latency %dms exceeds target of 200ms", avgLatency)
	}
}

// BenchmarkAlert_Update measures alert update performance.
func BenchmarkAlert_Update(b *testing.B) {
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Create test alert
	alert := entity.NewAlert(
		"bench-update-fp",
		"BenchUpdateAlert",
		"server-1",
		"http://example.com",
		"Benchmark update alert",
		entity.SeverityCritical,
	)

	err := repo.Save(ctx, alert)
	if err != nil {
		b.Fatalf("Failed to save alert: %v", err)
	}

	// Reset timer before benchmark
	b.ResetTimer()

	// Benchmark update operation
	for i := 0; i < b.N; i++ {
		// Fetch fresh version
		fresh, err := repo.FindByID(ctx, alert.ID)
		if err != nil {
			b.Fatalf("Failed to read alert: %v", err)
		}

		fresh.Summary = fmt.Sprintf("Updated summary %d", i)
		fresh.UpdatedAt = time.Now().UTC()

		err = repo.Update(ctx, fresh)
		if err != nil {
			b.Fatalf("Failed to update alert: %v", err)
		}
	}

	b.StopTimer()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM alerts WHERE id = ?", alert.ID)

	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N) / 1e6
	b.ReportMetric(float64(avgLatency), "ms/op")
}

// BenchmarkAlert_FindActive measures query performance for active alerts.
func BenchmarkAlert_FindActive(b *testing.B) {
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Create 100 test alerts
	for i := 0; i < 100; i++ {
		alert := entity.NewAlert(
			fmt.Sprintf("bench-active-fp-%d", i),
			fmt.Sprintf("BenchActiveAlert-%d", i),
			"server-1",
			"http://example.com",
			fmt.Sprintf("Benchmark active alert %d", i),
			entity.SeverityCritical,
		)
		_ = repo.Save(ctx, alert)
	}

	// Reset timer before benchmark
	b.ResetTimer()

	// Benchmark FindActive operation
	for i := 0; i < b.N; i++ {
		_, err := repo.FindActive(ctx)
		if err != nil {
			b.Fatalf("Failed to find active alerts: %v", err)
		}
	}

	b.StopTimer()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM alerts WHERE fingerprint LIKE 'bench-active-fp-%'")

	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N) / 1e6
	b.ReportMetric(float64(avgLatency), "ms/op")
}

// BenchmarkAckEvent_Write measures ack event write performance.
func BenchmarkAckEvent_Write(b *testing.B) {
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	alertRepo := NewAlertRepository(db)
	ackRepo := NewAckEventRepository(db)
	ctx := context.Background()

	// Create test alert
	alert := entity.NewAlert(
		"bench-ack-fp",
		"BenchAckAlert",
		"server-1",
		"http://example.com",
		"Benchmark ack alert",
		entity.SeverityCritical,
	)
	_ = alertRepo.Save(ctx, alert)

	// Reset timer before benchmark
	b.ResetTimer()

	// Benchmark ack event write
	for i := 0; i < b.N; i++ {
		ackEvent := entity.NewAckEvent(
			alert.ID,
			entity.AckSourceSlack,
			fmt.Sprintf("U%d", i),
			fmt.Sprintf("user%d@example.com", i),
			fmt.Sprintf("User %d", i),
		)

		err := ackRepo.Save(ctx, ackEvent)
		if err != nil {
			b.Fatalf("Failed to save ack event: %v", err)
		}
	}

	b.StopTimer()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM ack_events WHERE alert_id = ?", alert.ID)
	_, _ = db.Primary().Exec("DELETE FROM alerts WHERE id = ?", alert.ID)

	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N) / 1e6
	b.ReportMetric(float64(avgLatency), "ms/op")
}

// BenchmarkSilence_FindMatching measures silence matching performance.
func BenchmarkSilence_FindMatching(b *testing.B) {
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	silenceRepo := NewSilenceRepository(db)
	ctx := context.Background()

	// Create 50 test silences with various criteria
	for i := 0; i < 50; i++ {
		silence, _ := entity.NewSilenceMark(
			1*time.Hour,
			"admin@example.com",
			"admin@example.com",
			entity.AckSourceAPI,
		)

		switch i % 3 {
		case 0:
			silence.ForInstance(fmt.Sprintf("server-%d", i))
		case 1:
			silence.ForFingerprint(fmt.Sprintf("fp-%d", i))
		case 2:
			silence.WithLabel("env", "production")
		}

		_ = silenceRepo.Save(ctx, silence)
	}

	// Create test alert
	alert := entity.NewAlert(
		"bench-match-fp",
		"BenchMatchAlert",
		"server-10",
		"http://example.com",
		"Benchmark match alert",
		entity.SeverityCritical,
	)
	alert.AddLabel("env", "production")

	// Reset timer before benchmark
	b.ResetTimer()

	// Benchmark silence matching
	for i := 0; i < b.N; i++ {
		_, err := silenceRepo.FindMatchingAlert(ctx, alert)
		if err != nil {
			b.Fatalf("Failed to find matching silences: %v", err)
		}
	}

	b.StopTimer()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM silences")

	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N) / 1e6
	b.ReportMetric(float64(avgLatency), "ms/op")
}

// BenchmarkConcurrent_AlertWrites measures concurrent write performance.
func BenchmarkConcurrent_AlertWrites(b *testing.B) {
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	repo := NewAlertRepository(db)
	ctx := context.Background()

	// Reset timer before benchmark
	b.ResetTimer()

	// Run concurrent writes
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			alert := entity.NewAlert(
				fmt.Sprintf("bench-concurrent-fp-%d", i),
				fmt.Sprintf("BenchConcurrentAlert-%d", i),
				"server-1",
				"http://example.com",
				fmt.Sprintf("Benchmark concurrent alert %d", i),
				entity.SeverityCritical,
			)

			err := repo.Save(ctx, alert)
			if err != nil {
				b.Fatalf("Failed to save alert: %v", err)
			}
			i++
		}
	})

	b.StopTimer()

	// Clean up
	_, _ = db.Primary().Exec("DELETE FROM alerts WHERE fingerprint LIKE 'bench-concurrent-fp-%'")

	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N) / 1e6
	b.ReportMetric(float64(avgLatency), "ms/op")
}
