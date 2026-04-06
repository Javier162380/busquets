package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" //nolint:revive,stylecheck // pgx stdlib driver for migrations
)

// PostgresConfig contains PostgreSQL connection configuration.
type PostgresConfig struct {
	ConnectionString string
	MaxOpenConns     int
	MaxIdleConns     int
}

// PostgresPool wraps a pgxpool.Pool for PostgreSQL connections.
type PostgresPool struct {
	pool *pgxpool.Pool
}

// NewPostgresClient creates a new PostgreSQL connection pool.
func NewPostgresClient(ctx context.Context, cfg PostgresConfig) (*PostgresPool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres connection string: %w", err)
	}

	// Configure pool settings
	if cfg.MaxOpenConns > 0 {
		poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		poolConfig.MinConns = int32(cfg.MaxIdleConns)
	}
	poolConfig.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &PostgresPool{pool: pool}, nil
}

// Pool returns the underlying pgxpool.Pool for use with SQLC.
func (p *PostgresPool) Pool() *pgxpool.Pool {
	return p.pool
}

// Close closes the connection pool.
func (p *PostgresPool) Close() {
	p.pool.Close()
}

// StdlibDB returns a database/sql DB using pgx stdlib driver.
// This is needed for running migrations with goose.
func (p *PostgresPool) StdlibDB() (*sql.DB, error) {
	// Get the connection string from pool config
	connString := p.pool.Config().ConnString()

	// Open using pgx stdlib driver
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open pgx stdlib connection: %w", err)
	}

	return db, nil
}

// RunPostgresMigrations runs migrations on a Postgres database.
func RunPostgresMigrations(_ context.Context, cfg PostgresConfig) error {
	// Use stdlib for migrations (goose requires database/sql)
	db, err := sql.Open("pgx", cfg.ConnectionString)
	if err != nil {
		return fmt.Errorf("failed to open postgres for migrations: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	return RunMigrations(db, config.BackendPostgres)
}
