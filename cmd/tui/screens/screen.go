// Package screens provides screen implementations for the TUI.
package screens

import (
	"strings"

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

	// EditorMode returns true when the screen is on an edit focus.
	EditorMode() bool
}

// overlayContent overlays a modal's rendered view on top of a screen's base
// content, line by line: any overlay line with non-whitespace content wins,
// otherwise the base line shows through. Shared by any screen that renders
// modal overlays (PlansScreen, VersionsScreen) — it's a pure function of its
// two arguments, so there's nothing screen-specific to duplicate.
func overlayContent(base, overlay string) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure both have the same number of lines
	maxLines := max(len(baseLines), len(overlayLines))

	result := make([]string, maxLines)
	for i := range maxLines {
		var baseLine, overlayLine string
		if i < len(baseLines) {
			baseLine = baseLines[i]
		}
		if i < len(overlayLines) {
			overlayLine = overlayLines[i]
		}

		// If overlay line is not empty/whitespace, use it; otherwise use base
		if strings.TrimSpace(overlayLine) != "" {
			result[i] = overlayLine
		} else {
			result[i] = baseLine
		}
	}

	return strings.Join(result, "\n")
}
