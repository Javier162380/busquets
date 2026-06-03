package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommentModal is the overlay for viewing and adding comments on a plan.
type CommentModal struct {
	fileName   string
	syncSource string
	comments   []claudeviewer.Comment
	selected   int
	focus      types.Focus
	input      textarea.Model
	viewport   viewport.Model
	confirm    *ConfirmModal
	width      int
	height     int
	active     bool
}

// NewCommentModal creates an inactive comment modal ready to be opened.
func NewCommentModal() *CommentModal {
	ta := textarea.New()
	ta.Placeholder = "Write a comment..."
	ta.CharLimit = 2000
	ta.ShowLineNumbers = false
	ta.SetHeight(4)

	return &CommentModal{
		input:   ta,
		confirm: NewConfirmModal(),
	}
}

// Open activates the modal for the given plan and populates it with comments.
func (m *CommentModal) Open(fileName, syncSource string, comments []claudeviewer.Comment) {
	m.fileName = fileName
	m.syncSource = syncSource
	m.comments = comments
	m.selected = 0
	m.focus = types.FocusCommentList
	m.active = true
	m.input.Reset()
	m.input.Blur()
	m.confirm.Close()
	m.refreshViewport()
}

// Close deactivates the modal.
func (m *CommentModal) Close() {
	m.active = false
	m.input.Blur()
}

// IsActive returns whether the modal is visible.
func (m *CommentModal) IsActive() bool {
	return m.active
}

// SetComments replaces the comment list and refreshes the viewport.
func (m *CommentModal) SetComments(comments []claudeviewer.Comment) {
	m.comments = comments
	if m.selected >= len(m.comments) && len(m.comments) > 0 {
		m.selected = len(m.comments) - 1
	}
	m.refreshViewport()
}

// SetSize updates the modal dimensions.
func (m *CommentModal) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(width - 6)
	listHeight := m.listHeight()
	if listHeight > 0 {
		m.viewport = viewport.New(width-6, listHeight)
	}
	m.refreshViewport()
}

// Update handles keyboard input and returns a command.
func (m *CommentModal) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}

	// Confirm modal takes priority.
	if m.confirm.IsActive() {
		return m.confirm.Update(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		if m.focus == types.FocusCommentInput {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return cmd
		}
		return nil
	}

	switch m.focus {
	case types.FocusCommentList:
		return m.handleListKey(keyMsg)
	case types.FocusCommentInput:
		return m.handleInputKey(keyMsg)
	default:
		return nil
	}
}

func (m *CommentModal) handleListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up":
		if m.selected > 0 {
			m.selected--
			m.refreshViewport()
		}
	case "down":
		if m.selected < len(m.comments)-1 {
			m.selected++
			m.refreshViewport()
		}
	case "d":
		if len(m.comments) == 0 {
			return nil
		}
		target := m.comments[m.selected]
		m.confirm.Open("Delete this comment?", func() tea.Msg {
			return messages.DeleteCommentMsg{
				CommentID:  target.ID,
				FileName:   m.fileName,
				SyncSource: m.syncSource,
			}
		})
	case "tab":
		m.focus = types.FocusCommentInput
		m.input.Focus()
	case "esc":
		m.Close()
	}
	return nil
}

func (m *CommentModal) handleInputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+s":
		content := strings.TrimSpace(m.input.Value())
		if content == "" {
			return nil
		}
		m.input.Reset()
		return func() tea.Msg {
			return messages.AddCommentMsg{
				FileName:   m.fileName,
				SyncSource: m.syncSource,
				Content:    content,
			}
		}
	case "tab":
		m.focus = types.FocusCommentList
		m.input.Blur()
	case "esc":
		if m.input.Value() != "" {
			m.input.Reset()
			return nil
		}
		m.Close()
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return cmd
	}
	return nil
}

// View renders the comment modal.
func (m *CommentModal) View() string {
	if !m.active {
		return ""
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor).
		Padding(0, 1).
		Width(m.width - 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(styles.AccentColor).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	helpStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	title := titleStyle.Render(fmt.Sprintf("Comments (%d)", len(m.comments)))
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", mutedStyle.Render("[esc: close]"))

	divider := strings.Repeat("─", m.width-6)

	var body strings.Builder
	body.WriteString(header)
	body.WriteString("\n")
	body.WriteString(mutedStyle.Render(divider))
	body.WriteString("\n")

	body.WriteString(m.viewport.View())
	body.WriteString("\n")
	body.WriteString(mutedStyle.Render(divider))
	body.WriteString("\n")

	inputLabel := "Add comment:"
	if m.focus == types.FocusCommentInput {
		inputLabel = lipgloss.NewStyle().Foreground(styles.AccentColor).Render("Add comment:")
	}
	body.WriteString(inputLabel)
	body.WriteString("\n")
	body.WriteString(m.input.View())
	body.WriteString("\n")

	var hints []string
	if m.focus == types.FocusCommentList && len(m.comments) > 0 {
		hints = append(hints, "d: delete  tab: add new")
	} else {
		hints = append(hints, "ctrl+s: save  tab: list  esc: cancel")
	}
	body.WriteString(helpStyle.Render(strings.Join(hints, "  ")))

	rendered := modalStyle.Render(body.String())

	if m.confirm.IsActive() {
		rendered = overlayCenter(m.confirm.View(), m.width, m.height)
	}

	return rendered
}

// refreshViewport rebuilds the viewport content from the current comment list.
func (m *CommentModal) refreshViewport() {
	if m.height == 0 {
		return
	}

	var lines []string

	selectedStyle := lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(styles.ForegroundColor)
	metaStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	maxWidth := m.width - 8

	for i, c := range m.comments {
		ts := metaStyle.Render("[" + c.CreatedAt.Format(time.DateTime) + "]")
		content := wrapText(c.Content, maxWidth)

		var prefix string
		if i == m.selected && m.focus == types.FocusCommentList {
			prefix = selectedStyle.Render("> ")
		} else {
			prefix = "  "
		}

		firstLine := prefix + ts + " " + normalStyle.Render(firstLineOf(content))
		lines = append(lines, firstLine)

		rest := restLinesOf(content)
		for _, l := range rest {
			lines = append(lines, "    "+normalStyle.Render(l))
		}
		lines = append(lines, "")
	}

	if len(m.comments) == 0 {
		lines = append(lines, metaStyle.Render("  No comments yet. Press tab to add one."))
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m *CommentModal) listHeight() int {
	// Total modal height minus: border(2) + title(1) + divider(1) + divider(1) + label(1) + input(4) + help(1) + padding(2)
	h := m.height - 13
	if h < 3 {
		return 3
	}
	return h
}

// overlayCenter places overlay centered within a width×height area.
func overlayCenter(overlay string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(styles.MutedColor),
	)
}

func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return text
	}
	var sb strings.Builder
	for len(text) > maxWidth {
		sb.WriteString(text[:maxWidth])
		sb.WriteString("\n")
		text = text[maxWidth:]
	}
	sb.WriteString(text)
	return sb.String()
}

func firstLineOf(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}

func restLinesOf(s string) []string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return strings.Split(s[idx+1:], "\n")
	}
	return nil
}
