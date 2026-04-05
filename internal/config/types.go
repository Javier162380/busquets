// Package config provides configuration loading and types for the application.
package config

// DatabaseBackend represents the type of database backend to use.
type DatabaseBackend string

const (
	// BackendSQLite uses SQLite as the database backend.
	BackendSQLite DatabaseBackend = "sqlite"
	// BackendPostgres uses PostgreSQL as the database backend.
	BackendPostgres DatabaseBackend = "postgres"
)

// IsValid returns true if the backend is a supported value.
func (b DatabaseBackend) IsValid() bool {
	return b == BackendSQLite || b == BackendPostgres
}

// String returns the string representation of the backend.
func (b DatabaseBackend) String() string {
	return string(b)
}
