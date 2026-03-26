// Package storage provides a context-aware SQLite database client.
package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3" //nolint:revive,stylecheck // SQLite driver needed.
)

// DB is a wrapper around sql.DB to provide a context-aware interface.
type DB struct {
	db *sql.DB
}

// NewSQLiteClient creates a new DB instance with the given data source name.
func NewSQLiteClient(ctx context.Context, dataSourceName string, withSchema *string) (*DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", dataSourceName))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if withSchema != nil {
		if _, err = db.ExecContext(ctx, *withSchema); err != nil {
			return nil, fmt.Errorf("failed to execute schema: %w", err)
		}
	}

	return &DB{db: db}, nil
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
