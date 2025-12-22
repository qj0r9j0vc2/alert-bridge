package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Slack     SlackConfig     `yaml:"slack"`
	PagerDuty PagerDutyConfig `yaml:"pagerduty"`
	Alerting  AlertingConfig  `yaml:"alerting"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// SlackConfig holds Slack integration settings.
type SlackConfig struct {
	Enabled       bool   `yaml:"enabled"`
	BotToken      string `yaml:"bot_token"`
	SigningSecret string `yaml:"signing_secret"`
	ChannelID     string `yaml:"channel_id"`
	AppID         string `yaml:"app_id"`
}

// PagerDutyConfig holds PagerDuty integration settings.
type PagerDutyConfig struct {
	Enabled        bool   `yaml:"enabled"`
	APIToken       string `yaml:"api_token"`
	RoutingKey     string `yaml:"routing_key"`
	ServiceID      string `yaml:"service_id"`
	WebhookSecret  string `yaml:"webhook_secret"`
	FromEmail      string `yaml:"from_email"`
	DefaultSeverity string `yaml:"default_severity"`
}

// AlertingConfig holds alerting behavior settings.
type AlertingConfig struct {
	DeduplicationWindow time.Duration   `yaml:"deduplication_window"`
	ResendInterval      time.Duration   `yaml:"resend_interval"`
	SilenceDurations    []time.Duration `yaml:"silence_durations"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load reads configuration from file and environment.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	// Load from file if exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		if err == nil {
			// Expand environment variables in YAML
			expandedData := os.ExpandEnv(string(data))
			if err := yaml.Unmarshal([]byte(expandedData), cfg); err != nil {
				return nil, fmt.Errorf("parsing config file: %w", err)
			}
		}
	}

	// Override with environment variables
	cfg.overrideFromEnv()

	// Apply defaults
	cfg.applyDefaults()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// overrideFromEnv overrides config values from environment variables.
func (c *Config) overrideFromEnv() {
	// Server
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}

	// Slack
	if v := os.Getenv("SLACK_ENABLED"); v != "" {
		c.Slack.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("SLACK_BOT_TOKEN"); v != "" {
		c.Slack.BotToken = v
	}
	if v := os.Getenv("SLACK_SIGNING_SECRET"); v != "" {
		c.Slack.SigningSecret = v
	}
	if v := os.Getenv("SLACK_CHANNEL_ID"); v != "" {
		c.Slack.ChannelID = v
	}
	if v := os.Getenv("SLACK_APP_ID"); v != "" {
		c.Slack.AppID = v
	}

	// PagerDuty
	if v := os.Getenv("PAGERDUTY_ENABLED"); v != "" {
		c.PagerDuty.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("PAGERDUTY_API_TOKEN"); v != "" {
		c.PagerDuty.APIToken = v
	}
	if v := os.Getenv("PAGERDUTY_ROUTING_KEY"); v != "" {
		c.PagerDuty.RoutingKey = v
	}
	if v := os.Getenv("PAGERDUTY_SERVICE_ID"); v != "" {
		c.PagerDuty.ServiceID = v
	}
	if v := os.Getenv("PAGERDUTY_WEBHOOK_SECRET"); v != "" {
		c.PagerDuty.WebhookSecret = v
	}
	if v := os.Getenv("PAGERDUTY_FROM_EMAIL"); v != "" {
		c.PagerDuty.FromEmail = v
	}
	if v := os.Getenv("PAGERDUTY_DEFAULT_SEVERITY"); v != "" {
		c.PagerDuty.DefaultSeverity = v
	}

	// Logging
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		c.Logging.Format = v
	}
}

// applyDefaults sets default values for unset config options.
func (c *Config) applyDefaults() {
	// Server defaults
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 5 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 10 * time.Second
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = 30 * time.Second
	}

	// Alerting defaults
	if c.Alerting.DeduplicationWindow == 0 {
		c.Alerting.DeduplicationWindow = 5 * time.Minute
	}
	if c.Alerting.ResendInterval == 0 {
		c.Alerting.ResendInterval = 30 * time.Minute
	}
	if len(c.Alerting.SilenceDurations) == 0 {
		c.Alerting.SilenceDurations = []time.Duration{
			15 * time.Minute,
			1 * time.Hour,
			4 * time.Hour,
			24 * time.Hour,
		}
	}

	// PagerDuty defaults
	if c.PagerDuty.DefaultSeverity == "" {
		c.PagerDuty.DefaultSeverity = "warning"
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
}

// validate checks that required configuration is present.
func (c *Config) validate() error {
	if c.Slack.Enabled {
		if c.Slack.BotToken == "" {
			return fmt.Errorf("slack.bot_token is required when slack is enabled")
		}
		if c.Slack.SigningSecret == "" {
			return fmt.Errorf("slack.signing_secret is required when slack is enabled")
		}
		if c.Slack.ChannelID == "" {
			return fmt.Errorf("slack.channel_id is required when slack is enabled")
		}
	}

	if c.PagerDuty.Enabled {
		// Either routing key (Events API v2) or API token (REST API) is needed
		if c.PagerDuty.RoutingKey == "" && c.PagerDuty.APIToken == "" {
			return fmt.Errorf("pagerduty.routing_key or pagerduty.api_token is required when pagerduty is enabled")
		}
	}

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(c.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	// Validate log format
	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[strings.ToLower(c.Logging.Format)] {
		return fmt.Errorf("invalid log format: %s (must be json or text)", c.Logging.Format)
	}

	return nil
}

// IsSlackEnabled returns true if Slack integration is enabled.
func (c *Config) IsSlackEnabled() bool {
	return c.Slack.Enabled
}

// IsPagerDutyEnabled returns true if PagerDuty integration is enabled.
func (c *Config) IsPagerDutyEnabled() bool {
	return c.PagerDuty.Enabled
}
