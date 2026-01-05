package app

import (
	"fmt"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/handler"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/server"
	pdUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/pagerduty"
	slackUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/slack"
)

func (app *Application) initializeHandlers() error {
	logger := &slogAdapter{logger: app.logger.Get()}

	// Create readiness handler with dependency checkers
	readyHandler := handler.NewReadyHandler()
	if app.dbPinger != nil {
		readyHandler.AddChecker("database", app.dbPinger)
	}

	app.handlers = &server.Handlers{
		Health:  handler.NewHealthHandler(),
		Ready:   readyHandler,
		Reload:  handler.NewReloadHandler(app.configManager, logger),
		Metrics: handler.NewMetricsHandler(),
	}

	// Alertmanager handler
	app.handlers.Alertmanager = handler.NewAlertmanagerHandler(
		app.useCases.ProcessAlert,
		logger,
	)

	// Slack handlers (if enabled)
	if app.config.IsSlackEnabled() {
		queryAlertStatusUC := slackUseCase.NewQueryAlertStatusUseCase(
			app.alertRepo,
		)

		app.handlers.SlackCommands = handler.NewSlackCommandsHandler(
			queryAlertStatusUC,
			app.logger.Get(),
		)

		handleSlackInteractionUC := slackUseCase.NewHandleInteractionUseCase(
			app.alertRepo,
			app.silenceRepo,
			app.useCases.SyncAck,
			app.clients.Slack,
			logger,
		)
		app.handlers.SlackInteraction = handler.NewSlackInteractionHandler(
			handleSlackInteractionUC,
			logger,
		)
		app.handlers.SlackEvents = handler.NewSlackEventsHandler(
			logger,
		)
	}

	// PagerDuty handler (if enabled)
	if app.config.IsPagerDutyEnabled() {
		handlePDWebhookUC := pdUseCase.NewHandleWebhookUseCase(
			app.alertRepo,
			app.useCases.SyncAck,
			app.clients.Slack,
			logger,
		)
		app.handlers.PagerDutyWebhook = handler.NewPagerDutyWebhookHandler(
			handlePDWebhookUC,
			logger,
		)
	}

	return nil
}

func (app *Application) setupServer() error {
	routerConfig := &server.RouterConfig{
		ConfigManager:             app.configManager, // Enable hot-reload
		AlertmanagerWebhookSecret: app.config.Alertmanager.WebhookSecret,
		SlackSigningSecret:        app.config.Slack.SigningSecret,
		PagerDutyWebhookSecret:    app.config.PagerDuty.WebhookSecret,
		RequestTimeout:            app.config.Server.RequestTimeout,
		Metrics:                   app.telemetry.Metrics,
	}
	router := server.NewRouterWithConfig(app.handlers, app.logger.Get(), routerConfig)
	srv, err := server.New(*app.config, router, app.logger.Get())
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Configure health check to report Slack status
	if app.config.IsSlackEnabled() && app.handlers.Health != nil {
		app.handlers.Health.SetSlackStatus(
			true,
			app.config.Slack.SocketMode.Enabled,
			srv.SocketModeClient(),
		)
	}

	app.server = srv
	return nil
}
