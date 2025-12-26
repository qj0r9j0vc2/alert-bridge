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
	Server       ServerConfig       `yaml:"server"`
	Storage      StorageConfig      `yaml:"storage"`
	Slack        SlackConfig        `yaml:"slack"`
	PagerDuty    PagerDutyConfig    `yaml:"pagerduty"`
	Alerting     AlertingConfig     `yaml:"alerting"`
	Logging      LoggingConfig      `yaml:"logging"`
	Alertmanager AlertmanagerConfig `yaml:"alertmanager"`
}

// StorageConfig holds persistence storage settings.
type StorageConfig struct {
	Type   string       `yaml:"type"` // "memory", "sqlite", or "mysql"
	SQLite SQLiteConfig `yaml:"sqlite"`
	MySQL  MySQLConfig  `yaml:"mysql"`
}

// SQLiteConfig holds SQLite-specific settings.
type SQLiteConfig struct {
	Path string `yaml:"path"` // Database file path, use ":memory:" for in-memory
}

// MySQLConfig holds MySQL-specific settings.
type MySQLConfig struct {
	Primary   MySQLInstanceConfig `yaml:"primary"`
	Replica   MySQLReplicaConfig  `yaml:"replica"`
	Pool      MySQLPoolConfig     `yaml:"pool"`
	Timeout   time.Duration       `yaml:"timeout"`
	ParseTime bool                `yaml:"parse_time"`
	Charset   string              `yaml:"charset"`
}

