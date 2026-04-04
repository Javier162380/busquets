// Package tui main tui application.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// StartWithOptions launches the TUI application with optional debug mode.
func StartWithOptions(ctx context.Context, service UnifiedService, debug bool) error {
	app := New(ctx, service)

	if debug {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		logPath := filepath.Join(homeDir, ".claude-viewer", "tui-debug.log")
		//nolint:gosec // G304: Debug log path is constructed from home directory
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open debug log: %w", err)
		}
		defer f.Close()

		// Write session separator
		sessionStart := fmt.Sprintf("\n\n========== SESSION START: %s ==========\n\n",
			time.Now().Format("2006-01-02 15:04:05"))
		_, err = f.WriteString(sessionStart)
		if err != nil {
			return fmt.Errorf("failed to write to debug log: %w", err)
		}

		app.SetDump(f)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
