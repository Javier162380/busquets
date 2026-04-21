// Package screens provides screen implementations for the TUI.
package screens

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Screen is the interface that all screens must implement.
type Screen interface {
	// Init initializes the screen and returns an initial command.
	Init() tea.Cmd

	// Update handles messages and returns the updated screen and a command.
	Update(msg tea.Msg) (Screen, tea.Cmd)

	// View renders the screen content.
	View() string

	// SetSize updates the screen dimensions.
	SetSize(width, height int)

	// ShortHelp returns key binding help text for the status bar.
	ShortHelp() string

	// IsInputMode returns true when the screen is capturing text input.
	IsInputMode() bool
}

// Layout determines how content is displayed.
type Layout int

const (
	LayoutSplit      Layout = iota // Two panels side-by-side.
	LayoutFullscreen               // Single content area.
)

// Focus determines where user attention is.
type Focus int

const (
	FocusList      Focus = iota // Navigating a list.
	FocusContent                // Viewing content.
	FocusEditor                 // Editing content.
	FocusSearch                 // Search input.
	FocusTagFilter              // Tag filter input.
)
