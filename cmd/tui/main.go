// Package tui main tui application.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
)

// StartWithOptions launches the TUI application with optional debug mode.
func StartWithOptions(ctx context.Context, service claudeviewer.UnifiedService, debug bool) error {
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

	// Check if watch mode should be auto-started
	setting, exists, _ := service.GetSetting(ctx, claudeviewer.SettingWatchModeEnabled)
	if exists && setting.IsBoolean() && setting.GetBooleanValue() {
		// Get interval
		intervalSetting, exists, _ := service.GetSetting(ctx, claudeviewer.SettingWatchIntervalSeconds)
		intervalSeconds := 5.0 // default
		if exists && intervalSetting.IsNumber() {
			intervalSeconds = intervalSetting.GetNumberValue()
		}

		// Start watch mode
		if err := service.StartWatchMode(ctx, intervalSeconds); err != nil {
			// Log warning but don't fail
			fmt.Fprintf(os.Stderr, "Warning: failed to start watch mode: %v\n", err)
		}
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
