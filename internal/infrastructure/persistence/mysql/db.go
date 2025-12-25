package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

// DB wraps a MySQL database connection with health checking.
type DB struct {
	primary *sql.DB
	replica *sql.DB
	config  *config.MySQLConfig
}

// NewDB creates a new MySQL database connection with connection pooling.
// It establishes connections to both primary and optional replica instances.
func NewDB(cfg *config.MySQLConfig) (*DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("mysql config is required")
	}

	// Build primary DSN
	primaryDSN := buildDSN(
		cfg.Primary.Host,
		cfg.Primary.Port,
		cfg.Primary.Database,
		cfg.Primary.Username,
		cfg.Primary.Password,
		cfg.Charset,
		cfg.ParseTime,
		cfg.Timeout,
	)

	// Open primary connection
	primary, err := sql.Open("mysql", primaryDSN)
	if err != nil {
		return nil, fmt.Errorf("opening primary connection: %w", err)
	}

	// Configure primary connection pool
	primary.SetMaxOpenConns(cfg.Pool.MaxOpenConns)
	primary.SetMaxIdleConns(cfg.Pool.MaxIdleConns)
	primary.SetConnMaxLifetime(cfg.Pool.ConnMaxLifetime)
	primary.SetConnMaxIdleTime(cfg.Pool.ConnMaxIdleTime)

	// Test primary connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	if err := primary.PingContext(ctx); err != nil {
		primary.Close()
		return nil, fmt.Errorf("pinging primary database: %w", err)
	}

	db := &DB{
		primary: primary,
		config:  cfg,
	}

	// Set up replica if enabled
	if cfg.Replica.Enabled {
		replicaDSN := buildDSN(
			cfg.Replica.Host,
			cfg.Replica.Port,
			cfg.Replica.Database,
			cfg.Replica.Username,
			cfg.Replica.Password,
			cfg.Charset,
			cfg.ParseTime,
			cfg.Timeout,
		)

		replica, err := sql.Open("mysql", replicaDSN)
		if err != nil {
			primary.Close()
			return nil, fmt.Errorf("opening replica connection: %w", err)
		}

		// Configure replica connection pool (same settings as primary)
		replica.SetMaxOpenConns(cfg.Pool.MaxOpenConns)
		replica.SetMaxIdleConns(cfg.Pool.MaxIdleConns)
		replica.SetConnMaxLifetime(cfg.Pool.ConnMaxLifetime)
		replica.SetConnMaxIdleTime(cfg.Pool.ConnMaxIdleTime)

		// Test replica connection
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()

		if err := replica.PingContext(ctx); err != nil {
			primary.Close()
			replica.Close()
			return nil, fmt.Errorf("pinging replica database: %w", err)
		}

		db.replica = replica
	}

	return db, nil
}

// buildDSN constructs a MySQL DSN string.
// Format: user:password@tcp(host:port)/database?params
func buildDSN(host string, port int, database, username, password, charset string, parseTime bool, timeout time.Duration) string {
	// Format: user:password@tcp(host:port)/database?parseTime=true&charset=utf8mb4&timeout=5s
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&timeout=%s",
		username,
		password,
		host,
		port,
		database,
		charset,
		parseTime,
		timeout.String(),
	)
	return dsn
}

// Primary returns the primary database connection for writes and consistent reads.
func (db *DB) Primary() *sql.DB {
	return db.primary
}

// Replica returns the replica database connection for reads, or primary if no replica is configured.
func (db *DB) Replica() *sql.DB {
	if db.replica != nil {
		return db.replica
	}
	return db.primary
}

// Ping checks connectivity to the database.
// It pings both primary and replica (if configured).
func (db *DB) Ping(ctx context.Context) error {
	if err := db.primary.PingContext(ctx); err != nil {
		return fmt.Errorf("primary ping failed: %w", err)
	}

	if db.replica != nil {
		if err := db.replica.PingContext(ctx); err != nil {
			return fmt.Errorf("replica ping failed: %w", err)
		}
	}

	return nil
}

// Close closes the database connections.
// It gracefully closes both primary and replica connections.
func (db *DB) Close() error {
	var primaryErr, replicaErr error

	if db.primary != nil {
		primaryErr = db.primary.Close()
	}

	if db.replica != nil {
		replicaErr = db.replica.Close()
	}

	if primaryErr != nil {
		return fmt.Errorf("closing primary: %w", primaryErr)
	}
	if replicaErr != nil {
		return fmt.Errorf("closing replica: %w", replicaErr)
	}

	return nil
}

// Stats returns database connection pool statistics.
type Stats struct {
	Primary DBStats
	Replica *DBStats
}

// DBStats holds connection pool statistics for a database instance.
type DBStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	WaitCount          int64
	WaitDuration       time.Duration
}

// Stats returns connection pool statistics for monitoring.
func (db *DB) Stats() Stats {
	stats := Stats{
		Primary: dbStatsFromSQL(db.primary.Stats()),
	}

	if db.replica != nil {
		replicaStats := dbStatsFromSQL(db.replica.Stats())
		stats.Replica = &replicaStats
	}

	return stats
}

// dbStatsFromSQL converts sql.DBStats to our DBStats type.
func dbStatsFromSQL(s sql.DBStats) DBStats {
	return DBStats{
		MaxOpenConnections: s.MaxOpenConnections,
		OpenConnections:    s.OpenConnections,
		InUse:              s.InUse,
		Idle:               s.Idle,
		WaitCount:          s.WaitCount,
		WaitDuration:       s.WaitDuration,
	}
}
