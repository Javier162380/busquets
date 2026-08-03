package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plans", "plans"},
		{"My Plans", "my-plans"},
		{"WORK", "work"},
		{"A B C", "a-b-c"},
		{"already-slug", "already-slug"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			require.Equal(t, tc.want, Slugify(tc.input))
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("env overrides apply with no config file present", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("PLAN_VIEWER_DB_BACKEND", "postgres")
		t.Setenv("PLAN_VIEWER_POSTGRES_URL", "postgres://u:p@h:5432/db")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, BackendPostgres, cfg.Database.Backend)
		require.Equal(t, "postgres://u:p@h:5432/db", cfg.Database.Postgres.ConnectionString)
	})

	t.Run("env overrides apply with a config file present", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, ConfigFileName),
			[]byte("[database]\nbackend = \"sqlite\"\n"),
			0o600,
		))
		t.Chdir(dir)
		t.Setenv("PLAN_VIEWER_DB_BACKEND", "postgres")
		t.Setenv("PLAN_VIEWER_POSTGRES_URL", "postgres://u:p@h:5432/db")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, BackendPostgres, cfg.Database.Backend)
	})

	t.Run("defaults are applied with no config file present", func(t *testing.T) {
		t.Chdir(t.TempDir())

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, BackendSQLite, cfg.Database.Backend)
		require.NotEmpty(t, cfg.Database.SQLite.Path)
		require.NotEmpty(t, cfg.Paths.ViewerDir)
		require.Len(t, cfg.Paths.PlansDirs, 1)
		require.Equal(t, 10, cfg.Database.Postgres.MaxOpenConns)
		require.Equal(t, "claude-plan-viewer", cfg.MCP.ServerName)
	})

	t.Run("validation runs with no config file present", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("PLAN_VIEWER_DB_BACKEND", "mysql")

		_, err := LoadConfig()
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid database backend")
	})

	t.Run("postgres backend via env without a URL is rejected", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("PLAN_VIEWER_DB_BACKEND", "postgres")

		_, err := LoadConfig()
		require.Error(t, err)
		require.Contains(t, err.Error(), "postgres connection_string is required")
	})

	t.Run("malformed config file is an error", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, ConfigFileName),
			[]byte("[database\nbackend =\n"),
			0o600,
		))
		t.Chdir(dir)

		_, err := LoadConfig()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse config file")
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string // empty = no error expected
	}{
		{
			name: "valid single dir no label",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths:    PathsConfig{PlansDirs: []SyncDir{{Path: "/some/path"}}},
			},
		},
		{
			name: "valid two dirs distinct labels",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/a/plans", Label: "personal"},
					{Path: "/b/plans", Label: "work"},
				}},
			},
		},
		{
			name: "valid two dirs same base name but explicit distinct labels",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/a/plans", Label: "personal"},
					{Path: "/b/plans", Label: "work"},
				}},
			},
		},
		{
			name: "valid empty plans_dirs",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths:    PathsConfig{PlansDirs: nil},
			},
		},
		{
			name: "invalid db backend",
			cfg: Config{
				Database: DatabaseConfig{Backend: "invalid"},
				Paths:    PathsConfig{PlansDirs: []SyncDir{{Path: "/some/path"}}},
			},
			wantErr: "invalid database backend",
		},
		{
			name: "postgres missing connection string",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendPostgres},
				Paths:    PathsConfig{PlansDirs: []SyncDir{{Path: "/some/path"}}},
			},
			wantErr: "postgres connection_string is required",
		},
		{
			name: "missing path in first entry",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths:    PathsConfig{PlansDirs: []SyncDir{{Path: ""}}},
			},
			wantErr: "plans_dirs[0] is missing a path",
		},
		{
			name: "missing path in second entry",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/valid"},
					{Path: ""},
				}},
			},
			wantErr: "plans_dirs[1] is missing a path",
		},
		{
			name: "duplicate path",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/same/path", Label: "a"},
					{Path: "/same/path", Label: "b"},
				}},
			},
			wantErr: "plans_dirs[0] and plans_dirs[1] share the same path",
		},
		{
			name: "duplicate explicit label",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/a/other", Label: "work"},
					{Path: "/b/other", Label: "work"},
				}},
			},
			wantErr: "resolve to the same label",
		},
		{
			name: "duplicate effective label via base dir name",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/a/plans"}, // derives label "plans"
					{Path: "/b/plans"}, // also derives label "plans"
				}},
			},
			wantErr: "resolve to the same label",
		},
		{
			name: "mixed: one explicit label matches another's derived label",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/a/plans", Label: "plans"},
					{Path: "/b/other"},            // derives label "other" — OK
					{Path: "/c/plans", Label: ""}, // derives label "plans" — collision with index 0
				}},
			},
			wantErr: "resolve to the same label",
		},
		{
			name: "distinct raw labels collide once slugified",
			cfg: Config{
				Database: DatabaseConfig{Backend: BackendSQLite},
				Paths: PathsConfig{PlansDirs: []SyncDir{
					{Path: "/a/other", Label: "Work"},
					{Path: "/b/other", Label: "WORK!"}, // slugifies to "work" too
				}},
			},
			wantErr: "resolve to the same label",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}
