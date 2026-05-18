package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/internal/config"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

// gooseLogger adapts *slog.Logger to the goose.Logger interface.
type gooseLogger struct {
	l *slog.Logger
}

func (g gooseLogger) Fatalf(format string, v ...interface{}) {
	g.l.Error(strings.TrimRight(fmt.Sprintf(format, v...), "\n"))
	os.Exit(1)
}

func (g gooseLogger) Printf(format string, v ...interface{}) {
	g.l.Info(strings.TrimRight(fmt.Sprintf(format, v...), "\n"))
}

// RunMigrations runs database migrations for the specified dialect.
// Goose automatically creates and manages a goose_db_version table
// to track which migrations have been applied.
// A nil logger defaults to discarding all goose output.
func RunMigrations(db *sql.DB, backend config.DatabaseBackend, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	goose.SetLogger(gooseLogger{logger})

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
