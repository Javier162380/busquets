// Package tui main tui application.
package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
)

// StartWithOptions launches the TUI application with optional debug mode.
// viewerDir is only used when debug is true, to place tui-debug.log
// alongside the app's other on-disk state (cfg.Paths.ViewerDir) rather than
// reconstructing a path independently — doing so would recreate a stray
// ~/.claude-viewer directory post-rebrand and confuse
// config.MigrateLegacyViewerDir on the next run.
func StartWithOptions(ctx context.Context, service busquets.UnifiedService, debug bool, logger *slog.Logger, viewerDir string) error {
	app := New(ctx, service)

	if debug {
		logPath := filepath.Join(viewerDir, "tui-debug.log")
		//nolint:gosec // G304: Debug log path is constructed from the configured viewer directory
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
	setting, exists, _ := service.GetSetting(ctx, busquets.SettingWatchModeEnabled)
	if exists && setting.IsBoolean() && setting.GetBooleanValue() {
		// Get interval
		intervalSetting, exists, _ := service.GetSetting(ctx, busquets.SettingWatchIntervalSeconds)
		intervalSeconds := 5.0 // default
		if exists && intervalSetting.IsNumber() {
			intervalSeconds = intervalSetting.GetNumberValue()
		}

		// Start watch mode
		if err := service.StartWatchMode(ctx, intervalSeconds); err != nil {
			logger.Warn("failed to start watch mode", "error", err)
		}
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
