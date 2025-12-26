package app

import (
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/pagerduty"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/slack"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/ack"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/alert"
)

// Clients holds all external integration clients
type Clients struct {
	Notifiers []alert.Notifier
	Syncers   []ack.AckSyncer
	Slack     *slack.Client
	PagerDuty *pagerduty.Client
}

func (app *Application) initializeClients() error {
	app.clients = &Clients{
		Notifiers: make([]alert.Notifier, 0),
		Syncers:   make([]ack.AckSyncer, 0),
	}

	logger := &slogAdapter{logger: app.logger.Get()}
	retryPolicy := alert.DefaultRetryPolicy()

	if app.config.IsSlackEnabled() {
		app.clients.Slack = slack.NewClient(
			app.config.Slack.BotToken,
			app.config.Slack.ChannelID,
			app.config.Alerting.SilenceDurations,
		)

		// Wrap with retry logic
		retryableSlack := alert.NewRetryableNotifier(app.clients.Slack, retryPolicy, logger)
		app.clients.Notifiers = append(app.clients.Notifiers, retryableSlack)

		app.logger.Get().Info("Slack integration enabled",
			"channel", app.config.Slack.ChannelID,
		)
	}

	if app.config.IsPagerDutyEnabled() {
		app.clients.PagerDuty = pagerduty.NewClient(
			app.config.PagerDuty.APIToken,
			app.config.PagerDuty.RoutingKey,
			app.config.PagerDuty.ServiceID,
			app.config.PagerDuty.FromEmail,
			app.config.PagerDuty.DefaultSeverity,
		)

		// Wrap with retry logic
		retryablePagerDuty := alert.NewRetryableNotifier(app.clients.PagerDuty, retryPolicy, logger)
		app.clients.Notifiers = append(app.clients.Notifiers, retryablePagerDuty)
		app.clients.Syncers = append(app.clients.Syncers, app.clients.PagerDuty)

		app.logger.Get().Info("PagerDuty integration enabled")
	}

	return nil
}
