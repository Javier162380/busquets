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
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "sync":
		if err := runSync(); err != nil {
			log.Fatalf("Sync failed: %v", err)
		}
	case "serve":
		if err := runServe(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	case "tui":
		if err := runTUI(); err != nil {
			log.Fatalf("TUI failed: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runSync() error {
	ctx := context.Background()

	dbPath, viewerDir, plansDir, err := getPaths()
	if err != nil {
		return err
	}

	repo, err := claudeviewer.NewRepository(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	service, err := claudeviewer.New(repo, viewerDir, plansDir, true) // true = index full content
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	count, err := service.SyncPlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync plans: %w", err)
	}

	fmt.Printf("✓ Synced %d plans from %s to %s\n", count, plansDir, viewerDir)
	return nil
}

func runServe() error {
	ctx := context.Background()

	// Parse flags
	addr := flag.String("addr", ":8081", "HTTP server address")
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	dbPath, viewerDir, plansDir, err := getPaths()
	if err != nil {
		return err
	}

	repo, err := claudeviewer.NewRepository(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	service, err := claudeviewer.New(repo, viewerDir, plansDir, true) // true = index full content
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	server, err := httpserver.NewServer(service, *addr)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	fmt.Printf("Server running on http://localhost%s\n", *addr)
	return server.Server().Start(*addr)
}

func getPaths() (dbPath, viewerDir, plansDir string, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get home directory: %w", err)
	}

	viewerDir = filepath.Join(homeDir, ".claude-viewer")
	plansDir = filepath.Join(homeDir, ".claude", "plans")

	// Ensure viewer directory exists
	//nolint:gosec // G301: Standard directory permissions for user application directory
	if err := os.MkdirAll(viewerDir, 0o755); err != nil {
		return "", "", "", fmt.Errorf("failed to create viewer directory: %w", err)
	}

	dbPath = filepath.Join(viewerDir, "plans.db")
	return dbPath, viewerDir, plansDir, nil
}

func runTUI() error {
	ctx := context.Background()

	debug := os.Getenv("DEBUG") == "1"

	dbPath, viewerDir, plansDir, err := getPaths()
	if err != nil {
		return err
	}

	repo, err := claudeviewer.NewRepository(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	service, err := claudeviewer.New(repo, viewerDir, plansDir, true)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	return tuiapp.StartWithOptions(service, debug)
}

func printUsage() {
	fmt.Println(`Usage: plan-viewer <command>

Commands:
  sync                Copy and index plans from ~/.claude/plans/
  serve [-addr :port] Start web server (default: :8081)
  tui                 Start terminal user interface

Environment:
  DEBUG=1             Enable debug mode (logs messages to ~/.claude-viewer/tui-debug.log)

Examples:
  plan-viewer sync
  plan-viewer serve
  plan-viewer serve -addr :3000
  plan-viewer tui
  DEBUG=1 plan-viewer tui`)
}
