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
	Reload            *handler.ReloadHandler
}

// RouterConfig holds optional configuration for the router.
type RouterConfig struct {
	AlertmanagerWebhookSecret string
}

// NewRouter creates the HTTP router with all handlers (backward compatible).
func NewRouter(handlers *Handlers, logger *slog.Logger) http.Handler {
	return NewRouterWithConfig(handlers, logger, nil)
}

// NewRouterWithConfig creates the HTTP router with all handlers and optional config.
func NewRouterWithConfig(handlers *Handlers, logger *slog.Logger, cfg *RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.Handle("/health", handlers.Health)
	mux.Handle("/ready", handlers.Health)
	mux.Handle("/", handlers.Health) // Root path returns health

	// Admin endpoints
	if handlers.Reload != nil {
		mux.Handle("/-/reload", handlers.Reload)
	}

	// Webhook endpoints
	if handlers.Alertmanager != nil {
		var h http.Handler = handlers.Alertmanager

		// Apply authentication middleware if secret is configured
		if cfg != nil && cfg.AlertmanagerWebhookSecret != "" {
			h = middleware.AlertmanagerAuth(cfg.AlertmanagerWebhookSecret, logger)(h)
			logger.Info("Alertmanager webhook authentication enabled")
		}

		mux.Handle("/webhook/alertmanager", h)
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
