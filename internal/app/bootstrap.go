package app

import (
	"fmt"
)

func (app *Application) bootstrap(configPath string) error {
	// 1. Load configuration
	if err := app.loadConfig(configPath); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Setup logger
	if err := app.setupLogger(); err != nil {
		return fmt.Errorf("setting up logger: %w", err)
	}

	// 3. Setup telemetry (OpenTelemetry)
	if err := app.setupTelemetry(); err != nil {
		return fmt.Errorf("setting up telemetry: %w", err)
	}

	// 4. Setup config manager with reload callback
	if err := app.setupConfigManager(configPath); err != nil {
		return fmt.Errorf("setting up config manager: %w", err)
	}

	// 5. Initialize storage layer
	if err := app.initializeStorage(); err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	// 6. Initialize infrastructure clients
	if err := app.initializeClients(); err != nil {
		return fmt.Errorf("initializing clients: %w", err)
	}

	// 7. Initialize use cases
	if err := app.initializeUseCases(); err != nil {
		return fmt.Errorf("initializing use cases: %w", err)
	}

	// 8. Initialize HTTP handlers
	if err := app.initializeHandlers(); err != nil {
		return fmt.Errorf("initializing handlers: %w", err)
	}

	// 9. Setup HTTP server
	if err := app.setupServer(); err != nil {
		return fmt.Errorf("setting up server: %w", err)
	}

	return nil
}
