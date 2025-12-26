package app

import (
	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/handler"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/server"
	pdUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/pagerduty"
	slackUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/slack"
)

func (app *Application) initializeHandlers() error {
	logger := &slogAdapter{logger: app.logger.Get()}

	app.handlers = &server.Handlers{
		Health:  handler.NewHealthHandler(),
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
	app.server = server.New(app.config.Server, router, app.logger.Get())
	return nil
}
