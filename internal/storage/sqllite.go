// Package storage provides database clients for SQLite and PostgreSQL.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Javier162380/busquets/internal/config"

	_ "github.com/mattn/go-sqlite3" //nolint:revive,stylecheck // SQLite driver needed.
)

// DB is a wrapper around sql.DB to provide a context-aware interface.
type DB struct {
	db *sql.DB
}

// NewSQLiteClientWithMigrations creates a new DB instance and runs migrations.
// A nil logger discards all goose migration output.
func NewSQLiteClientWithMigrations(_ context.Context, dataSourceName string, logger *slog.Logger) (*DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL", dataSourceName))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Limit to single connection to avoid SQLite locking issues
	db.SetMaxOpenConns(1)

	if err := RunMigrations(db, config.BackendSQLite, logger); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database connection.
func (c *DB) Close() error {
	return c.db.Close()
}

// DB returns the underlying sql.DB for use with goose.
func (c *DB) DB() *sql.DB {
	return c.db
}

// ExecContext executes a query with the given context and arguments.
func (c *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}

// PrepareContext prepares a statement for execution with the given context.
func (c *DB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return c.db.PrepareContext(ctx, query)
}

// QueryContext executes a query with the given context and arguments, returning the rows.
func (c *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}

// QueryRowContext executes a query with the given context and arguments, returning a single row.
func (c *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}
