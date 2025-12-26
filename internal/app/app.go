package app

import (
	"context"
	"io"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/observability"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/server"
)

// Application holds all application dependencies and lifecycle
type Application struct {
	config        *config.Config
	configManager *config.ConfigManager
	logger        *AtomicLogger
	telemetry     *observability.Telemetry

	// Storage
	alertRepo    repository.AlertRepository
	ackEventRepo repository.AckEventRepository
	silenceRepo  repository.SilenceRepository
	txManager    repository.TransactionManager
	dbCloser     io.Closer // For cleanup

	// Infrastructure clients
	clients *Clients

	// Use cases
	useCases *UseCases

	// HTTP layer
	handlers *server.Handlers
	server   *server.Server
}

// New creates a new Application instance
func New(configPath string) (*Application, error) {
	app := &Application{}

	if err := app.bootstrap(configPath); err != nil {
		return nil, err
	}

	return app, nil
}

// Start runs the application until context is cancelled
func (app *Application) Start(ctx context.Context) error {
	app.logger.Get().Info("starting alert-bridge",
		"port", app.config.Server.Port,
	)

	return app.server.Run(ctx)
}

// Shutdown gracefully stops the application
func (app *Application) Shutdown() error {
	app.logger.Get().Info("shutting down alert-bridge")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown telemetry
	if app.telemetry != nil {
		if err := app.telemetry.Shutdown(ctx); err != nil {
			app.logger.Get().Error("failed to shutdown telemetry", "error", err)
		}
	}

	// Close database
	if app.dbCloser != nil {
		if err := app.dbCloser.Close(); err != nil {
			app.logger.Get().Error("failed to close database", "error", err)
			return err
		}
	}

	app.logger.Get().Info("alert-bridge stopped")
	return nil
}
