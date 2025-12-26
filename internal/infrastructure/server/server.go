package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/slack"
)

// Server represents the HTTP server with optional Socket Mode client.
type Server struct {
	server           *http.Server
	logger           *slog.Logger
	cfg              config.ServerConfig
	socketModeClient *slack.SocketModeClient
}

// New creates a new HTTP server with optional Socket Mode client.
func New(cfg config.Config, handler http.Handler, logger *slog.Logger) (*Server, error) {
	s := &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:      handler,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
		logger: logger,
		cfg:    cfg.Server,
	}

	// Initialize Socket Mode client if enabled
	if cfg.Slack.Enabled && cfg.Slack.SocketMode.Enabled {
		logger.Info("initializing Socket Mode client",
			"debug", cfg.Slack.SocketMode.Debug,
			"ping_interval", cfg.Slack.SocketMode.PingInterval)

		slackLogger := slack.NewSlogAdapter(logger)
		client, err := slack.NewSocketModeClient(
			cfg.Slack.BotToken,
			cfg.Slack.SocketMode,
			slackLogger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Socket Mode client: %w", err)
		}

		s.socketModeClient = client
		logger.Info("Socket Mode client initialized successfully")
	}

	return s, nil
}

// Run starts the server and Socket Mode client, handling graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Buffer for both HTTP server and Socket Mode errors
	errChan := make(chan error, 2)

	// Start HTTP server
	go func() {
		s.logger.Info("starting HTTP server",
			"addr", s.server.Addr,
		)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Start Socket Mode client if enabled
	if s.socketModeClient != nil {
		go func() {
			s.logger.Info("starting Socket Mode client")
			if err := s.socketModeClient.Run(ctx); err != nil {
				// Log connection details on error
				if s.socketModeClient.IsConnected() {
					s.logger.Error("Socket Mode client error after connection",
						"error", err,
						"connection_id", s.socketModeClient.ConnectionID(),
						"last_reconnect", s.socketModeClient.LastReconnect())
				} else {
					s.logger.Error("Socket Mode client failed to connect",
						"error", err)
				}
				errChan <- fmt.Errorf("Socket Mode error: %w", err)
			}
		}()

		// Monitor Socket Mode connection status
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			// Initial connection check
			time.Sleep(2 * time.Second)
			if s.socketModeClient.IsConnected() {
				s.logger.Info("Socket Mode connected",
					"connection_id", s.socketModeClient.ConnectionID(),
					"connected_at", s.socketModeClient.LastReconnect())
			}

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if s.socketModeClient.IsConnected() {
						s.logger.Debug("Socket Mode health check",
							"connection_id", s.socketModeClient.ConnectionID(),
							"uptime", time.Since(s.socketModeClient.LastReconnect()))
					} else {
						s.logger.Warn("Socket Mode disconnected, waiting for reconnection")
					}
				}
			}
		}()
	}

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		s.logger.Info("shutdown signal received")
	case err := <-errChan:
		return err
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	s.logger.Info("shutting down server",
		"timeout", s.cfg.ShutdownTimeout,
	)

	// Shutdown HTTP server
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("HTTP server shutdown: %w", err)
	}

	// Socket Mode client will stop when context is cancelled
	if s.socketModeClient != nil {
		s.logger.Info("Socket Mode client stopped")
	}

	s.logger.Info("server stopped gracefully")
	return nil
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return s.server.Addr
}

// SetReadTimeout sets the read timeout.
func (s *Server) SetReadTimeout(d time.Duration) {
	s.server.ReadTimeout = d
}

// SetWriteTimeout sets the write timeout.
func (s *Server) SetWriteTimeout(d time.Duration) {
	s.server.WriteTimeout = d
}

// SocketModeClient returns the Socket Mode client (may be nil if not configured).
func (s *Server) SocketModeClient() *slack.SocketModeClient {
	return s.socketModeClient
}
