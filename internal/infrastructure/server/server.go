package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

// Server represents the HTTP server.
type Server struct {
	server *http.Server
	logger *slog.Logger
	cfg    config.ServerConfig
}

// New creates a new HTTP server.
func New(cfg config.ServerConfig, handler http.Handler, logger *slog.Logger) *Server {
	return &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
		logger: logger,
		cfg:    cfg,
	}
}

// Run starts the server and handles graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("starting server",
			"addr", s.server.Addr,
		)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		s.logger.Info("shutdown signal received")
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	s.logger.Info("shutting down server",
		"timeout", s.cfg.ShutdownTimeout,
	)

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
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
