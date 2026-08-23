package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// ConfigFileName is the name of the configuration file.
	ConfigFileName = "busquets.toml"

	// LegacyConfigFileName is the pre-rebrand config file name. LoadConfig
	// falls back to it (with a deprecation warning) when ConfigFileName
	// isn't found, so a config file created before the rebrand keeps
	// working rather than silently reverting to defaults.
	LegacyConfigFileName = "plan-viewer.toml"

	// LegacyViewerDirName is the pre-rebrand default viewer directory name.
	// Exported (not just internal to config) because services/busquets also
	// needs this exact literal to reconcile stale DB file_path prefixes —
	// see Service.MigrateLegacyFilePathPrefix. Kept as a standalone
	// constant, not derived from any "previous defaults" concept, since it
	// only ever needs to describe this one historical value.
	LegacyViewerDirName = ".claude-viewer"
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
		legacyPath := filepath.Join(cwd, LegacyConfigFileName)
		if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
			return cfg, nil // Neither config file exists, use defaults
		}
		fmt.Fprintf(os.Stderr, "warning: %s is deprecated; rename it to %s\n", LegacyConfigFileName, ConfigFileName)
		configPath = legacyPath
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
				Path: filepath.Join(homeDir, ".busquets", "plans.db"),
			},
			Postgres: PostgresConfig{
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
		},
		Paths: PathsConfig{
			ViewerDir: filepath.Join(homeDir, ".busquets"),
			// ~/.claude/plans is Claude Code's own on-disk convention — one
			// built-in default source among potentially several; add more
			// via [[paths.plans_dirs]] in busquets.toml for any other
			// LLM/AI assistant that writes plan files to disk.
			PlansDirs: []SyncDir{{
				Path:  filepath.Join(homeDir, ".claude", "plans"),
				Label: "plans",
			}},
		},
		MCP: MCPConfig{
			ServerName: "busquets",
			Version:    "1.0.0",
		},
	}
}

// MigrateLegacyViewerDir renames the legacy ~/.claude-viewer directory to the
// new default ~/.busquets location. The move is a single os.Rename on the
// directory, which is atomic on the same filesystem (AGENTS.md rule 12) —
// the SQLite DB file, every mirrored plan file, version file, and log file
// move together in one filesystem operation. There is no window where some
// have moved and others haven't, and a failure leaves the legacy directory
// completely untouched — no transaction machinery needed, the syscall's own
// atomicity is the guarantee.
//
// No-op if cfg isn't using the default viewer dir (a custom
// PLAN_VIEWER_DIR/viewer_dir override means this migration doesn't apply),
// if the legacy dir doesn't exist, or if the new dir already exists (logs a
// warning and leaves both in place rather than clobbering). Idempotent: safe
// to call on every startup.
//
// Safe under concurrent callers (e.g. a TUI session and an MCP server both
// launching around the same time on first run after upgrading): the
// existence checks below are advisory only, not a lock — the real race is
// resolved at the os.Rename call itself, whose ENOENT/ErrNotExist result
// (oldDir already gone — a concurrent caller won the race) is treated as
// success, not failure. Whoever loses the race for the rename just
// discovers oldDir is already gone and moves on; it never hard-exits the
// process over a directory that's already exactly where it should be.
func MigrateLegacyViewerDir(cfg *Config) error {
	defaults := DefaultConfig()
	if cfg.Paths.ViewerDir != defaults.Paths.ViewerDir {
		return nil // custom viewer_dir — not our concern
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil // can't resolve $HOME; nothing safe to migrate
	}
	oldDir := filepath.Join(homeDir, LegacyViewerDirName)
	newDir := cfg.Paths.ViewerDir // == defaults.Paths.ViewerDir

	oldInfo, err := os.Stat(oldDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil // nothing to migrate
	}
	if err != nil {
		return fmt.Errorf("failed to stat legacy viewer dir %s: %w", oldDir, err)
	}
	if !oldInfo.IsDir() {
		return nil // unexpected non-directory at the legacy path; leave it alone
	}

	if _, err := os.Stat(newDir); err == nil {
		fmt.Fprintf(os.Stderr,
			"warning: both %s and %s exist; using %s — remove the stale %s manually once you've confirmed no data is missing\n",
			oldDir, newDir, newDir, oldDir)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %w", newDir, err)
	}

	if err := os.Rename(oldDir, newDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// oldDir vanished between our stat above and this rename — a
			// concurrent process already migrated it. That's the outcome we
			// wanted, just achieved by someone else; nothing left to do.
			return nil
		}
		return fmt.Errorf(
			"failed to migrate %s to %s: %w (you can migrate manually with: mv %q %q)",
			oldDir, newDir, err, oldDir, newDir,
		)
	}
	fmt.Fprintf(os.Stderr, "migrated %s to %s\n", oldDir, newDir)
	return nil
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
		fmt.Fprintf(os.Stderr, "warning: PLAN_VIEWER_PLANS_DIR is deprecated; use [[paths.plans_dirs]] in busquets.toml instead\n")
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
