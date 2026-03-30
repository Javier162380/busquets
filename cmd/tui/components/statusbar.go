package components

import (
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar displays status messages and key hints.
type StatusBar struct {
	width int

	// State.
	isLoading      bool
	loadingMessage string
	errorMessage   string
	successMessage string
	helpText       string

	// Styles.
	baseStyle lipgloss.Style
}

// NewStatusBar creates a new status bar.
func NewStatusBar(width int) *StatusBar {
	return &StatusBar{
		width:     width,
		baseStyle: styles.StatusStyle(width),
	}
}

// SetWidth updates the status bar width.
func (s *StatusBar) SetWidth(width int) {
	s.width = width
	s.baseStyle = s.baseStyle.Width(width)
}

// SetLoading sets the loading state with a message.
func (s *StatusBar) SetLoading(message string) {
	s.isLoading = true
	s.loadingMessage = message
	s.errorMessage = ""
	s.successMessage = ""
}

// SetError sets an error message.
func (s *StatusBar) SetError(message string) {
	s.isLoading = false
	s.errorMessage = message
	s.successMessage = ""
}

// SetSuccess sets a success message.
func (s *StatusBar) SetSuccess(message string) {
	s.isLoading = false
	s.successMessage = message
	s.errorMessage = ""
}

// SetHelp sets the help text (key bindings).
func (s *StatusBar) SetHelp(text string) {
	s.helpText = text
}

// Clear clears all messages.
func (s *StatusBar) Clear() {
	s.isLoading = false
	s.loadingMessage = ""
	s.errorMessage = ""
	s.successMessage = ""
}

// ClearMessages clears error and success messages but not loading.
func (s *StatusBar) ClearMessages() {
	s.errorMessage = ""
	s.successMessage = ""
}

// IsLoading returns true if loading.
func (s *StatusBar) IsLoading() bool {
	return s.isLoading
}

// View renders the status bar.
func (s *StatusBar) View() string {
	var content string

	switch {
	case s.isLoading:
		content = styles.LoadingStyle.Render(fmt.Sprintf("⟳ %s", s.loadingMessage))
	case s.errorMessage != "":
		content = styles.ErrorStyle.Render("✗ " + s.errorMessage)
	case s.successMessage != "":
		content = styles.SuccessStyle.Render("✓ " + s.successMessage)
	default:
		content = s.helpText
	}
	return s.baseStyle.Render(content)
}

// Height returns the height of the status bar (always 1).
func (s *StatusBar) Height() int {
	return 1
}
