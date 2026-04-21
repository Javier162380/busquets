package components

import (
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TagFilter is a component for filtering plans by tags.
type TagFilter struct {
	input    textinput.Model
	isActive bool
	matchAll bool // true = AND logic, false = OR logic
	width    int
	style    lipgloss.Style
}

// NewTagFilter creates a new tag filter component.
func NewTagFilter(width int) *TagFilter {
	ti := textinput.New()
	ti.Placeholder = "Enter tags (comma-separated)..."
	ti.Prompt = "Filter tags: "
	ti.PromptStyle = styles.AccentStyle
	ti.Width = width - 10

	return &TagFilter{
		input:    ti,
		isActive: false,
		matchAll: false, // Default to OR logic
		width:    width,
		style: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(styles.AccentColor).
			Padding(0, 1),
	}
}

// Focus activates the tag filter and focuses the input.
func (t *TagFilter) Focus() tea.Cmd {
	t.isActive = true
	t.input.Focus()
	return textinput.Blink
}

// Blur deactivates the tag filter.
func (t *TagFilter) Blur() {
	t.isActive = false
	t.input.Blur()
}

// Update handles input messages.
func (t *TagFilter) Update(msg tea.Msg) tea.Cmd {
	if !t.isActive {
		return nil
	}

	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return cmd
}

// View renders the tag filter.
func (t *TagFilter) View() string {
	if !t.isActive {
		return ""
	}

	// Build view with mode indicator
	modeText := "OR"
	if t.matchAll {
		modeText = "AND"
	}
	modeStyle := lipgloss.NewStyle().
		Foreground(styles.AccentColor).
		Bold(true)

	view := t.input.View() + "  " + modeStyle.Render("["+modeText+"]")

	return t.style.Width(t.width - 4).Render(view)
}

// Value returns the current input value.
func (t *TagFilter) Value() string {
	return t.input.Value()
}

// Tags returns the tags as a slice (split by comma).
func (t *TagFilter) Tags() []string {
	value := strings.TrimSpace(t.input.Value())
	if value == "" {
		return []string{}
	}

	tags := strings.Split(value, ",")
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// SetValue sets the input value.
func (t *TagFilter) SetValue(value string) {
	t.input.SetValue(value)
}

// Reset clears the input value.
func (t *TagFilter) Reset() {
	t.input.Reset()
}

// IsActive returns whether the tag filter is active.
func (t *TagFilter) IsActive() bool {
	return t.isActive
}

// MatchAll returns whether AND logic is enabled.
func (t *TagFilter) MatchAll() bool {
	return t.matchAll
}

// ToggleMatchMode toggles between AND and OR logic.
func (t *TagFilter) ToggleMatchMode() {
	t.matchAll = !t.matchAll
}

// SetWidth updates the tag filter width.
func (t *TagFilter) SetWidth(width int) {
	t.width = width
	t.input.Width = width - 10
}
