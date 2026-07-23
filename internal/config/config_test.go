package config

import (
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
