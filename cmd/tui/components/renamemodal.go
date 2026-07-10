package components

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RenameModal is the overlay for renaming a plan's file. It captures a single new
// file name and, on enter, shows a confirmation dialog before emitting the rename
// request — so the user can't rename by accident.
type RenameModal struct {
	fileName   string
	syncSource string
	filePath   string
	input      textinput.Model
	confirm    *ConfirmModal
	width      int
	height     int
	active     bool
}

// NewRenameModal creates an inactive rename modal ready to be opened.
func NewRenameModal() *RenameModal {
	ti := textinput.New()
	ti.Placeholder = "new-file-name.md"
	ti.CharLimit = 255

	return &RenameModal{
		input:   ti,
		confirm: NewConfirmModal(),
	}
}

// Open activates the modal for the given plan, pre-filling the current file name.
func (m *RenameModal) Open(fileName, syncSource, filePath string) {
	m.fileName = fileName
	m.syncSource = syncSource
	m.filePath = filePath
	m.active = true
	m.input.SetValue(fileName)
	m.input.CursorEnd()
	m.input.Focus()
	m.confirm.Close()
}

// Close deactivates the modal.
func (m *RenameModal) Close() {
	m.active = false
	m.input.Blur()
	m.confirm.Close()
}

// IsActive returns whether the modal is visible.
func (m *RenameModal) IsActive() bool {
	return m.active
}

// SetSize updates the modal dimensions.
func (m *RenameModal) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = width - 8
}

// Update handles keyboard input and returns a command.
func (m *RenameModal) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}

	// Once the confirmation dialog is showing it owns all input. When it closes
	// (confirmed or cancelled), the rename modal's job is done too.
	if m.confirm.IsActive() {
		cmd := m.confirm.Update(msg)
		if !m.confirm.IsActive() {
			m.Close()
		}
		return cmd
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return cmd
	}

	switch keyMsg.String() {
	case "esc":
		m.Close()
		return nil
	case "enter":
		newName := strings.TrimSpace(m.input.Value())
		if newName == "" || newName == m.fileName {
			return nil // nothing to rename
		}
		fileName, syncSource, filePath := m.fileName, m.syncSource, m.filePath
		m.confirm.Open(
			fmt.Sprintf("Rename %q to %q?", fileName, newName),
			func() tea.Msg {
				return messages.RenamePlanFileMsg{
					FileName:    fileName,
					SyncSource:  syncSource,
					FilePath:    filePath,
					NewFileName: newName,
				}
			},
		)
		return nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(keyMsg)
		return cmd
	}
}

// View renders the rename modal, with the confirmation dialog overlaid when active.
func (m *RenameModal) View() string {
	if !m.active {
		return ""
	}

	if m.confirm.IsActive() {
		return overlayCenter(m.confirm.View(), m.width, m.height)
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor).
		Padding(1, 2).
		Width(m.width - 2)

	titleStyle := lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	var body strings.Builder
	body.WriteString(titleStyle.Render("Rename plan file"))
	body.WriteString("\n\n")
	body.WriteString("New file name:")
	body.WriteString("\n")
	body.WriteString(m.input.View())
	body.WriteString("\n\n")
	body.WriteString(helpStyle.Render("enter: rename  esc: cancel"))

	return modalStyle.Render(body.String())
}
