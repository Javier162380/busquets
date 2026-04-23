package components

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Editor wraps textarea with modification tracking.
type Editor struct {
	textarea textarea.Model
	original string
	modified bool
	width    int
	height   int
}

// NewEditor creates a new editor component.
func NewEditor(width, height int) *Editor {
	ta := textarea.New()
	ta.Placeholder = "Plan content will appear here..."
	ta.ShowLineNumbers = true
	ta.SetWidth(width)
	ta.SetHeight(height)
	ta.Blur()

	return &Editor{
		textarea: ta,
		width:    width,
		height:   height,
	}
}

// SetContent sets the editor content and resets modification state.
func (e *Editor) SetContent(content string) {
	e.original = content
	e.textarea.SetValue(content)
	e.modified = false
}

// Content returns the current editor content.
func (e *Editor) Content() string {
	return e.textarea.Value()
}

// IsModified returns true if content has been modified.
func (e *Editor) IsModified() bool {
	return e.modified
}

// SetSize updates the editor dimensions.
func (e *Editor) SetSize(width, height int) {
	e.width = width
	e.height = height
	e.textarea.SetWidth(width)
	e.textarea.SetHeight(height)
}

// Focus gives focus to the editor.
func (e *Editor) Focus() tea.Cmd {
	// Move cursor to beginning of document before focusing
	e.textarea.Focus()
	e.MoveCursorToFirstRow()
	return nil
}

// Blur removes focus from the editor.
func (e *Editor) Blur() {
	e.textarea.Blur()
}

// Focused returns true if the editor has focus.
func (e *Editor) Focused() bool {
	return e.textarea.Focused()
}

// Update handles editor updates.
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	e.textarea, cmd = e.textarea.Update(msg)

	// Check if content was modified.
	if e.textarea.Value() != e.original {
		e.modified = true
	}

	return cmd
}

// View renders the editor.
func (e *Editor) View() string {
	return e.textarea.View()
}

// Reset restores the original content.
func (e *Editor) Reset() {
	e.textarea.SetValue(e.original)
	e.modified = false
}

// MoveCursorToFirstRow moves the cursor to the very beginning of the document (row 0, col 0).
func (e *Editor) MoveCursorToFirstRow() {
	e.moveCursorToStart()
}

// MoveCursorToLastRow moves the cursor to the very end of the document.
func (e *Editor) MoveCursorToLastRow() {
	e.moveCursorToEnd()
}

// moveCursorToStart moves the cursor to the very beginning (row 0, col 0).
// This simulates pressing alt+< which triggers the InputBegin keybinding.
func (e *Editor) moveCursorToStart() {
	// Simulate alt+< key press to trigger moveToBegin() internally
	altLessThanMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'<'},
		Alt:   true,
	}
	e.textarea, _ = e.textarea.Update(altLessThanMsg)
}

// moveCursorToEnd moves the cursor to the very end of the document.
// This simulates pressing alt+> which triggers the InputEnd keybinding.
func (e *Editor) moveCursorToEnd() {
	// Simulate alt+> key press to trigger moveToEnd() internally
	altGreaterThanMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'>'},
		Alt:   true,
	}
	e.textarea, _ = e.textarea.Update(altGreaterThanMsg)
}
