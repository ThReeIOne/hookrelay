package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Load – valid YAML file
// ---------------------------------------------------------------------------

func TestLoad_ValidFullConfig(t *testing.T) {
	yamlContent := `
server:
  port: 9090
  read_timeout: "15s"
  write_timeout: "20s"

database:
  dsn: "postgres://user:pass@localhost:5432/hookrelay"
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: "10m"

delivery:
  workers: 8
  poll_interval: "2s"
  batch_size: 100

api:
  key: "my-secret-key"

rate_limit:
  enabled: false

redis:
  addr: "localhost:6379"
  password: "redis-pass"
  db: 2

logging:
  level: "debug"
  format: "text"

metrics:
  enabled: false
  path: "/custom-metrics"
`
	tmpFile := writeTempYAML(t, yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", tmpFile, err)
	}

	// Server
	assertEqual(t, "Server.Port", cfg.Server.Port, 9090)
	assertEqual(t, "Server.ReadTimeout", cfg.Server.ReadTimeout, "15s")
	assertEqual(t, "Server.WriteTimeout", cfg.Server.WriteTimeout, "20s")

	// Database
	assertEqual(t, "Database.DSN", cfg.Database.DSN, "postgres://user:pass@localhost:5432/hookrelay")
	assertEqual(t, "Database.MaxOpenConns", cfg.Database.MaxOpenConns, 50)
	assertEqual(t, "Database.MaxIdleConns", cfg.Database.MaxIdleConns, 10)
	assertEqual(t, "Database.ConnMaxLifetime", cfg.Database.ConnMaxLifetime, "10m")

	// Delivery
	assertEqual(t, "Delivery.Workers", cfg.Delivery.Workers, 8)
	assertEqual(t, "Delivery.PollInterval", cfg.Delivery.PollInterval, "2s")
	assertEqual(t, "Delivery.BatchSize", cfg.Delivery.BatchSize, 100)

	// API
	assertEqual(t, "API.Key", cfg.API.Key, "my-secret-key")

	// RateLimit
	assertEqual(t, "RateLimit.Enabled", cfg.RateLimit.Enabled, false)

	// Redis
	assertEqual(t, "Redis.Addr", cfg.Redis.Addr, "localhost:6379")
	assertEqual(t, "Redis.Password", cfg.Redis.Password, "redis-pass")
	assertEqual(t, "Redis.DB", cfg.Redis.DB, 2)

	// Logging
	assertEqual(t, "Logging.Level", cfg.Logging.Level, "debug")
	assertEqual(t, "Logging.Format", cfg.Logging.Format, "text")

	// Metrics
	assertEqual(t, "Metrics.Enabled", cfg.Metrics.Enabled, false)
	assertEqual(t, "Metrics.Path", cfg.Metrics.Path, "/custom-metrics")
}

// ---------------------------------------------------------------------------
// Load – missing file
// ---------------------------------------------------------------------------

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/tmp/nonexistent-hookrelay-config-file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// ---------------------------------------------------------------------------
// Load – defaults are filled for omitted fields
// ---------------------------------------------------------------------------

