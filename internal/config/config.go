package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	// ConfigFileName is the name of the configuration file.
	ConfigFileName = "plan-viewer.toml"
)

// Config represents the application configuration.
type Config struct {
	Database DatabaseConfig `toml:"database"`
	Paths    PathsConfig    `toml:"paths"`
	MCP      MCPConfig      `toml:"mcp"`
}

// DatabaseConfig contains database-related configuration.
type DatabaseConfig struct {
	Backend  DatabaseBackend `toml:"backend"`
	SQLite   SQLiteConfig    `toml:"sqlite"`
	Postgres PostgresConfig  `toml:"postgres"`
}

// SQLiteConfig contains SQLite-specific configuration.
type SQLiteConfig struct {
	Path string `toml:"path"`
}

// PostgresConfig contains PostgreSQL-specific configuration.
type PostgresConfig struct {
	ConnectionString string `toml:"connection_string"`
	MaxOpenConns     int    `toml:"max_open_conns"`
	MaxIdleConns     int    `toml:"max_idle_conns"`
}

// PathsConfig contains path-related configuration.
type PathsConfig struct {
	ViewerDir string `toml:"viewer_dir"`
	PlansDir  string `toml:"plans_dir"`
}

// MCPConfig contains MCP server configuration.
type MCPConfig struct {
	ServerName string `toml:"server_name"`
	Version    string `toml:"version"`
}

// LoadConfig loads configuration from the current working directory.
// If no config file exists, returns default configuration.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// Look for config in current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return cfg, nil // Return defaults if can't get CWD
	}

	configPath := filepath.Join(cwd, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil // Config doesn't exist, use defaults
	}

	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Apply defaults for empty values
	cfg.applyDefaults()

	return cfg, nil
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()

	return &Config{
		Database: DatabaseConfig{
			Backend: BackendSQLite,
			SQLite: SQLiteConfig{
				Path: filepath.Join(homeDir, ".claude-viewer", "plans.db"),
			},
			Postgres: PostgresConfig{
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
		},
		Paths: PathsConfig{
			ViewerDir: filepath.Join(homeDir, ".claude-viewer"),
			PlansDir:  filepath.Join(homeDir, ".claude", "plans"),
		},
		MCP: MCPConfig{
			ServerName: "claude-plan-viewer",
			Version:    "1.0.0",
		},
	}
}

// applyEnvOverrides applies environment variable overrides to the config.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("PLAN_VIEWER_DB_BACKEND"); v != "" {
		c.Database.Backend = DatabaseBackend(v)
	}
	if v := os.Getenv("PLAN_VIEWER_SQLITE_PATH"); v != "" {
		c.Database.SQLite.Path = v
	}
	if v := os.Getenv("PLAN_VIEWER_POSTGRES_URL"); v != "" {
		c.Database.Postgres.ConnectionString = v
	}
	if v := os.Getenv("PLAN_VIEWER_DIR"); v != "" {
		c.Paths.ViewerDir = v
	}
	if v := os.Getenv("PLAN_VIEWER_PLANS_DIR"); v != "" {
		c.Paths.PlansDir = v
	}
	if v := os.Getenv("PLAN_VIEWER_MCP_SERVER_NAME"); v != "" {
		c.MCP.ServerName = v
	}
	if v := os.Getenv("PLAN_VIEWER_MCP_VERSION"); v != "" {
		c.MCP.Version = v
	}
}

// applyDefaults fills in default values for empty configuration fields.
func (c *Config) applyDefaults() {
	defaults := DefaultConfig()

	if c.Database.SQLite.Path == "" {
		c.Database.SQLite.Path = defaults.Database.SQLite.Path
	}
	if c.Database.Postgres.MaxOpenConns == 0 {
		c.Database.Postgres.MaxOpenConns = defaults.Database.Postgres.MaxOpenConns
	}
	if c.Database.Postgres.MaxIdleConns == 0 {
		c.Database.Postgres.MaxIdleConns = defaults.Database.Postgres.MaxIdleConns
	}
	if c.Paths.ViewerDir == "" {
		c.Paths.ViewerDir = defaults.Paths.ViewerDir
	}
	if c.Paths.PlansDir == "" {
		c.Paths.PlansDir = defaults.Paths.PlansDir
	}
	if c.MCP.ServerName == "" {
		c.MCP.ServerName = defaults.MCP.ServerName
	}
	if c.MCP.Version == "" {
		c.MCP.Version = defaults.MCP.Version
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if !c.Database.Backend.IsValid() {
		return fmt.Errorf("invalid database backend: %q (must be %q or %q)",
			c.Database.Backend, BackendSQLite, BackendPostgres)
	}

	if c.Database.Backend == BackendPostgres && c.Database.Postgres.ConnectionString == "" {
		return fmt.Errorf("postgres connection_string is required when backend is %q", BackendPostgres)
	}

	return nil
}
