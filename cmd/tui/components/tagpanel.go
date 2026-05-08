package components

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UntaggedSentinel is the Name value used in TagPanelEntry to represent the "Untagged" category.
const UntaggedSentinel = "\x00untagged"

// CreateTagRequestedMsg is emitted by TagPanel when a new tag creation is confirmed.
type CreateTagRequestedMsg struct {
	Name string
}

// TagPanelEntry represents one row in the tag panel.
type TagPanelEntry struct {
	Name  string // "" == "All", UntaggedSentinel == "Untagged", otherwise the tag name
	Count int
}

// TagPanel is a lightweight scrollable tag list with optional inline tag creation.
// It does not wrap bubbles/list — it renders a simple cursor-highlighted list
// so it stays narrow without pagination or filtering overhead.
type TagPanel struct {
	entries     []TagPanelEntry
	cursor      int
	width       int
	height      int
	focused     bool
	creating    bool
	createInput textinput.Model
}

// NewTagPanel creates a new TagPanel sized to the given dimensions.
func NewTagPanel(width, height int) *TagPanel {
	ti := textinput.New()
	ti.Placeholder = "new tag name..."
	ti.Prompt = "❯ "
	ti.PromptStyle = styles.AccentStyle
	ti.Width = width - 6

	return &TagPanel{
		entries:     []TagPanelEntry{{Name: "", Count: 0}},
		width:       width,
		height:      height,
		createInput: ti,
	}
}

// SetEntries replaces the tag list. "All" and "Untagged" entries are prepended automatically.
// totalPlans is the accurate total plan count for the "All" row.
// untaggedCount is the number of plans with no tags.
func (t *TagPanel) SetEntries(entries []TagPanelEntry, totalPlans, untaggedCount int) {
	header := []TagPanelEntry{
		{Name: "", Count: totalPlans},
		{Name: UntaggedSentinel, Count: untaggedCount},
	}

	t.entries = header
	t.entries = append(t.entries, entries...)
	if t.cursor >= len(t.entries) {
		t.cursor = 0
	}
}

// Focus marks the panel as focused (affects cursor rendering).
func (t *TagPanel) Focus() {
	t.focused = true
}

// Blur marks the panel as unfocused.
func (t *TagPanel) Blur() {
	t.focused = false
}

// SelectedTag returns the Name of the entry under the cursor.
// An empty string means "All" (no tag filter applied).
func (t *TagPanel) SelectedTag() string {
	if t.cursor < 0 || t.cursor >= len(t.entries) {
		return ""
	}
	return t.entries[t.cursor].Name
}

// IsCreating returns true while the inline tag creation input is open.
func (t *TagPanel) IsCreating() bool {
	return t.creating
}

// SetSize updates panel dimensions.
func (t *TagPanel) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.createInput.Width = width - 6
}

// MoveUp moves the cursor up one entry and returns the newly selected tag name.
func (t *TagPanel) MoveUp() string {
	if t.cursor > 0 {
		t.cursor--
	}
	return t.SelectedTag()
}

// MoveDown moves the cursor down one entry and returns the newly selected tag name.
func (t *TagPanel) MoveDown() string {
	if t.cursor < len(t.entries)-1 {
		t.cursor++
	}
	return t.SelectedTag()
}

// StartCreating opens the inline tag creation input.
func (t *TagPanel) StartCreating() tea.Cmd {
	t.creating = true
	t.createInput.Reset()
	return t.createInput.Focus()
}

// Update handles messages when the panel is in creation mode.
// When the user confirms (enter) it emits CreateTagRequestedMsg;
// esc cancels. Navigation keys are handled by the parent screen directly.
func (t *TagPanel) Update(msg tea.Msg) tea.Cmd {
	if !t.creating {
		return nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			t.creating = false
			t.createInput.Reset()
			return nil
		case "enter":
			name := strings.TrimSpace(t.createInput.Value())
			t.creating = false
			t.createInput.Reset()
			if name == "" {
				return nil
			}
			return func() tea.Msg {
				return CreateTagRequestedMsg{Name: name}
			}
		}
	}
	var cmd tea.Cmd
	t.createInput, cmd = t.createInput.Update(msg)
	return cmd
}

// View renders the tag panel as a scrollable list with optional creation input.
func (t *TagPanel) View() string {
	var sb strings.Builder

	// Reserve rows for the creation input when active.
	maxRows := t.height
	if t.creating {
		maxRows -= 3
	}
	if maxRows < 1 {
		maxRows = 1
	}

	// Keep cursor visible inside the scroll window.
	start := 0
	if t.cursor >= maxRows {
		start = t.cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(t.entries) {
		end = len(t.entries)
	}

	maxLabel := t.width - 9
	if maxLabel < 4 {
		maxLabel = 4
	}

	for i := start; i < end; i++ {
		entry := t.entries[i]
		label := "All"
		switch entry.Name {
		case UntaggedSentinel:
			label = "Untagged"
		case "":
			label = "All"
		default:
			label = entry.Name
		}

		if len(label) > maxLabel {
			label = label[:maxLabel-1] + "…"
		}

		countStr := fmt.Sprintf("(%d)", entry.Count)
		line := fmt.Sprintf("%-*s %s", maxLabel, label, countStr)

		if i == t.cursor {
			if t.focused {
				sb.WriteString(styles.ActiveStyle.Render("❯ " + line))
			} else {
				sb.WriteString(styles.MutedStyle.Render("❯ " + line))
			}
		} else {
			sb.WriteString(styles.InactiveStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	if t.creating {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.AccentColor).Render("New tag:"))
		sb.WriteString("\n")
		sb.WriteString(t.createInput.View())
	}

	return sb.String()
}
