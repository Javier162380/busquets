package config

import (
	"os"
	"path/filepath"
	"sync"
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

// newDefaultCfgForHome builds a Config via DefaultConfig() while HOME is
// pointed at tempDir, so cfg.Paths.ViewerDir resolves under the test's own
// throwaway directory instead of the real machine's home — MigrateLegacyViewerDir
// gates on cfg.Paths.ViewerDir matching DefaultConfig()'s value, so tests must
// go through DefaultConfig() itself rather than hand-building a Config to stay
// on that gate's happy path.
func newDefaultCfgForHome(t *testing.T, tempDir string) *Config {
	t.Helper()
	t.Setenv("HOME", tempDir)
	return DefaultConfig()
}

func TestMigrateLegacyViewerDir(t *testing.T) {
	t.Run("no-op when a custom viewer dir is configured", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := newDefaultCfgForHome(t, tempDir)
		cfg.Paths.ViewerDir = filepath.Join(tempDir, "custom-viewer-dir")

		legacyDir := filepath.Join(tempDir, LegacyViewerDirName)
		require.NoError(t, os.MkdirAll(legacyDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "marker.txt"), []byte("legacy"), 0o600))

		require.NoError(t, MigrateLegacyViewerDir(cfg))

		// A custom viewer_dir means this migration doesn't apply — the
		// legacy dir must be left exactly as it was.
		require.DirExists(t, legacyDir)
		require.NoDirExists(t, cfg.Paths.ViewerDir)
	})

	t.Run("no-op when neither the legacy nor the new dir exists", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := newDefaultCfgForHome(t, tempDir)

		require.NoError(t, MigrateLegacyViewerDir(cfg))

		require.NoDirExists(t, filepath.Join(tempDir, LegacyViewerDirName))
		require.NoDirExists(t, cfg.Paths.ViewerDir)
	})

	t.Run("renames the legacy dir to the new default location, preserving its contents", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := newDefaultCfgForHome(t, tempDir)

		legacyDir := filepath.Join(tempDir, LegacyViewerDirName)
		require.NoError(t, os.MkdirAll(filepath.Join(legacyDir, "plans", "1"), 0o750))
		require.NoError(t, os.WriteFile(
			filepath.Join(legacyDir, "plans", "1", "example.md"), []byte("# Example"), 0o600))

		require.NoError(t, MigrateLegacyViewerDir(cfg))

		require.NoDirExists(t, legacyDir)
		require.DirExists(t, cfg.Paths.ViewerDir)
		data, err := os.ReadFile(filepath.Join(cfg.Paths.ViewerDir, "plans", "1", "example.md"))
		require.NoError(t, err)
		require.Equal(t, "# Example", string(data))
	})

	t.Run("is idempotent: a second call is a no-op once already migrated", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := newDefaultCfgForHome(t, tempDir)

		legacyDir := filepath.Join(tempDir, LegacyViewerDirName)
		require.NoError(t, os.MkdirAll(legacyDir, 0o750))

		require.NoError(t, MigrateLegacyViewerDir(cfg))
		require.NoError(t, MigrateLegacyViewerDir(cfg)) // must not error just because legacyDir is already gone

		require.DirExists(t, cfg.Paths.ViewerDir)
	})

	t.Run("warns and leaves both directories in place rather than clobbering when both exist", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := newDefaultCfgForHome(t, tempDir)

		legacyDir := filepath.Join(tempDir, LegacyViewerDirName)
		require.NoError(t, os.MkdirAll(legacyDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "marker.txt"), []byte("legacy"), 0o600))
		require.NoError(t, os.MkdirAll(cfg.Paths.ViewerDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(cfg.Paths.ViewerDir, "marker.txt"), []byte("current"), 0o600))

		require.NoError(t, MigrateLegacyViewerDir(cfg))

		legacyMarker, err := os.ReadFile(filepath.Join(legacyDir, "marker.txt"))
		require.NoError(t, err)
		require.Equal(t, "legacy", string(legacyMarker))

		currentMarker, err := os.ReadFile(filepath.Join(cfg.Paths.ViewerDir, "marker.txt"))
		require.NoError(t, err)
		require.Equal(t, "current", string(currentMarker))
	})

	// Regression test for a TOCTOU race: two processes (e.g. a TUI session
	// and an MCP server) can both launch around the same time on first run
	// after upgrading and both call this concurrently. The loser's
	// os.Rename used to fail with ENOENT once the winner had already moved
	// oldDir, and that error was propagated as a hard failure. Neither
	// caller should ever error — the loser must recognize "oldDir is
	// already gone" as success, since that's exactly the outcome it wanted.
	t.Run("concurrent callers never error, even when they race the same rename", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := newDefaultCfgForHome(t, tempDir)

		legacyDir := filepath.Join(tempDir, LegacyViewerDirName)
		require.NoError(t, os.MkdirAll(legacyDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "marker.txt"), []byte("legacy"), 0o600))

		const callers = 8
		start := make(chan struct{})
		errs := make([]error, callers)
		var wg sync.WaitGroup
		wg.Add(callers)
		for i := range callers {
			go func(i int) {
				defer wg.Done()
				<-start // maximize the chance every goroutine reaches os.Rename around the same time
				errs[i] = MigrateLegacyViewerDir(cfg)
			}(i)
		}
		close(start)
		wg.Wait()

		for i, err := range errs {
			require.NoErrorf(t, err, "caller %d", i)
		}

		require.NoDirExists(t, legacyDir)
		require.DirExists(t, cfg.Paths.ViewerDir)
		data, err := os.ReadFile(filepath.Join(cfg.Paths.ViewerDir, "marker.txt"))
		require.NoError(t, err)
		require.Equal(t, "legacy", string(data))
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("returns defaults when neither config file exists", func(t *testing.T) {
		t.Chdir(t.TempDir())

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, DefaultConfig().Database.Backend, cfg.Database.Backend)
	})

	t.Run("reads busquets.toml when present", func(t *testing.T) {
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(ConfigFileName, []byte(`
[database]
backend = "postgres"
[database.postgres]
connection_string = "postgres://example"
`), 0o600))

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, BackendPostgres, cfg.Database.Backend)
	})

	t.Run("falls back to the legacy plan-viewer.toml name when busquets.toml is absent", func(t *testing.T) {
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(LegacyConfigFileName, []byte(`
[database]
backend = "postgres"
[database.postgres]
connection_string = "postgres://example"
`), 0o600))

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, BackendPostgres, cfg.Database.Backend)
	})

	t.Run("prefers busquets.toml over the legacy name when both exist", func(t *testing.T) {
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(ConfigFileName, []byte(`
[database]
backend = "sqlite"
`), 0o600))
		require.NoError(t, os.WriteFile(LegacyConfigFileName, []byte(`
[database]
backend = "postgres"
[database.postgres]
connection_string = "postgres://example"
`), 0o600))

		cfg, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, BackendSQLite, cfg.Database.Backend)
	})
}
