package config

import (
	"fmt"
	"time"
)

// reloadableKeys defines the whitelist of configuration keys that can be hot-reloaded.
var reloadableKeys = map[string]bool{
	"logging.level":                 true,
	"logging.format":                true,
	"slack.channel_id":              true,
	"alerting.deduplication_window": true,
	"alerting.resend_interval":      true,
}

// staticKeys defines configuration keys that require application restart.
var staticKeys = map[string]string{
	"server.port":         "HTTP listener restart required",
	"storage.type":        "Storage backend initialization required",
	"storage.sqlite.path": "Database connection recreation required",
	"storage.mysql":       "Database connection pool recreation required",
}

// IsReloadable returns true if the given config key can be hot-reloaded.
func IsReloadable(key string) bool {
	return reloadableKeys[key]
}

// GetRestartReason returns the reason why a static config key requires restart.
func getRestartReason(key string) string {
	if reason, ok := staticKeys[key]; ok {
		return reason
	}
	return "unknown configuration requires restart"
}

// ValidateLogLevel checks if the log level is valid.
func ValidateLogLevel(level string) error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", level)
	}
	return nil
}

// ValidateLogFormat checks if the log format is valid.
func ValidateLogFormat(format string) error {
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[format] {
		return fmt.Errorf("invalid log format: %s (must be json or text)", format)
	}
	return nil
}

// ValidateNonEmpty checks if a string is non-empty.
func ValidateNonEmpty(value string, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	return nil
}

// ValidateDuration checks if a duration is greater than zero.
func ValidateDuration(duration time.Duration, fieldName string) error {
	if duration <= 0 {
		return fmt.Errorf("%s must be greater than 0", fieldName)
	}
	return nil
}

// ValidatePort checks if a port number is valid.
func ValidatePort(port int, fieldName string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", fieldName, port)
	}
	return nil
}

// ValidateStorageType checks if the storage type is valid.
func ValidateStorageType(storageType string) error {
	validTypes := map[string]bool{
		"memory": true,
		"sqlite": true,
		"mysql":  true,
	}
	if !validTypes[storageType] {
		return fmt.Errorf("invalid storage type: %s (must be memory, sqlite, or mysql)", storageType)
	}
	return nil
}

