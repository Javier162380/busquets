package components

import (
	"strings"

	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MetadataRow is a single label/value line in the metadata modal. Copyable
// marks rows the user can copy to the clipboard (paths); informational rows
// (size, reading time, timestamps) leave it false.
type MetadataRow struct {
	Label    string
	Value    string
	Copyable bool
}

// MetadataModal is a read-only overlay showing plan/version metadata
// (paths, size, reading time, timestamps). Rows are navigable as one
// ordered list regardless of whether they're copyable — only a copyable
// row responds to "c".
type MetadataModal struct {
	active        bool
	title         string
	rows          []MetadataRow
	selectedIdx   int
	width, height int
}

// NewMetadataModal creates an inactive metadata modal ready to be opened.
func NewMetadataModal() *MetadataModal {
	return &MetadataModal{}
}

// Open activates the modal with the given title and rows, selecting the first row.
func (m *MetadataModal) Open(title string, rows []MetadataRow) {
	m.title = title
	m.rows = rows
	m.selectedIdx = 0
	m.active = true
}

// Close deactivates the modal.
func (m *MetadataModal) Close() {
	m.active = false
}

// IsActive returns whether the modal is visible.
func (m *MetadataModal) IsActive() bool {
	return m.active
}

// SetSize updates the modal dimensions.
func (m *MetadataModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles keyboard input and returns a command.
func (m *MetadataModal) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case "up":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}
	case "down":
		if m.selectedIdx < len(m.rows)-1 {
			m.selectedIdx++
		}
	case "c":
		if len(m.rows) == 0 {
			return nil
		}
		row := m.rows[m.selectedIdx]
		if !row.Copyable {
			return nil
		}
		return func() tea.Msg {
			return messages.CopyToClipboardMsg{Text: row.Value, Label: row.Label}
		}
	case "esc":
		m.Close()
	}

	return nil
}

// View renders the metadata modal.
func (m *MetadataModal) View() string {
	if !m.active {
		return ""
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor).
		Padding(1, 2).
		Width(m.width - 2)

	titleStyle := lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true)
	selectedLabelStyle := lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true)
	mutedLabelStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	helpStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	var body strings.Builder
	body.WriteString(titleStyle.Render(m.title))
	body.WriteString("\n\n")

	for i, row := range m.rows {
		if i == m.selectedIdx {
			body.WriteString(selectedLabelStyle.Render("❯ " + row.Label))
		} else {
			body.WriteString(mutedLabelStyle.Render("  " + row.Label))
		}
		body.WriteString("\n")
		body.WriteString("  " + row.Value)
		body.WriteString("\n\n")
	}

	help := "up/down: select  esc: close"
	if len(m.rows) > 0 && m.rows[m.selectedIdx].Copyable {
		help = "up/down: select  c: copy  esc: close"
	}
	body.WriteString(helpStyle.Render(help))

	return modalStyle.Render(body.String())
}
