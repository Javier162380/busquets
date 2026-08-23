package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	mcphandler "github.com/Javier162380/busquets/cmd/mcp"
	tuiapp "github.com/Javier162380/busquets/cmd/tui"
	"github.com/Javier162380/busquets/internal/config"
	"github.com/Javier162380/busquets/internal/connectors"
	"github.com/Javier162380/busquets/internal/connectors/ollama"
	"github.com/Javier162380/busquets/internal/connectors/telegram"
	"github.com/Javier162380/busquets/internal/storage"
	"github.com/Javier162380/busquets/services/busquets"
	"github.com/Javier162380/busquets/services/busquets/dto"
	"github.com/Javier162380/busquets/services/busquets/repository/postgres"
	"github.com/Javier162380/busquets/services/busquets/repository/sqlite"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newLogger creates a logger appropriate for the given command.
// MCP uses stderr to avoid corrupting the stdio transport (stdout is the MCP protocol).
// TUI uses a file to avoid corrupting alt-screen rendering.
// All other commands use stderr.
func newLogger(command string, cfg *config.Config) *slog.Logger {
	switch command {
	case "tui":
		if err := os.MkdirAll(cfg.Paths.ViewerDir, 0o750); err != nil {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		}
		//nolint:gosec // G304: log path is constructed from configured viewer dir, not user input
		f, err := os.OpenFile(
			filepath.Join(cfg.Paths.ViewerDir, "app.log"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600,
		)
		if err != nil {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		}
		return slog.New(slog.NewTextHandler(f, nil))
	default:
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Must run before newLogger or initRepository touch the viewer
	// directory — see config.MigrateLegacyViewerDir's doc comment for why
	// this needs to happen first, and its own idempotency guarantees.
	if err := config.MigrateLegacyViewerDir(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to migrate legacy viewer directory: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(command, cfg)

	switch command {
	case "sync":
		if err := runSync(cfg, logger); err != nil {
			logger.Error("sync failed", "error", err)
			os.Exit(1)
		}
	case "rsync":
		if err := runRSync(cfg, logger); err != nil {
			logger.Error("rsync failed", "error", err)
			os.Exit(1)
		}
	case "dump":
		if err := runDump(cfg, logger); err != nil {
			logger.Error("dump failed", "error", err)
			os.Exit(1)
		}
	case "tui":
		if err := runTUI(cfg, logger); err != nil {
			logger.Error("TUI failed", "error", err)
			os.Exit(1)
		}
	case "migrate":
		if err := runMigrate(cfg, logger); err != nil {
			logger.Error("migration failed", "error", err)
			os.Exit(1)
		}
	case "mcp":
		if err := runMCP(cfg, logger); err != nil {
			logger.Error("MCP server failed", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// initRepository creates a repository based on the configuration.
// Returns the repository and a cleanup function.
func initRepository(ctx context.Context, cfg *config.Config, logger *slog.Logger) (dto.Repository, func(), error) {
	// Ensure viewer directory exists
	//nolint:gosec // G301: Standard directory permissions for user application directory
	if err := os.MkdirAll(cfg.Paths.ViewerDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create viewer directory: %w", err)
	}

	switch cfg.Database.Backend {
	case config.BackendSQLite:
		db, err := storage.NewSQLiteClientWithMigrations(ctx, cfg.Database.SQLite.Path, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize SQLite: %w", err)
		}
		repo := sqlite.NewRepository(db.DB())
		cleanup := func() { _ = db.Close() }
		return repo, cleanup, nil

	case config.BackendPostgres:
		// Run migrations first
		if err := storage.RunPostgresMigrations(ctx, storage.PostgresConfig{
			ConnectionString: cfg.Database.Postgres.ConnectionString,
		}, logger); err != nil {
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

func runSync(cfg *config.Config, logger *slog.Logger) error {
	ctx := context.Background()

	repo, cleanup, err := initRepository(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := busquets.New(ctx, repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDirs, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}
	defer service.Close()
	service.SetLogger(logger)

	count, err := service.SyncPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync plans: %w", err)
	}

	fmt.Printf("✓ Synced %d plans to %s (backend: %s)\n",
		count, cfg.Paths.ViewerDir, cfg.Database.Backend)
	return nil
}

func runRSync(cfg *config.Config, logger *slog.Logger) error {
	ctx := context.Background()

	repo, cleanup, err := initRepository(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := busquets.New(ctx, repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDirs, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}
	defer service.Close()
	service.SetLogger(logger)

	count, err := service.RSyncPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync plans: %w", err)
	}

	fmt.Printf("✓ RSynced %d plans from %s (backend: %s)\n",
		count, cfg.Paths.ViewerDir, cfg.Database.Backend)
	return nil
}

func runDump(cfg *config.Config, logger *slog.Logger) error {
	ctx := context.Background()

	repo, cleanup, err := initRepository(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := busquets.New(ctx, repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDirs, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}
	defer service.Close()
	service.SetLogger(logger)

	count, err := service.DumpPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to dump plans: %w", err)
	}

	fmt.Printf("✓ Dumped %d plans from database (backend: %s)\n",
		count, cfg.Database.Backend)
	return nil
}

func runTUI(cfg *config.Config, logger *slog.Logger) error {
	ctx := context.Background()

	debug := os.Getenv("DEBUG") == "1"

	repo, cleanup, err := initRepository(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := busquets.New(ctx, repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDirs, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}
	defer service.Close()
	service.SetLogger(logger)

	// Initialize connectors
	registry := connectors.NewRegistry()
	_ = registry.Register(telegram.New())
	_ = registry.Register(ollama.New())

	connectorManager := connectors.NewManager(registry, repo)
	service.SetConnectorManager(connectorManager)

	return tuiapp.StartWithOptions(ctx, service, debug, logger, cfg.Paths.ViewerDir)
}

func runMigrate(cfg *config.Config, logger *slog.Logger) error {
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

		db, err := storage.NewSQLiteClientWithMigrations(ctx, cfg.Database.SQLite.Path, logger)
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
		}, logger); err != nil {
			return fmt.Errorf("failed to run Postgres migrations: %w", err)
		}
		fmt.Println("✓ PostgreSQL migrations completed")

	default:
		return fmt.Errorf("unsupported database backend: %s", cfg.Database.Backend)
	}

	// Storage-layout migration is separate from the schema migration above: it
	// moves plan files, not DB rows. initRepository re-running schema
	// migrations here is a harmless no-op (goose is idempotent).
	repo, cleanup, err := initRepository(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := busquets.New(ctx, repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDirs, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}
	defer service.Close()
	service.SetLogger(logger)

	migrated, err := service.MigrateStorageLayout(ctx)
	if err != nil {
		return fmt.Errorf("failed to migrate storage layout: %w", err)
	}
	fmt.Printf("✓ Storage layout migration: %d plan(s) migrated to id-keyed storage\n", migrated)

	return nil
}

func runMCP(cfg *config.Config, logger *slog.Logger) error {
	ctx := context.Background()

	repo, cleanup, err := initRepository(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer cleanup()

	service, err := busquets.New(ctx, repo, cfg.Paths.ViewerDir, cfg.Paths.PlansDirs, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}
	defer service.Close()
	service.SetLogger(logger)

	// Create MCP server with config
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    cfg.MCP.ServerName,
			Version: cfg.MCP.Version,
		}, nil,
	)

	handler := mcphandler.NewHandler(service, mcpServer)
	if err := handler.Register(); err != nil {
		return fmt.Errorf("failed to register MCP tools: %w", err)
	}

	// Log to stderr — stdout is the MCP stdio transport.
	logger.Info("starting MCP server",
		"name", cfg.MCP.ServerName,
		"version", cfg.MCP.Version,
		"backend", cfg.Database.Backend,
	)

	// Run with stdio transport
	transport := &mcp.StdioTransport{}
	return mcpServer.Run(ctx, transport)
}

func printUsage() {
	fmt.Println(`Usage: busquets <command>

Commands:
  sync                Copy and index plans from source directory
  dump                Write all plans from the database back to the source plans directory
  tui                 Start terminal user interface
  mcp                 Start MCP server for AI assistant integration
  migrate             Run database schema migrations and migrate plan storage
                      to the current on-disk layout. Run this after upgrading,
                      before starting the TUI/MCP server.

Configuration:
  Place a busquets.toml file in the current directory to configure:
  - Database backend (sqlite or postgres)
  - Connection settings
  - Directory paths

  If no config file exists, defaults to SQLite at ~/.busquets/plans.db

Environment:
  DEBUG=1                       Enable debug mode for TUI
  PLAN_VIEWER_DB_BACKEND        Override database backend
  PLAN_VIEWER_POSTGRES_URL      Override Postgres connection string

Examples:
  busquets sync
  busquets tui
  busquets mcp
  busquets migrate
  DEBUG=1 busquets tui`)
}
