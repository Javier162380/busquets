package components

import (
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputModal is a generic single-line text input overlay with no
// confirmation step. Unlike RenameModal it has no knowledge of what the
// value means beyond how to parse it into T — callers supply a title, an
// optional rune filter, a parse function, and a submit callback.
type InputModal[T any] struct {
	input    textinput.Model
	active   bool
	title    string
	filter   func(r rune) bool // optional: reject keystrokes that fail this check
	parse    func(raw string) (T, error)
	onSubmit func(value T)
	width    int
	height   int
}

// NewInputModal creates an inactive input modal ready to be opened.
func NewInputModal[T any]() *InputModal[T] {
	return &InputModal[T]{input: textinput.New(), width: 40}
}

// Open activates the modal.
//   - title is rendered above the input (e.g. "Go to line (1-120)").
//   - placeholder is shown in the empty input.
//   - filter, if non-nil, is called per keystroke rune; a rejected rune drops the whole keystroke.
//   - parse converts the trimmed input into T on Enter; a non-nil error closes the modal without submitting.
//   - onSubmit is called with the parsed value when parse succeeds.
func (m *InputModal[T]) Open(title, placeholder string, filter func(r rune) bool, parse func(string) (T, error), onSubmit func(T)) {
	m.active = true
	m.title = title
	m.filter = filter
	m.parse = parse
	m.onSubmit = onSubmit
	m.input.Placeholder = placeholder
	m.input.SetValue("")
	m.input.Focus()
}

// Close deactivates the modal without emitting anything.
func (m *InputModal[T]) Close() {
	m.active = false
	m.input.Blur()
	m.filter = nil
	m.parse = nil
	m.onSubmit = nil
}

// IsActive returns whether the modal is currently shown.
func (m *InputModal[T]) IsActive() bool { return m.active }

// SetSize updates the modal dimensions.
func (m *InputModal[T]) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = width - 8
}

// Update handles keyboard input and returns a command.
func (m *InputModal[T]) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEsc:
			m.Close()
			return nil
		case tea.KeyEnter:
			raw := strings.TrimSpace(m.input.Value())
			parse, onSubmit := m.parse, m.onSubmit
			m.Close()
			if raw == "" {
				return nil
			}
			if value, err := parse(raw); err == nil && onSubmit != nil {
				onSubmit(value)
			}
			return nil
		case tea.KeyRunes:
			if m.filter != nil {
				for _, r := range keyMsg.Runes {
					if !m.filter(r) {
						return nil
					}
				}
			}
		default:
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// View renders the input modal.
func (m *InputModal[T]) View() string {
	if !m.active {
		return ""
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor).
		Padding(1, 2).
		Width(m.width - 2)

	titleStyle := lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	var body strings.Builder
	body.WriteString(titleStyle.Render(m.title))
	body.WriteString("\n\n")
	body.WriteString(m.input.View())
	body.WriteString("\n\n")
	body.WriteString(helpStyle.Render("enter: confirm  esc: cancel"))

	return modalStyle.Render(body.String())
}
