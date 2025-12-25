package server

import (
	"log/slog"
	"net/http"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/handler"
	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/handler/middleware"
)

// Handlers holds all HTTP handlers.
type Handlers struct {
	Alertmanager      *handler.AlertmanagerHandler
	SlackInteraction  *handler.SlackInteractionHandler
	SlackEvents       *handler.SlackEventsHandler
	PagerDutyWebhook  *handler.PagerDutyWebhookHandler
	Health            *handler.HealthHandler
}

// NewRouter creates the HTTP router with all handlers.
func NewRouter(handlers *Handlers, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.Handle("/health", handlers.Health)
	mux.Handle("/ready", handlers.Health)
	mux.Handle("/", handlers.Health) // Root path returns health

	// Webhook endpoints
	if handlers.Alertmanager != nil {
		mux.Handle("/webhook/alertmanager", handlers.Alertmanager)
	}

	if handlers.SlackInteraction != nil {
		mux.Handle("/webhook/slack/interactions", handlers.SlackInteraction)
	}

	if handlers.SlackEvents != nil {
		mux.Handle("/webhook/slack/events", handlers.SlackEvents)
	}

	if handlers.PagerDutyWebhook != nil {
		mux.Handle("/webhook/pagerduty", handlers.PagerDutyWebhook)
	}

	// Apply middleware stack
	var h http.Handler = mux
	h = middleware.RequestID(h)
	h = middleware.Logging(logger)(h)
	h = middleware.Recovery(logger)(h)

	return h
}
