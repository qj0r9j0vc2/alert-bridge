package slack

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Logger interface for structured logging.
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// SocketModeClient wraps Slack's Socket Mode client with reconnection logic.
type SocketModeClient struct {
	client             *socketmode.Client
	slackAPI           *slack.Client
	cfg                config.SocketModeConfig
	logger             Logger
	reconnectCfg       ReconnectionConfig
	circuitBreaker     *CircuitBreaker
	eventHandler       EventHandler
	commandHandler     CommandHandler
	interactionHandler InteractionHandler
	isConnected        bool
	connectionID       string
	lastReconnect      time.Time
}

// EventHandler handles Slack events.
type EventHandler interface {
	HandleEvent(evt *socketmode.Event) error
}

// CommandHandler handles slash commands.
type CommandHandler interface {
	HandleCommand(cmd *slack.SlashCommand) error
}

// InteractionHandler handles interactive components.
type InteractionHandler interface {
	HandleInteraction(callback *slack.InteractionCallback) error
}

// NewSocketModeClient creates a new Socket Mode client.
func NewSocketModeClient(botToken string, cfg config.SocketModeConfig, logger Logger) (*SocketModeClient, error) {
	if cfg.AppToken == "" {
		return nil, fmt.Errorf("socket mode app token is required")
	}

	if botToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	// Create Slack API client
	slackAPI := slack.New(
		botToken,
		slack.OptionDebug(cfg.Debug),
		slack.OptionAppLevelToken(cfg.AppToken),
	)

	// Create Socket Mode client
	socketClient := socketmode.New(
		slackAPI,
		socketmode.OptionDebug(cfg.Debug),
	)

	return &SocketModeClient{
		client:         socketClient,
		slackAPI:       slackAPI,
		cfg:            cfg,
		logger:         logger,
		reconnectCfg:   DefaultReconnectionConfig(),
		circuitBreaker: NewCircuitBreaker(5),
		isConnected:    false,
	}, nil
}

// SetEventHandler sets the event handler.
func (c *SocketModeClient) SetEventHandler(handler EventHandler) {
	c.eventHandler = handler
}

// SetCommandHandler sets the command handler.
func (c *SocketModeClient) SetCommandHandler(handler CommandHandler) {
	c.commandHandler = handler
}

// SetInteractionHandler sets the interaction handler.
func (c *SocketModeClient) SetInteractionHandler(handler InteractionHandler) {
	c.interactionHandler = handler
}

// Connect establishes connection to Slack via Socket Mode with automatic reconnection.
func (c *SocketModeClient) Connect(ctx context.Context) error {
	attempt := 0

	for {
		// Check circuit breaker
		if !ShouldRetry(c.circuitBreaker) {
			c.logger.Error("Circuit breaker is open, stopping reconnection attempts",
				"consecutive_failures", c.circuitBreaker.ConsecutiveFailures())
			return fmt.Errorf("circuit breaker open after %d consecutive failures", c.circuitBreaker.ConsecutiveFailures())
		}

		// Attempt connection
		err := c.attemptConnection(ctx)
		if err == nil {
			// Connection successful
			c.circuitBreaker.RecordSuccess()
			c.isConnected = true
			c.lastReconnect = time.Now()
			c.logger.Info("Successfully connected to Slack via Socket Mode",
				"connection_id", c.connectionID,
				"attempt", attempt+1)

			// Start event loop
			go c.runEventLoop(ctx)
			return nil
		}

		// Connection failed
		c.logger.Warn("Failed to connect to Slack via Socket Mode",
			"error", err.Error(),
			"attempt", attempt+1)

		circuitOpen := c.circuitBreaker.RecordFailure()
		if circuitOpen {
			c.logger.Error("Circuit breaker opened after consecutive failures",
				"failures", c.circuitBreaker.ConsecutiveFailures())
			return fmt.Errorf("circuit breaker opened: %w", err)
		}

		// Calculate backoff
		backoff := CalculateBackoff(c.reconnectCfg, attempt)
		c.logger.Info("Waiting before retry",
			"backoff", backoff.String(),
			"next_attempt", attempt+2)

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}

		attempt++
	}
}

