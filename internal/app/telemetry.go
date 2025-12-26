package app

import (
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/observability"
)

// setupTelemetry initializes OpenTelemetry tracing and metrics.
func (app *Application) setupTelemetry() error {
	telemetry, err := observability.NewTelemetry("alert-bridge", "v1.0.0")
	if err != nil {
		return err
	}

	app.telemetry = telemetry

	app.logger.Get().Info("telemetry initialized",
		"service", "alert-bridge",
		"metrics_enabled", true,
		"tracing_enabled", false, // NoOp tracer for now
	)

	return nil
}
