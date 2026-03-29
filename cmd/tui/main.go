// Package tui main tui application.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Start launches the TUI application.
func Start(service UnifiedService) error {
	app := New(service)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
