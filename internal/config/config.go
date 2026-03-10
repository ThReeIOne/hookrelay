package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for HookRelay.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Delivery  DeliveryConfig  `yaml:"delivery"`
	API       APIConfig       `yaml:"api"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Redis     RedisConfig     `yaml:"redis"`
	Logging   LoggingConfig   `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         int    `yaml:"port"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	DSN             string `yaml:"dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

// DeliveryConfig holds webhook delivery worker settings.
type DeliveryConfig struct {
	Workers      int    `yaml:"workers"`
	PollInterval string `yaml:"poll_interval"`
	BatchSize    int    `yaml:"batch_size"`
}

// APIConfig holds API authentication settings.
type APIConfig struct {
	Key string `yaml:"key"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled bool `yaml:"enabled"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// MetricsConfig holds Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// setDefaults populates zero-valued fields with sensible defaults.
func (c *Config) setDefaults() {
	// Server defaults
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.ReadTimeout == "" {
		c.Server.ReadTimeout = "30s"
	}
	if c.Server.WriteTimeout == "" {
		c.Server.WriteTimeout = "30s"
	}

	// Database defaults
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 25
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 5
	}
	if c.Database.ConnMaxLifetime == "" {
		c.Database.ConnMaxLifetime = "5m"
	}

	// Delivery defaults
	if c.Delivery.Workers == 0 {
		c.Delivery.Workers = 4
	}
	if c.Delivery.PollInterval == "" {
		c.Delivery.PollInterval = "1s"
	}
	if c.Delivery.BatchSize == 0 {
		c.Delivery.BatchSize = 50
	}

	// API defaults
	if c.API.Key == "" {
		c.API.Key = "changeme"
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	// Metrics defaults
	if c.Metrics.Path == "" {
		c.Metrics.Path = "/metrics"
	}
}

// applyEnvOverrides reads environment variables with the HOOKRELAY_ prefix
// and overrides the corresponding configuration values.
func (c *Config) applyEnvOverrides() {
	// Server overrides
	if v := os.Getenv("HOOKRELAY_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("HOOKRELAY_SERVER_READ_TIMEOUT"); v != "" {
		c.Server.ReadTimeout = v
	}
	if v := os.Getenv("HOOKRELAY_SERVER_WRITE_TIMEOUT"); v != "" {
		c.Server.WriteTimeout = v
	}

	// Database overrides
	if v := os.Getenv("HOOKRELAY_DATABASE_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("HOOKRELAY_DATABASE_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Database.MaxOpenConns = n
		}
	}
	if v := os.Getenv("HOOKRELAY_DATABASE_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Database.MaxIdleConns = n
		}
	}
	if v := os.Getenv("HOOKRELAY_DATABASE_CONN_MAX_LIFETIME"); v != "" {
		c.Database.ConnMaxLifetime = v
	}

	// Delivery overrides
	if v := os.Getenv("HOOKRELAY_DELIVERY_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Delivery.Workers = n
		}
	}
	if v := os.Getenv("HOOKRELAY_DELIVERY_POLL_INTERVAL"); v != "" {
		c.Delivery.PollInterval = v
	}
	if v := os.Getenv("HOOKRELAY_DELIVERY_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Delivery.BatchSize = n
		}
	}

	// API overrides
	if v := os.Getenv("HOOKRELAY_API_KEY"); v != "" {
		c.API.Key = v
	}

	// RateLimit overrides
	if v := os.Getenv("HOOKRELAY_RATELIMIT_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.RateLimit.Enabled = b
		}
	}

	// Redis overrides
	if v := os.Getenv("HOOKRELAY_REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("HOOKRELAY_REDIS_PASSWORD"); v != "" {
		c.Redis.Password = v
	}
	if v := os.Getenv("HOOKRELAY_REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Redis.DB = n
		}
	}

	// Logging overrides
	if v := os.Getenv("HOOKRELAY_LOGGING_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("HOOKRELAY_LOGGING_FORMAT"); v != "" {
		c.Logging.Format = v
	}

	// Metrics overrides
	if v := os.Getenv("HOOKRELAY_METRICS_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Metrics.Enabled = b
		}
	}
	if v := os.Getenv("HOOKRELAY_METRICS_PATH"); v != "" {
		c.Metrics.Path = v
	}
}

// Load reads configuration from a YAML file at the given path, applies
// environment variable overrides (HOOKRELAY_ prefix), fills in defaults
// for any unset fields, and returns the resulting Config.
func Load(path string) (*Config, error) {
	cfg := &Config{
		// Pre-set boolean defaults that would be lost by zero-value semantics.
		RateLimit: RateLimitConfig{Enabled: true},
		Metrics:   MetricsConfig{Enabled: true},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	cfg.applyEnvOverrides()
	cfg.setDefaults()

	return cfg, nil
}
