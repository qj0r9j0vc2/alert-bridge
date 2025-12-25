package mysql

import (
	"context"
	"fmt"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

// Repositories holds all MySQL repository implementations.
type Repositories struct {
	Alert   repository.AlertRepository
	AckEvent repository.AckEventRepository
	Silence  repository.SilenceRepository
}

// NewRepositories creates all MySQL repository implementations.
// It establishes a database connection, runs migrations, and returns all repositories.
func NewRepositories(cfg *config.MySQLConfig) (*Repositories, *DB, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("mysql config is required")
	}

	// Create database connection
	db, err := NewDB(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("creating database connection: %w", err)
	}

	// Run migrations
	migrator := NewMigrator(db.Primary())
	ctx := context.Background()
	if err := migrator.Up(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("running migrations: %w", err)
	}

	// Create repositories
	repos := &Repositories{
		Alert:    NewAlertRepository(db),
		AckEvent: NewAckEventRepository(db),
		Silence:  NewSilenceRepository(db),
	}

	return repos, db, nil
}