// Validate performs comprehensive validation on the configuration.
// Returns an error if any validation fails.
func (c *Config) Validate() error {
	var errors []string

	// Server validation
	if err := ValidatePort(c.Server.Port, "server.port"); err != nil {
		errors = append(errors, err.Error())
	}
	if err := ValidateDuration(c.Server.ReadTimeout, "server.read_timeout"); err != nil {
		errors = append(errors, err.Error())
	}
	if err := ValidateDuration(c.Server.WriteTimeout, "server.write_timeout"); err != nil {
		errors = append(errors, err.Error())
	}
	if err := ValidateDuration(c.Server.RequestTimeout, "server.request_timeout"); err != nil {
		errors = append(errors, err.Error())
	}
	if err := ValidateDuration(c.Server.ShutdownTimeout, "server.shutdown_timeout"); err != nil {
		errors = append(errors, err.Error())
	}

	// Logical constraint: RequestTimeout should be less than WriteTimeout
	if c.Server.RequestTimeout >= c.Server.WriteTimeout {
		errors = append(errors, "server.request_timeout must be less than server.write_timeout")
	}

	// Storage validation
	if err := ValidateStorageType(c.Storage.Type); err != nil {
		errors = append(errors, err.Error())
	}

	// SQLite-specific validation
	if c.Storage.Type == "sqlite" {
		if err := ValidateNonEmpty(c.Storage.SQLite.Path, "storage.sqlite.path"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// MySQL-specific validation
	if c.Storage.Type == "mysql" {
		if err := ValidateNonEmpty(c.Storage.MySQL.Primary.Host, "storage.mysql.primary.host"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidatePort(c.Storage.MySQL.Primary.Port, "storage.mysql.primary.port"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidateNonEmpty(c.Storage.MySQL.Primary.Database, "storage.mysql.primary.database"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidateNonEmpty(c.Storage.MySQL.Primary.Username, "storage.mysql.primary.username"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidateNonEmpty(c.Storage.MySQL.Primary.Password, "storage.mysql.primary.password"); err != nil {
			errors = append(errors, err.Error())
		}

		// Replica validation (if enabled)
		if c.Storage.MySQL.Replica.Enabled {
			if err := ValidateNonEmpty(c.Storage.MySQL.Replica.Host, "storage.mysql.replica.host"); err != nil {
				errors = append(errors, err.Error())
			}
			if err := ValidatePort(c.Storage.MySQL.Replica.Port, "storage.mysql.replica.port"); err != nil {
				errors = append(errors, err.Error())
			}
			if err := ValidateNonEmpty(c.Storage.MySQL.Replica.Database, "storage.mysql.replica.database"); err != nil {
				errors = append(errors, err.Error())
			}
			if err := ValidateNonEmpty(c.Storage.MySQL.Replica.Username, "storage.mysql.replica.username"); err != nil {
				errors = append(errors, err.Error())
			}
			if err := ValidateNonEmpty(c.Storage.MySQL.Replica.Password, "storage.mysql.replica.password"); err != nil {
				errors = append(errors, err.Error())
			}
		}

		// Connection pool validation
		if c.Storage.MySQL.Pool.MaxOpenConns < 1 {
			errors = append(errors, "storage.mysql.pool.max_open_conns must be at least 1")
		}
		if c.Storage.MySQL.Pool.MaxIdleConns < 0 {
			errors = append(errors, "storage.mysql.pool.max_idle_conns cannot be negative")
		}
		if c.Storage.MySQL.Pool.MaxIdleConns > c.Storage.MySQL.Pool.MaxOpenConns {
			errors = append(errors, "storage.mysql.pool.max_idle_conns cannot exceed max_open_conns")
		}
	}

	// Slack validation
	if c.IsSlackEnabled() {
		if err := ValidateNonEmpty(c.Slack.BotToken, "slack.bot_token"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidateNonEmpty(c.Slack.ChannelID, "slack.channel_id"); err != nil {
			errors = append(errors, err.Error())
		}

		// Socket Mode validation
		if c.Slack.SocketMode.Enabled {
			// Socket Mode requires app token
			if err := ValidateNonEmpty(c.Slack.SocketMode.AppToken, "slack.socket_mode.app_token"); err != nil {
				errors = append(errors, err.Error())
			}
			// Socket Mode requires ping interval
			if err := ValidateDuration(c.Slack.SocketMode.PingInterval, "slack.socket_mode.ping_interval"); err != nil {
				errors = append(errors, err.Error())
			}
			// Note: Signing secret is optional in Socket Mode (only used for HTTP webhooks)
		} else {
			// HTTP Mode requires signing secret
			if err := ValidateNonEmpty(c.Slack.SigningSecret, "slack.signing_secret"); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	// PagerDuty validation
	if c.IsPagerDutyEnabled() {
		if err := ValidateNonEmpty(c.PagerDuty.APIToken, "pagerduty.api_token"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidateNonEmpty(c.PagerDuty.RoutingKey, "pagerduty.routing_key"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidateNonEmpty(c.PagerDuty.ServiceID, "pagerduty.service_id"); err != nil {
			errors = append(errors, err.Error())
		}
		if err := ValidateNonEmpty(c.PagerDuty.FromEmail, "pagerduty.from_email"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Alerting validation
	if err := ValidateDuration(c.Alerting.DeduplicationWindow, "alerting.deduplication_window"); err != nil {
		errors = append(errors, err.Error())
	}
	if err := ValidateDuration(c.Alerting.ResendInterval, "alerting.resend_interval"); err != nil {
		errors = append(errors, err.Error())
	}

	// Logical constraint: ResendInterval should be greater than DeduplicationWindow
	if c.Alerting.ResendInterval <= c.Alerting.DeduplicationWindow {
		errors = append(errors, "alerting.resend_interval must be greater than alerting.deduplication_window")
	}

	// Silence durations validation
	for _, duration := range c.Alerting.SilenceDurations {
		if duration <= 0 {
			errors = append(errors, fmt.Sprintf("alerting.silence_durations contains invalid duration: %s", duration))
		}
	}

	// Logging validation
	if err := ValidateLogLevel(c.Logging.Level); err != nil {
		errors = append(errors, err.Error())
	}
	if err := ValidateLogFormat(c.Logging.Format); err != nil {
		errors = append(errors, err.Error())
	}

	// Return all validation errors
	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", joinErrors(errors))
	}

	return nil
}

// joinErrors joins multiple error messages with newlines and bullets.
func joinErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	result := errors[0]
	for i := 1; i < len(errors); i++ {
		result += "\n  - " + errors[i]
	}
	return result
}
