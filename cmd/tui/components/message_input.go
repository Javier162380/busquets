package components

import (
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// MessageInput provides a text input for composing messages.
type MessageInput struct {
	textarea textarea.Model
	active   bool
	width    int
}

// NewMessageInput creates a new message input component.
func NewMessageInput(width int) *MessageInput {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send)"
	ta.ShowLineNumbers = false
	ta.SetWidth(width)
	ta.SetHeight(3) // 3 lines for input
	ta.Blur()

	// Configure textarea for chat input
	ta.CharLimit = 10000
	ta.MaxHeight = 10 // Allow expansion up to 10 lines

	// Enable key passthrough for enter
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return &MessageInput{
		textarea: ta,
		width:    width,
		active:   false,
	}
}

// Focus gives focus to the message input.
func (m *MessageInput) Focus() tea.Cmd {
	m.active = true
	return m.textarea.Focus()
}

// Blur removes focus from the message input.
func (m *MessageInput) Blur() {
	m.active = false
	m.textarea.Blur()
}

// Value returns the current message content.
func (m *MessageInput) Value() string {
	return m.textarea.Value()
}

// Clear clears the message input.
func (m *MessageInput) Clear() {
	m.textarea.Reset()
}

// SetSize updates the input width.
func (m *MessageInput) SetSize(width int) {
	m.width = width
	m.textarea.SetWidth(width)
}

// Update handles messages for the textarea.
func (m *MessageInput) Update(msg tea.Msg) tea.Cmd {
	// Don't let the textarea handle plain enter key - we use it for sending
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "enter" {
			// Let the parent handle this for message sending
			return nil
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return cmd
}

// View renders the message input.
func (m *MessageInput) View() string {
	if m.active {
		return styles.MutedStyle.Render(m.textarea.View())
	}
	return styles.MetaStyle.Render(m.textarea.View())
}

// IsActive returns whether the input is currently focused.
func (m *MessageInput) IsActive() bool {
	return m.active
}

// Focused returns whether the input is currently focused (alias for IsActive).
func (m *MessageInput) Focused() bool {
	return m.active
}
