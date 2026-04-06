package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	httpserver "github.com/Javier162380/claude-plan-viewer/cmd/http"
	tuiapp "github.com/Javier162380/claude-plan-viewer/cmd/tui"
	"github.com/Javier162380/claude-plan-viewer/internal/config"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors/telegram"
	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/postgres"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch command {
	case "sync":
		if err := runSync(cfg); err != nil {
			log.Fatalf("Sync failed: %v", err)
		}
	case "serve":
		if err := runServe(cfg); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	case "tui":
		if err := runTUI(cfg); err != nil {
			log.Fatalf("TUI failed: %v", err)
		}
	case "migrate":
		if err := runMigrate(cfg); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// initRepository creates a repository based on the configuration.
// Returns the repository and a cleanup function.
func initRepository(ctx context.Context, cfg *config.Config) (dto.Repository, func(), error) {
	// Ensure viewer directory exists
	//nolint:gosec // G301: Standard directory permissions for user application directory
	if err := os.MkdirAll(cfg.Paths.ViewerDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create viewer directory: %w", err)
	}

	switch cfg.Database.Backend {
	case config.BackendSQLite:
		db, err := storage.NewSQLiteClientWithMigrations(ctx, cfg.Database.SQLite.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize SQLite: %w", err)
		}
		repo := sqlite.NewRepository(db)
		cleanup := func() { _ = db.Close() }
		return repo, cleanup, nil

	case config.BackendPostgres:
		// Run migrations first
		if err := storage.RunPostgresMigrations(ctx, storage.PostgresConfig{
			ConnectionString: cfg.Database.Postgres.ConnectionString,
		}); err != nil {
			return nil, nil, fmt.Errorf("failed to run postgres migrations: %w", err)
		}

		// Create pool for queries
		pool, err := storage.NewPostgresClient(ctx, storage.PostgresConfig{
			ConnectionString: cfg.Database.Postgres.ConnectionString,
			MaxOpenConns:     cfg.Database.Postgres.MaxOpenConns,
			MaxIdleConns:     cfg.Database.Postgres.MaxIdleConns,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize Postgres: %w", err)
		}

		repo := postgres.NewRepository(pool.Pool())
		cleanup := func() { pool.Close() }
		return repo, cleanup, nil

	default:
		return nil, nil, fmt.Errorf("unsupported database backend: %s", cfg.Database.Backend)
	}
}

func runSync(cfg *config.Config) error {
	ctx := context.Background()

	repo, cleanup, err := initRepository(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := claudeviewer.New(repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDir, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	count, err := service.SyncPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync plans: %w", err)
	}

	fmt.Printf("✓ Synced %d plans from %s to %s (backend: %s)\n",
		count, cfg.Paths.PlansDir, cfg.Paths.ViewerDir, cfg.Database.Backend)
	return nil
}

func runServe(cfg *config.Config) error {
	ctx := context.Background()

	// Parse flags
	addr := flag.String("addr", ":8081", "HTTP server address")
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	repo, cleanup, err := initRepository(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := claudeviewer.New(repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDir, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	server, err := httpserver.NewServer(service, *addr)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	fmt.Printf("Server running on http://localhost%s (backend: %s)\n", *addr, cfg.Database.Backend)
	return server.Server().Start(*addr)
}

func runTUI(cfg *config.Config) error {
	ctx := context.Background()

	debug := os.Getenv("DEBUG") == "1"

	repo, cleanup, err := initRepository(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := claudeviewer.New(repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDir, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Initialize connectors
	registry := connectors.NewRegistry()
	_ = registry.Register(telegram.New())

	connectorManager := connectors.NewManager(registry, repo)
	service.SetConnectorManager(connectorManager)

	return tuiapp.StartWithOptions(ctx, service, debug)
}

func runMigrate(cfg *config.Config) error {
	ctx := context.Background()

	fmt.Printf("Running migrations for %s backend...\n", cfg.Database.Backend)

	switch cfg.Database.Backend {
	case config.BackendSQLite:
		// Ensure directory exists
		dir := filepath.Dir(cfg.Database.SQLite.Path)
		//nolint:gosec // G301: Standard directory permissions
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		db, err := storage.NewSQLiteClientWithMigrations(ctx, cfg.Database.SQLite.Path)
		if err != nil {
			return fmt.Errorf("failed to run SQLite migrations: %w", err)
		}
		defer func() {
			_ = db.Close()
		}()
		fmt.Printf("✓ SQLite migrations completed: %s\n", cfg.Database.SQLite.Path)

	case config.BackendPostgres:
		if err := storage.RunPostgresMigrations(ctx, storage.PostgresConfig{
			ConnectionString: cfg.Database.Postgres.ConnectionString,
		}); err != nil {
			return fmt.Errorf("failed to run Postgres migrations: %w", err)
		}
		fmt.Println("✓ PostgreSQL migrations completed")

	default:
		return fmt.Errorf("unsupported database backend: %s", cfg.Database.Backend)
	}

	return nil
}

func printUsage() {
	fmt.Println(`Usage: plan-viewer <command>

Commands:
  sync                Copy and index plans from source directory
  serve [-addr :port] Start web server (default: :8081)
  tui                 Start terminal user interface
  migrate             Run database migrations

Configuration:
  Place a plan-viewer.toml file in the current directory to configure:
  - Database backend (sqlite or postgres)
  - Connection settings
  - Directory paths

  If no config file exists, defaults to SQLite at ~/.claude-viewer/plans.db

Environment:
  DEBUG=1                       Enable debug mode for TUI
  PLAN_VIEWER_DB_BACKEND        Override database backend
  PLAN_VIEWER_POSTGRES_URL      Override Postgres connection string

Examples:
  plan-viewer sync
  plan-viewer serve
  plan-viewer serve -addr :3000
  plan-viewer tui
  plan-viewer migrate
  DEBUG=1 plan-viewer tui`)
}
