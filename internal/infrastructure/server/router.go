package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/handler"
	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/handler/middleware"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/observability"
)

// Handlers holds all HTTP handlers.
type Handlers struct {
	Alertmanager     *handler.AlertmanagerHandler
	SlackCommands    *handler.SlackCommandsHandler
	SlackInteraction *handler.SlackInteractionHandler
	SlackEvents      *handler.SlackEventsHandler
	PagerDutyWebhook *handler.PagerDutyWebhookHandler
	Health           *handler.HealthHandler
	Ready            *handler.ReadyHandler
	Reload           *handler.ReloadHandler
	Metrics          *handler.MetricsHandler
}

// RouterConfig holds optional configuration for the router.
type RouterConfig struct {
	// Config manager for hot-reload support
	ConfigManager *config.ConfigManager
	// Static configuration (backward compatibility)
	AlertmanagerWebhookSecret string
	SlackSigningSecret        string
	PagerDutyWebhookSecret    string
	RequestTimeout            time.Duration
	Metrics                   *observability.Metrics
}

// NewRouter creates the HTTP router with all handlers (backward compatible).
func NewRouter(handlers *Handlers, logger *slog.Logger) http.Handler {
	return NewRouterWithConfig(handlers, logger, nil)
}

// NewRouterWithConfig creates the HTTP router with all handlers and optional config.
func NewRouterWithConfig(handlers *Handlers, logger *slog.Logger, cfg *RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// Health check endpoints (liveness)
	mux.Handle("/health", handlers.Health)
	mux.Handle("/", handlers.Health) // Root path returns health

	// Readiness check endpoint (checks dependencies)
	if handlers.Ready != nil {
		mux.Handle("/ready", handlers.Ready)
	} else {
		// Fallback to health handler if Ready handler not configured
		mux.Handle("/ready", handlers.Health)
	}

	// Observability endpoints
	if handlers.Metrics != nil {
		mux.Handle("/metrics", handlers.Metrics)
	}

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

	if handlers.SlackCommands != nil {
		var h http.Handler = handlers.SlackCommands

		// Apply Slack authentication middleware
		if cfg != nil && cfg.SlackSigningSecret != "" {
			h = middleware.SlackAuth(cfg.SlackSigningSecret, logger)(h)
			logger.Info("Slack commands webhook authentication enabled")
		}

		mux.Handle("/webhook/slack/commands", h)
	}

	if handlers.SlackInteraction != nil {
		var h http.Handler = handlers.SlackInteraction

		// Apply Slack authentication middleware
		if cfg != nil && cfg.SlackSigningSecret != "" {
			h = middleware.SlackAuth(cfg.SlackSigningSecret, logger)(h)
			logger.Info("Slack interactions webhook authentication enabled")
		}

		mux.Handle("/webhook/slack/interactions", h)
	}

	if handlers.SlackEvents != nil {
		var h http.Handler = handlers.SlackEvents

		// Apply Slack authentication middleware
		if cfg != nil && cfg.SlackSigningSecret != "" {
			h = middleware.SlackAuth(cfg.SlackSigningSecret, logger)(h)
			logger.Info("Slack events webhook authentication enabled")
		}

		mux.Handle("/webhook/slack/events", h)
	}

	if handlers.PagerDutyWebhook != nil {
		var h http.Handler = handlers.PagerDutyWebhook

		// Apply PagerDuty authentication middleware with hot-reload support
		if cfg != nil {
			// Create getter function that reads secret from config on each request
			var secretGetter middleware.WebhookSecretGetter
			if cfg.ConfigManager != nil {
				// Hot-reload enabled: read from ConfigManager on each request
				secretGetter = func() string {
					return cfg.ConfigManager.Get().PagerDuty.WebhookSecret
				}
			} else {
				// Backward compatibility: use static secret
				secretGetter = func() string {
					return cfg.PagerDutyWebhookSecret
				}
			}
			h = middleware.PagerDutyAuth(secretGetter, logger)(h)
			logger.Info("PagerDuty webhook authentication enabled",
				"hot_reload", cfg.ConfigManager != nil,
			)
		}

		mux.Handle("/webhook/pagerduty", h)
	}

	// Apply middleware stack
	var h http.Handler = mux
	h = middleware.RequestID(h)
	h = middleware.Logging(logger)(h)
	h = middleware.Recovery(logger)(h)

	// Apply observability metrics middleware if configured
	if cfg != nil && cfg.Metrics != nil {
		h = middleware.Observability(cfg.Metrics)(h)
		logger.Info("observability metrics middleware enabled")
	}

	// Apply request timeout if configured
	if cfg != nil && cfg.RequestTimeout > 0 {
		h = middleware.Timeout(cfg.RequestTimeout, logger)(h)
	}

	return h
}