func TestLoad_DefaultValues(t *testing.T) {
	// Minimal YAML with only DSN set; everything else should get defaults.
	yamlContent := `
database:
  dsn: "postgres://localhost/test"
`
	tmpFile := writeTempYAML(t, yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Server defaults
	assertEqual(t, "Server.Port", cfg.Server.Port, 8080)
	assertEqual(t, "Server.ReadTimeout", cfg.Server.ReadTimeout, "30s")
	assertEqual(t, "Server.WriteTimeout", cfg.Server.WriteTimeout, "30s")

	// Database defaults (DSN was provided, others default)
	assertEqual(t, "Database.DSN", cfg.Database.DSN, "postgres://localhost/test")
	assertEqual(t, "Database.MaxOpenConns", cfg.Database.MaxOpenConns, 25)
	assertEqual(t, "Database.MaxIdleConns", cfg.Database.MaxIdleConns, 5)
	assertEqual(t, "Database.ConnMaxLifetime", cfg.Database.ConnMaxLifetime, "5m")

	// Delivery defaults
	assertEqual(t, "Delivery.Workers", cfg.Delivery.Workers, 4)
	assertEqual(t, "Delivery.PollInterval", cfg.Delivery.PollInterval, "1s")
	assertEqual(t, "Delivery.BatchSize", cfg.Delivery.BatchSize, 50)

	// API default
	assertEqual(t, "API.Key", cfg.API.Key, "changeme")

	// RateLimit default (pre-set to true in Load before unmarshalling,
	// but the YAML does not override it, so it stays true)
	assertEqual(t, "RateLimit.Enabled", cfg.RateLimit.Enabled, true)

	// Logging defaults
	assertEqual(t, "Logging.Level", cfg.Logging.Level, "info")
	assertEqual(t, "Logging.Format", cfg.Logging.Format, "json")

	// Metrics defaults (pre-set to true in Load)
	assertEqual(t, "Metrics.Enabled", cfg.Metrics.Enabled, true)
	assertEqual(t, "Metrics.Path", cfg.Metrics.Path, "/metrics")
}

// ---------------------------------------------------------------------------
// Load – completely empty YAML file still produces valid defaults
// ---------------------------------------------------------------------------

func TestLoad_EmptyYAML(t *testing.T) {
	tmpFile := writeTempYAML(t, "")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	assertEqual(t, "Server.Port", cfg.Server.Port, 8080)
	assertEqual(t, "API.Key", cfg.API.Key, "changeme")
	assertEqual(t, "Delivery.Workers", cfg.Delivery.Workers, 4)
}

// ---------------------------------------------------------------------------
// Load – environment variable overrides
// ---------------------------------------------------------------------------

func TestLoad_EnvOverride_ServerPort(t *testing.T) {
	yamlContent := `
server:
  port: 9090
`
	tmpFile := writeTempYAML(t, yamlContent)

	t.Setenv("HOOKRELAY_SERVER_PORT", "3000")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	assertEqual(t, "Server.Port", cfg.Server.Port, 3000)
}

func TestLoad_EnvOverride_DatabaseDSN(t *testing.T) {
	yamlContent := `
database:
  dsn: "postgres://yaml-host/db"
`
	tmpFile := writeTempYAML(t, yamlContent)

	t.Setenv("HOOKRELAY_DATABASE_DSN", "postgres://env-host/envdb")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	assertEqual(t, "Database.DSN", cfg.Database.DSN, "postgres://env-host/envdb")
}

func TestLoad_EnvOverride_APIKey(t *testing.T) {
	yamlContent := `
api:
  key: "yaml-key"
`
	tmpFile := writeTempYAML(t, yamlContent)

	t.Setenv("HOOKRELAY_API_KEY", "env-secret-key")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	assertEqual(t, "API.Key", cfg.API.Key, "env-secret-key")
}

func TestLoad_EnvOverride_MultipleVars(t *testing.T) {
	yamlContent := `
server:
  port: 8080
database:
  dsn: "postgres://yaml/db"
api:
  key: "yaml-key"
`
	tmpFile := writeTempYAML(t, yamlContent)

	t.Setenv("HOOKRELAY_SERVER_PORT", "4000")
	t.Setenv("HOOKRELAY_DATABASE_DSN", "postgres://env/db")
	t.Setenv("HOOKRELAY_API_KEY", "env-key")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	assertEqual(t, "Server.Port", cfg.Server.Port, 4000)
	assertEqual(t, "Database.DSN", cfg.Database.DSN, "postgres://env/db")
	assertEqual(t, "API.Key", cfg.API.Key, "env-key")
}

func TestLoad_EnvOverride_InvalidPortIgnored(t *testing.T) {
	yamlContent := `
server:
  port: 9090
`
	tmpFile := writeTempYAML(t, yamlContent)

	t.Setenv("HOOKRELAY_SERVER_PORT", "not-a-number")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Invalid env value is ignored; YAML value remains
	assertEqual(t, "Server.Port", cfg.Server.Port, 9090)
}

func TestLoad_EnvOverride_AppliedBeforeDefaults(t *testing.T) {
	// Use an empty YAML so all fields are zero-valued.
	// Set an env var for port. The env override should apply, then
	// setDefaults should NOT overwrite it because it is non-zero.
	tmpFile := writeTempYAML(t, "")

	t.Setenv("HOOKRELAY_SERVER_PORT", "5555")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	assertEqual(t, "Server.Port", cfg.Server.Port, 5555)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeTempYAML creates a temporary YAML file and registers cleanup.
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}
	return path
}

// assertEqual is a generic test helper for comparing values.
func assertEqual[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}
