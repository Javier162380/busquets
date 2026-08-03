package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	ViewerDir string    `toml:"viewer_dir"`
	PlansDirs []SyncDir `toml:"plans_dirs"`
}

// MCPConfig contains MCP server configuration.
type MCPConfig struct {
	ServerName string `toml:"server_name"`
	Version    string `toml:"version"`
}

// LoadConfig loads configuration from the current working directory.
// A missing config file is not an error — defaults plus environment overrides
// are a complete configuration, so overrides, validation, and defaults run on
// every path. A config file only supplies an extra layer of values.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	configPath, err := configFilePath()
	if err != nil {
		return nil, err
	}
	if configPath != "" {
		if _, err := toml.DecodeFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}
	}

	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	cfg.applyDefaults()

	return cfg, nil
}

// configFilePath returns the config file path in the current working directory,
// or "" if there is none. An undeterminable CWD counts as "no config file".
func configFilePath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil
	}

	path := filepath.Join(cwd, ConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to stat config file %s: %w", path, err)
	}
	return path, nil
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
			PlansDirs: []SyncDir{{
				Path:  filepath.Join(homeDir, ".claude", "plans"),
				Label: "plans",
			}},
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
		// PLAN_VIEWER_PLANS_DIR was replaced by [[paths.plans_dirs]] in the TOML config.
		// Fall back gracefully: treat the value as a single unlabelled sync directory.
		fmt.Fprintf(os.Stderr, "warning: PLAN_VIEWER_PLANS_DIR is deprecated; use [[paths.plans_dirs]] in plan-viewer.toml instead\n")
		if len(c.Paths.PlansDirs) == 0 {
			c.Paths.PlansDirs = []SyncDir{{Path: v}}
		}
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
	if len(c.Paths.PlansDirs) == 0 {
		c.Paths.PlansDirs = defaults.Paths.PlansDirs
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

	seenPaths := make(map[string]int, len(c.Paths.PlansDirs))
	seenLabels := make(map[string]int, len(c.Paths.PlansDirs))
	for i, d := range c.Paths.PlansDirs {
		if d.Path == "" {
			return fmt.Errorf("plans_dirs[%d] is missing a path", i)
		}
		if strings.ContainsAny(d.Label, "/\\") {
			return fmt.Errorf("plans_dirs[%d] label %q must not contain path separators", i, d.Label)
		}
		if prev, dup := seenPaths[d.Path]; dup {
			return fmt.Errorf("plans_dirs[%d] and plans_dirs[%d] share the same path %q", prev, i, d.Path)
		}
		seenPaths[d.Path] = i

		effectiveLabel := d.Label
		if effectiveLabel == "" {
			effectiveLabel = filepath.Base(d.Path)
		}
		slug := Slugify(effectiveLabel)
		if prev, dup := seenLabels[slug]; dup {
			return fmt.Errorf(
				"plans_dirs[%d] and plans_dirs[%d] resolve to the same label %q — set distinct labels to avoid viewer directory collisions",
				prev, i, slug,
			)
		}
		seenLabels[slug] = i
	}

	return nil
}

// Slugify converts a label to a safe directory name: lowercased, with only
// [a-z0-9-_] kept, spaces turned into hyphens, and everything else stripped.
// Used both to name a sync source's viewer mirror directory and, in
// Validate, to detect labels that would collide once slugified even though
// their raw values differ (e.g. "Work" and "WORK!").
func Slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}
