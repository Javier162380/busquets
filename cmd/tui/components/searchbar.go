package components

import (
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchBar is a search input component.
type SearchBar struct {
	input    textinput.Model
	isActive bool
	width    int
	style    lipgloss.Style
}

// NewSearchBar creates a new search bar component.
func NewSearchBar(width int) *SearchBar {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.Prompt = "Search: "
	ti.PromptStyle = styles.AccentStyle
	ti.Width = width - 10

	return &SearchBar{
		input:    ti,
		isActive: false,
		width:    width,
		style: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(styles.AccentColor).
			Padding(0, 1),
	}
}

// Focus activates the search bar and focuses the input.
func (s *SearchBar) Focus() tea.Cmd {
	s.isActive = true
	s.input.Focus()
	return textinput.Blink
}

// Blur deactivates the search bar.
func (s *SearchBar) Blur() {
	s.isActive = false
	s.input.Blur()
}

// Update handles input messages.
func (s *SearchBar) Update(msg tea.Msg) tea.Cmd {
	if !s.isActive {
		return nil
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return cmd
}

// View renders the search bar.
func (s *SearchBar) View() string {
	if !s.isActive {
		return ""
	}
	return s.style.Width(s.width - 4).Render(s.input.View())
}

// Value returns the current input value.
func (s *SearchBar) Value() string {
	return s.input.Value()
}

// SetValue sets the input value.
func (s *SearchBar) SetValue(value string) {
	s.input.SetValue(value)
}

// Reset clears the input value.
func (s *SearchBar) Reset() {
	s.input.Reset()
}

// IsActive returns whether the search bar is active.
func (s *SearchBar) IsActive() bool {
	return s.isActive
}

// SetWidth updates the search bar width.
func (s *SearchBar) SetWidth(width int) {
	s.width = width
	s.input.Width = width - 10
}

// SetSize updates the search bar width (alias for SetWidth).
func (s *SearchBar) SetSize(width int) {
	s.SetWidth(width)
}

// Clear clears the input value (alias for Reset).
func (s *SearchBar) Clear() {
	s.Reset()
}
