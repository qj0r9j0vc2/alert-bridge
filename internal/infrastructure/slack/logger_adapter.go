package slack

import "log/slog"

// SlogAdapter adapts slog.Logger to the Logger interface.
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new slog adapter.
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: logger}
}

func (l *SlogAdapter) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg, fields...)
}

func (l *SlogAdapter) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg, fields...)
}

func (l *SlogAdapter) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg, fields...)
}

func (l *SlogAdapter) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg, fields...)
}