// attemptConnection attempts to establish a single connection.
func (c *SocketModeClient) attemptConnection(ctx context.Context) error {
	// Test authentication
	authTest, err := c.slackAPI.AuthTestContext(ctx)
	if err != nil {
		return fmt.Errorf("auth test failed: %w", err)
	}

	c.connectionID = authTest.TeamID
	c.logger.Debug("Auth test passed",
		"team_id", authTest.TeamID,
		"user_id", authTest.UserID)

	return nil
}

// runEventLoop processes events from Socket Mode.
func (c *SocketModeClient) runEventLoop(ctx context.Context) {
	c.logger.Info("Starting Socket Mode event loop")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping event loop")
			c.isConnected = false
			return

		case evt := <-c.client.Events:
			c.handleSocketModeEvent(evt)
		}
	}
}

// handleSocketModeEvent routes Socket Mode events to appropriate handlers.
func (c *SocketModeClient) handleSocketModeEvent(evt socketmode.Event) {
	c.logger.Debug("Received Socket Mode event", "type", evt.Type)

	switch evt.Type {
	case socketmode.EventTypeConnecting:
		c.logger.Info("Connecting to Slack...")

	case socketmode.EventTypeConnectionError:
		c.logger.Error("Connection error", "error", evt.Data)

	case socketmode.EventTypeConnected:
		c.logger.Info("Connected to Slack via Socket Mode!")

	case socketmode.EventTypeSlashCommand:
		// Handle slash command
		cmd, ok := evt.Data.(slack.SlashCommand)
		if !ok {
			c.logger.Error("Failed to cast slash command event")
			return
		}

		if c.commandHandler != nil {
			if err := c.commandHandler.HandleCommand(&cmd); err != nil {
				c.logger.Error("Failed to handle slash command",
					"command", cmd.Command,
					"error", err.Error())
			}
		}

		// Acknowledge the event
		c.client.Ack(*evt.Request)

	case socketmode.EventTypeInteractive:
		// Handle interactive component
		callback, ok := evt.Data.(slack.InteractionCallback)
		if !ok {
			c.logger.Error("Failed to cast interaction callback event")
			return
		}

		if c.interactionHandler != nil {
			if err := c.interactionHandler.HandleInteraction(&callback); err != nil {
				c.logger.Error("Failed to handle interaction",
					"type", callback.Type,
					"error", err.Error())
			}
		}

		// Acknowledge the event
		c.client.Ack(*evt.Request)

	case socketmode.EventTypeEventsAPI:
		// Handle Events API
		eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			c.logger.Error("Failed to cast events API event")
			return
		}

		// Acknowledge the event first
		c.client.Ack(*evt.Request, eventsAPI)

		// Route to event handler
		if c.eventHandler != nil {
			if err := c.eventHandler.HandleEvent(&evt); err != nil {
				c.logger.Error("Failed to handle event",
					"type", eventsAPI.Type,
					"error", err.Error())
			}
		}

	default:
		c.logger.Debug("Unhandled event type", "type", evt.Type)
	}
}

// Run starts the Socket Mode client (blocking call).
func (c *SocketModeClient) Run(ctx context.Context) error {
	c.logger.Info("Starting Socket Mode client")

	// Connect first
	if err := c.Connect(ctx); err != nil {
		return err
	}

	// Run the client (blocking)
	return c.client.RunContext(ctx)
}

// IsConnected returns true if the client is currently connected.
func (c *SocketModeClient) IsConnected() bool {
	return c.isConnected
}

// ConnectionID returns the current connection ID.
func (c *SocketModeClient) ConnectionID() string {
	return c.connectionID
}

// LastReconnect returns the timestamp of the last successful reconnection.
func (c *SocketModeClient) LastReconnect() time.Time {
	return c.lastReconnect
}

// SlackAPI returns the underlying Slack API client.
func (c *SocketModeClient) SlackAPI() *slack.Client {
	return c.slackAPI
}

// SimpleLogger implements Logger interface using standard log package.
type SimpleLogger struct{}

func (l *SimpleLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, fields)
}

func (l *SimpleLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, fields)
}

func (l *SimpleLogger) Warn(msg string, fields ...interface{}) {
	log.Printf("[WARN] %s %v", msg, fields)
}

func (l *SimpleLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}
