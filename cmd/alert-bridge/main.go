package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/qj0r9j0vc2/alert-bridge/internal/adapter/handler"
	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/repository"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/pagerduty"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/persistence/memory"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/persistence/mysql"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/persistence/sqlite"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/server"
	infraslack "github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/slack"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/ack"
	"github.com/qj0r9j0vc2/alert-bridge/internal/usecase/alert"
	pdUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/pagerduty"
	slackUseCase "github.com/qj0r9j0vc2/alert-bridge/internal/usecase/slack"
)

func main() {
	// Setup logger
	logger := setupLogger("info", "json")

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("configuration loaded",
		"slack_enabled", cfg.IsSlackEnabled(),
		"pagerduty_enabled", cfg.IsPagerDutyEnabled(),
		"storage_type", cfg.Storage.Type,
		"server_port", cfg.Server.Port,
	)

	// Initialize repositories based on storage type
	var alertRepo repository.AlertRepository
	var ackEventRepo repository.AckEventRepository
	var silenceRepo repository.SilenceRepository
	var sqliteDB *sqlite.DB
	var mysqlDB *mysql.DB

	switch cfg.Storage.Type {
	case "mysql":
		// Initialize MySQL database
		repos, db, err := mysql.NewRepositories(&cfg.Storage.MySQL)
		if err != nil {
			logger.Error("failed to initialize MySQL database", "error", err)
			os.Exit(1)
		}
		mysqlDB = db

		// Assign repositories
		alertRepo = repos.Alert
		ackEventRepo = repos.AckEvent
		silenceRepo = repos.Silence

		logger.Info("MySQL storage initialized",
			"host", cfg.Storage.MySQL.Primary.Host,
			"database", cfg.Storage.MySQL.Primary.Database,
			"pool_max_open", cfg.Storage.MySQL.Pool.MaxOpenConns,
			"pool_max_idle", cfg.Storage.MySQL.Pool.MaxIdleConns,
		)

	case "sqlite":
		// Initialize SQLite database
		var err error
		sqliteDB, err = sqlite.NewDB(cfg.Storage.SQLite.Path)
		if err != nil {
			logger.Error("failed to initialize SQLite database", "error", err, "path", cfg.Storage.SQLite.Path)
			os.Exit(1)
		}

		// Run migrations
		if err := sqliteDB.Migrate(context.Background()); err != nil {
			logger.Error("failed to run database migrations", "error", err)
			sqliteDB.Close()
			os.Exit(1)
		}

		// Create repositories
		repos := sqlite.NewRepositories(sqliteDB.DB)
		alertRepo = repos.Alert
		ackEventRepo = repos.AckEvent
		silenceRepo = repos.Silence

		logger.Info("SQLite storage initialized", "path", cfg.Storage.SQLite.Path)

	case "memory", "":
		// Use in-memory repositories (default)
		alertRepo = memory.NewAlertRepository()
		ackEventRepo = memory.NewAckEventRepository()
		silenceRepo = memory.NewSilenceRepository()

		logger.Info("in-memory storage initialized")

	default:
		logger.Error("unknown storage type", "type", cfg.Storage.Type)
		os.Exit(1)
	}

	// Initialize infrastructure clients
	var notifiers []alert.Notifier
	var syncers []ack.AckSyncer
	var slackClient *infraslack.Client
	var pdClient *pagerduty.Client

	if cfg.IsSlackEnabled() {
		slackClient = infraslack.NewClient(
			cfg.Slack.BotToken,
			cfg.Slack.ChannelID,
			cfg.Alerting.SilenceDurations,
		)
		notifiers = append(notifiers, slackClient)
		logger.Info("Slack integration enabled",
			"channel", cfg.Slack.ChannelID,
		)
	}

	if cfg.IsPagerDutyEnabled() {
		pdClient = pagerduty.NewClient(
			cfg.PagerDuty.APIToken,
			cfg.PagerDuty.RoutingKey,
			cfg.PagerDuty.ServiceID,
			cfg.PagerDuty.FromEmail,
			cfg.PagerDuty.DefaultSeverity,
		)
		notifiers = append(notifiers, pdClient)
		syncers = append(syncers, pdClient)
		logger.Info("PagerDuty integration enabled")
	}

	// Create a slog adapter for use cases
	useCaseLogger := &slogAdapter{logger: logger}

	// Initialize use cases
	syncAckUC := ack.NewSyncAckUseCase(alertRepo, ackEventRepo, syncers, useCaseLogger)
	processAlertUC := alert.NewProcessAlertUseCase(alertRepo, silenceRepo, notifiers, useCaseLogger)

	// Initialize handlers
	handlers := &server.Handlers{
		Health: handler.NewHealthHandler(),
	}

	// Alertmanager handler
	handlers.Alertmanager = handler.NewAlertmanagerHandler(processAlertUC, useCaseLogger)

	// Slack handlers
	if cfg.IsSlackEnabled() {
		handleSlackInteractionUC := slackUseCase.NewHandleInteractionUseCase(
			alertRepo,
			silenceRepo,
			syncAckUC,
			slackClient,
			useCaseLogger,
		)
		handlers.SlackInteraction = handler.NewSlackInteractionHandler(
			handleSlackInteractionUC,
			cfg.Slack.SigningSecret,
			useCaseLogger,
		)
		handlers.SlackEvents = handler.NewSlackEventsHandler(
			cfg.Slack.SigningSecret,
			useCaseLogger,
		)
	}

	// PagerDuty handler
	if cfg.IsPagerDutyEnabled() {
		handlePDWebhookUC := pdUseCase.NewHandleWebhookUseCase(
			alertRepo,
			syncAckUC,
			slackClient, // May be nil if Slack is disabled
			useCaseLogger,
		)
		handlers.PagerDutyWebhook = handler.NewPagerDutyWebhookHandler(
			handlePDWebhookUC,
			cfg.PagerDuty.WebhookSecret,
			useCaseLogger,
		)
	}

	// Setup router and server
	router := server.NewRouter(handlers, logger)
	srv := server.New(cfg.Server, router, logger)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("starting alert-bridge",
		"port", cfg.Server.Port,
	)

	if err := srv.Run(ctx); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	// Close MySQL database if it was initialized
	if mysqlDB != nil {
		if err := mysqlDB.Close(); err != nil {
			logger.Error("failed to close MySQL database", "error", err)
		} else {
			logger.Info("MySQL database closed successfully")
		}
	}

	// Close SQLite database if it was initialized
	if sqliteDB != nil {
		if err := sqliteDB.Close(); err != nil {
			logger.Error("failed to close SQLite database", "error", err)
		} else {
			logger.Info("SQLite database closed successfully")
		}
	}

	logger.Info("alert-bridge stopped")
}

// setupLogger creates and configures the logger.
func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// slogAdapter adapts slog.Logger to the alert.Logger interface.
type slogAdapter struct {
	logger *slog.Logger
}

func (a *slogAdapter) Debug(msg string, keysAndValues ...any) {
	a.logger.Debug(msg, keysAndValues...)
}

func (a *slogAdapter) Info(msg string, keysAndValues ...any) {
	a.logger.Info(msg, keysAndValues...)
}

func (a *slogAdapter) Warn(msg string, keysAndValues ...any) {
	a.logger.Warn(msg, keysAndValues...)
}

func (a *slogAdapter) Error(msg string, keysAndValues ...any) {
	a.logger.Error(msg, keysAndValues...)
}
