package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"github.com/Javier162380/claude-plan-viewer/internal/config"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

// RunMigrations runs database migrations for the specified dialect.
// Goose automatically creates and manages a goose_db_version table
// to track which migrations have been applied.
func RunMigrations(db *sql.DB, backend config.DatabaseBackend) error {
	var migrations embed.FS
	var migrationsDir string
	var gooseDialect string

	switch backend {
	case config.BackendSQLite:
		migrations = sqliteMigrations
		migrationsDir = "migrations/sqlite"
		gooseDialect = "sqlite3" // Goose uses "sqlite3", not "sqlite"
	case config.BackendPostgres:
		migrations = postgresMigrations
		migrationsDir = "migrations/postgres"
		gooseDialect = "postgres"
	default:
		return fmt.Errorf("unsupported dialect: %s", backend)
	}

	goose.SetBaseFS(migrations)

	if err := goose.SetDialect(gooseDialect); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