// MySQLInstanceConfig holds MySQL instance connection settings.
type MySQLInstanceConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// MySQLReplicaConfig holds MySQL replica settings.
type MySQLReplicaConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// MySQLPoolConfig holds MySQL connection pool settings.
type MySQLPoolConfig struct {
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	RequestTimeout  time.Duration `yaml:"request_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// SlackConfig holds Slack integration settings.
type SlackConfig struct {
	Enabled       bool             `yaml:"enabled"`
	BotToken      string           `yaml:"bot_token"`
	SigningSecret string           `yaml:"signing_secret"`
	ChannelID     string           `yaml:"channel_id"`
	AppID         string           `yaml:"app_id"`
	APIURL        string           `yaml:"api_url,omitempty"` // Optional: for E2E testing with mock services
	SocketMode    SocketModeConfig `yaml:"socket_mode"`
}

// SocketModeConfig holds Socket Mode settings for local development.
type SocketModeConfig struct {
	Enabled      bool          `yaml:"enabled"`
	AppToken     string        `yaml:"app_token"`
	Debug        bool          `yaml:"debug"`
	PingInterval time.Duration `yaml:"ping_interval"`
}

// PagerDutyConfig holds PagerDuty integration settings.
type PagerDutyConfig struct {
	Enabled         bool   `yaml:"enabled"`
	APIToken        string `yaml:"api_token"`
	RoutingKey      string `yaml:"routing_key"`
	ServiceID       string `yaml:"service_id"`
	WebhookSecret   string `yaml:"webhook_secret"`
	FromEmail       string `yaml:"from_email"`
	DefaultSeverity string `yaml:"default_severity"`
	APIURL          string `yaml:"api_url,omitempty"` // Optional: for E2E testing with mock services
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

// AlertmanagerConfig holds Alertmanager webhook settings.
type AlertmanagerConfig struct {
	WebhookSecret string   `yaml:"webhook_secret"`
	AllowedIPs    []string `yaml:"allowed_ips"` // Optional IP whitelist (not yet implemented)
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
	if err := cfg.Validate(); err != nil {
		return nil, err
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

	// Slack Socket Mode
	if v := os.Getenv("SLACK_SOCKET_MODE_ENABLED"); v != "" {
		c.Slack.SocketMode.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("SLACK_SOCKET_MODE_APP_TOKEN"); v != "" {
		c.Slack.SocketMode.AppToken = v
	}
	if v := os.Getenv("SLACK_SOCKET_MODE_DEBUG"); v != "" {
		c.Slack.SocketMode.Debug = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("SLACK_SOCKET_MODE_PING_INTERVAL"); v != "" {
		if duration, err := time.ParseDuration(v); err == nil {
			c.Slack.SocketMode.PingInterval = duration
		}
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

	// Alertmanager
	if v := os.Getenv("ALERTMANAGER_WEBHOOK_SECRET"); v != "" {
		c.Alertmanager.WebhookSecret = v
	}

	// Storage
	if v := os.Getenv("STORAGE_TYPE"); v != "" {
		c.Storage.Type = v
	}
	if v := os.Getenv("SQLITE_DATABASE_PATH"); v != "" {
		c.Storage.SQLite.Path = v
	}

	// MySQL
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		c.Storage.MySQL.Primary.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Storage.MySQL.Primary.Port = port
		}
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		c.Storage.MySQL.Primary.Database = v
	}
	if v := os.Getenv("MYSQL_USERNAME"); v != "" {
		c.Storage.MySQL.Primary.Username = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		c.Storage.MySQL.Primary.Password = v
	}
	if v := os.Getenv("MYSQL_MAX_OPEN_CONNS"); v != "" {
		if conns, err := strconv.Atoi(v); err == nil {
			c.Storage.MySQL.Pool.MaxOpenConns = conns
		}
	}
	if v := os.Getenv("MYSQL_MAX_IDLE_CONNS"); v != "" {
		if conns, err := strconv.Atoi(v); err == nil {
			c.Storage.MySQL.Pool.MaxIdleConns = conns
		}
	}
	if v := os.Getenv("MYSQL_CONN_MAX_LIFETIME"); v != "" {
		if duration, err := time.ParseDuration(v); err == nil {
			c.Storage.MySQL.Pool.ConnMaxLifetime = duration
		}
	}
	if v := os.Getenv("MYSQL_CONN_MAX_IDLE_TIME"); v != "" {
		if duration, err := time.ParseDuration(v); err == nil {
			c.Storage.MySQL.Pool.ConnMaxIdleTime = duration
		}
	}

	// MySQL Replica (optional)
	if v := os.Getenv("MYSQL_REPLICA_ENABLED"); v != "" {
		c.Storage.MySQL.Replica.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("MYSQL_REPLICA_HOST"); v != "" {
		c.Storage.MySQL.Replica.Host = v
	}
	if v := os.Getenv("MYSQL_REPLICA_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Storage.MySQL.Replica.Port = port
		}
	}
	if v := os.Getenv("MYSQL_REPLICA_DATABASE"); v != "" {
		c.Storage.MySQL.Replica.Database = v
	}
	if v := os.Getenv("MYSQL_REPLICA_USERNAME"); v != "" {
		c.Storage.MySQL.Replica.Username = v
	}
	if v := os.Getenv("MYSQL_REPLICA_PASSWORD"); v != "" {
		c.Storage.MySQL.Replica.Password = v
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
		c.Server.WriteTimeout = 30 * time.Second
	}
	if c.Server.RequestTimeout == 0 {
		c.Server.RequestTimeout = 25 * time.Second
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

	// Slack Socket Mode defaults
	if c.Slack.SocketMode.PingInterval == 0 {
		c.Slack.SocketMode.PingInterval = 30 * time.Second
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

	// Storage defaults
	if c.Storage.Type == "" {
		c.Storage.Type = "memory"
	}
	if c.Storage.SQLite.Path == "" {
		c.Storage.SQLite.Path = "./data/alert-bridge.db"
	}

	// MySQL defaults (from research.md)
	if c.Storage.MySQL.Pool.MaxOpenConns == 0 {
		c.Storage.MySQL.Pool.MaxOpenConns = 25
	}
	if c.Storage.MySQL.Pool.MaxIdleConns == 0 {
		c.Storage.MySQL.Pool.MaxIdleConns = 5
	}
	if c.Storage.MySQL.Pool.ConnMaxLifetime == 0 {
		c.Storage.MySQL.Pool.ConnMaxLifetime = 3 * time.Minute
	}
	if c.Storage.MySQL.Pool.ConnMaxIdleTime == 0 {
		c.Storage.MySQL.Pool.ConnMaxIdleTime = 1 * time.Minute
	}
	if c.Storage.MySQL.Timeout == 0 {
		c.Storage.MySQL.Timeout = 5 * time.Second
	}
	if !c.Storage.MySQL.ParseTime {
		c.Storage.MySQL.ParseTime = true
	}
	if c.Storage.MySQL.Charset == "" {
		c.Storage.MySQL.Charset = "utf8mb4"
	}
	if c.Storage.MySQL.Primary.Port == 0 {
		c.Storage.MySQL.Primary.Port = 3306
	}
	if c.Storage.MySQL.Replica.Port == 0 {
		c.Storage.MySQL.Replica.Port = 3306
	}
}

// validate checks that required configuration is present.
// validate is deprecated - use Validate() from validator.go instead
// Kept for backward compatibility but should not be used
func (c *Config) validate() error {
	return c.Validate()
}

// IsSlackEnabled returns true if Slack integration is enabled.
func (c *Config) IsSlackEnabled() bool {
	return c.Slack.Enabled
}

// IsPagerDutyEnabled returns true if PagerDuty integration is enabled.
func (c *Config) IsPagerDutyEnabled() bool {
	return c.PagerDuty.Enabled
}
