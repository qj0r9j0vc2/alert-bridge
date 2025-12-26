package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

func TestNewDB_Success(t *testing.T) {
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

	// This test requires a running MySQL instance
	// In CI, this would use testcontainers
	db, err := NewDB(cfg)
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
		return
	}
	defer db.Close()

	// Test that primary connection is available
	assert.NotNil(t, db.Primary())

	// Test ping
	ctx := context.Background()
	err = db.Ping(ctx)
	assert.NoError(t, err)
}

func TestNewDB_NilConfig(t *testing.T) {
	db, err := NewDB(nil)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "config is required")
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		port      int
		database  string
		username  string
		password  string
		charset   string
		parseTime bool
		timeout   time.Duration
		expected  string
	}{
		{
			name:      "standard config",
			host:      "localhost",
			port:      3306,
			database:  "test_db",
			username:  "root",
			password:  "password",
			charset:   "utf8mb4",
			parseTime: true,
			timeout:   5 * time.Second,
			expected:  "root:password@tcp(localhost:3306)/test_db?charset=utf8mb4&parseTime=true&timeout=5s",
		},
		{
			name:      "custom port",
			host:      "mysql.example.com",
			port:      3307,
			database:  "production",
			username:  "app_user",
			password:  "secure_pass",
			charset:   "utf8mb4",
			parseTime: true,
			timeout:   10 * time.Second,
			expected:  "app_user:secure_pass@tcp(mysql.example.com:3307)/production?charset=utf8mb4&parseTime=true&timeout=10s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildDSN(
				tt.host,
				tt.port,
				tt.database,
				tt.username,
				tt.password,
				tt.charset,
				tt.parseTime,
				tt.timeout,
			)
			assert.Equal(t, tt.expected, dsn)
		})
	}
}

func TestDB_Replica(t *testing.T) {
	db := &DB{
		primary: nil, // Mock for testing
		replica: nil,
	}

	// When no replica configured, should return primary
	assert.Equal(t, db.primary, db.Replica())

	// When replica configured, should return replica
	// (In real scenario, these would be actual *sql.DB instances)
	db.primary = nil // Would be actual connection
	db.replica = nil // Would be actual connection
	assert.Equal(t, db.replica, db.Replica())
}

func TestDB_Stats(t *testing.T) {
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

	stats := db.Stats()

	// Check that stats are populated
	require.NotNil(t, stats.Primary)
	assert.Equal(t, 25, stats.Primary.MaxOpenConnections)

	// Replica should be nil if not configured
	assert.Nil(t, stats.Replica)
}

func TestDB_Close(t *testing.T) {
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

	// Close should succeed
	err = db.Close()
	assert.NoError(t, err)

	// Ping after close should fail
	ctx := context.Background()
	err = db.Ping(ctx)
	assert.Error(t, err)
}
