package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	httpserver "github.com/Javier162380/claude-plan-viewer/cmd/http"
	mcphandler "github.com/Javier162380/claude-plan-viewer/cmd/mcp"
	tuiapp "github.com/Javier162380/claude-plan-viewer/cmd/tui"
	"github.com/Javier162380/claude-plan-viewer/internal/config"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors/ollama"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors/telegram"
	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/postgres"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/sqlite"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newLogger creates a logger appropriate for the given command.
// MCP uses stderr to avoid corrupting the stdio transport (stdout is the MCP protocol).
// TUI uses a file to avoid corrupting alt-screen rendering.
// All other commands use stderr.
func newLogger(command string) *slog.Logger {
	switch command {
	case "tui":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		}
		logDir := filepath.Join(homeDir, ".claude-viewer")
		if err := os.MkdirAll(logDir, 0o750); err != nil {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		}
		//nolint:gosec // G304: log path is constructed from home directory, not user input
		f, err := os.OpenFile(
			filepath.Join(logDir, "app.log"),
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
	logger := newLogger(command)

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

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
	case "serve":
		if err := runServe(cfg, logger); err != nil {
			logger.Error("server failed", "error", err)
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
		repo := sqlite.NewRepository(db.DB())
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

func runSync(cfg *config.Config, logger *slog.Logger) error {
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
	defer service.Close()
	service.SetLogger(logger)

	count, err := service.SyncPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync plans: %w", err)
	}

	fmt.Printf("✓ Synced %d plans from %s to %s (backend: %s)\n",
		count, cfg.Paths.PlansDir, cfg.Paths.ViewerDir, cfg.Database.Backend)
	return nil
}

func runRSync(cfg *config.Config, logger *slog.Logger) error {
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
	defer service.Close()
	service.SetLogger(logger)

	count, err := service.RSyncPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync plans: %w", err)
	}

	fmt.Printf("✓ RSynced %d plans from %s to %s (backend: %s)\n",
		count, cfg.Paths.ViewerDir, cfg.Paths.PlansDir, cfg.Database.Backend)
	return nil
}

func runDump(cfg *config.Config, logger *slog.Logger) error {
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
	defer service.Close()
	service.SetLogger(logger)

	count, err := service.DumpPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to dump plans: %w", err)
	}

	fmt.Printf("✓ Dumped %d plans from database to %s (backend: %s)\n",
		count, cfg.Paths.PlansDir, cfg.Database.Backend)
	return nil
}

func runServe(cfg *config.Config, logger *slog.Logger) error {
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
	defer service.Close()
	service.SetLogger(logger)

	server, err := httpserver.NewServer(service, *addr)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	fmt.Printf("Server running on http://localhost%s (backend: %s)\n", *addr, cfg.Database.Backend)
	return server.Server().Start(*addr)
}

func runTUI(cfg *config.Config, logger *slog.Logger) error {
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
	defer service.Close()
	service.SetLogger(logger)

	// Initialize connectors
	registry := connectors.NewRegistry()
	_ = registry.Register(telegram.New())
	_ = registry.Register(ollama.New())

	connectorManager := connectors.NewManager(registry, repo)
	service.SetConnectorManager(connectorManager)

	return tuiapp.StartWithOptions(ctx, service, debug, logger)
}

func runMigrate(cfg *config.Config, _ *slog.Logger) error {
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

func runMCP(cfg *config.Config, logger *slog.Logger) error {
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
	fmt.Println(`Usage: plan-viewer <command>

Commands:
  sync                Copy and index plans from source directory
  dump                Write all plans from the database back to the source plans directory
  serve [-addr :port] Start web server (default: :8081)
  tui                 Start terminal user interface
  mcp                 Start MCP server for Claude integration
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
  plan-viewer mcp
  plan-viewer migrate
  DEBUG=1 plan-viewer tui`)
}
