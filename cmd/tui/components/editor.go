package components

import (
	"github.com/Javier162380/busquets/cmd/tui/types"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Editor wraps textarea with modification tracking.
type Editor struct {
	textarea     textarea.Model
	editorStatus types.EditorMode
	original     string
	modified     bool
	width        int
	height       int
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
		textarea:     ta,
		width:        width,
		height:       height,
		editorStatus: types.EditorModeNavigation,
	}
}

// SetContent sets the editor content and resets modification state.
func (e *Editor) SetContent(content string) {
	e.original = content
	e.textarea.SetValue(content)
	e.modified = false
}

// SetEditorMode sets editor mode.
func (e *Editor) SetEditorMode(mode types.EditorMode) {
	e.editorStatus = mode
}

func (e *Editor) EditorMode() types.EditorMode {
	return e.editorStatus
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

// FocusAtLine gives focus to the editor and moves the cursor to the given
// 1-indexed line instead of the beginning of the document.
func (e *Editor) FocusAtLine(line int) tea.Cmd {
	e.textarea.Focus()
	e.JumpToLine(line)
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

// MarkSaved updates the baseline content used by IsModified and Reset to a
// just-saved value, without touching the current textarea value or cursor.
// Used after a background save completes while the editor is still focused:
// Reset (esc) and modification tracking should compare against the new save
// point, but the user's in-progress typing (and cursor position) must be
// left alone, unlike SetContent which replaces the textarea outright.
func (e *Editor) MarkSaved(content string) {
	e.original = content
	e.modified = e.textarea.Value() != content
}

// CurrentLine returns the cursor's current 1-indexed line.
func (e *Editor) CurrentLine() int {
	return e.textarea.Line() + 1
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

const maxJumpSteps = 100000

// jumpNoOpMsg is sent through textarea.Update solely to trigger its internal
// repositionView (which only runs inside Update, never from CursorUp/
// CursorDown directly) so the textarea's own viewport scrolls to keep the
// cursor visible after JumpToLine moves it. It's not a key or paste message,
// so no binding matches it and no content is touched.
type jumpNoOpMsg struct{}

// JumpToLine moves the cursor to the start of the given 1-indexed line,
// clamping out-of-range values to the first or last line, and scrolls the
// textarea's viewport to keep it visible.
func (e *Editor) JumpToLine(line int) {
	if line < 1 {
		line = 1
	}
	target := line - 1 // 0-indexed row

	switch current := e.textarea.Line(); {
	case target < current:
		for i := 0; i < maxJumpSteps && e.textarea.Line() > target; i++ {
			e.textarea.CursorUp()
		}
	case target > current:
		for i := 0; i < maxJumpSteps && e.textarea.Line() < target; i++ {
			e.textarea.CursorDown()
		}
	}

	e.textarea.CursorStart()

	// textarea's internal viewport only populates its line buffer as a side
	// effect of View() — never from Update(). If View() hasn't run yet since
	// the last SetContent (e.g. jumping to a line right after loading a
	// plan, before the editor has ever been drawn), repositionView below
	// would clamp against a stale/empty buffer and silently fail to scroll.
	// Calling View() here (result discarded) forces that buffer in sync
	// first, in the same call, instead of relying on the app's next render.
	_ = e.textarea.View()
	e.textarea, _ = e.textarea.Update(jumpNoOpMsg{})
}

// LineCount returns the total number of lines in the editor content.
func (e *Editor) LineCount() int {
	return e.textarea.LineCount()
}

// DeleteCurrentLine deletes the entire line where the cursor is positioned,
// content included.
func (e *Editor) DeleteCurrentLine() {
	beforeLines := e.LineCount()
	isLastLine := e.CurrentLine() == beforeLines

	// Go to start of current line, then clear it. ctrl+k (DeleteAfterCursor).
	e.textarea, _ = e.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
	e.textarea, _ = e.textarea.Update(tea.KeyMsg{Type: tea.KeyCtrlK})

	if e.LineCount() == beforeLines {
		// The row is cleared but still present (ctrl+k didn't merge it away
		// on its own, meaning it had content to clear) — merge it into a
		// neighbor so it disappears: pull the next line up, or on the last
		// line, join into the previous one (nothing below to pull up).
		if isLastLine {
			e.textarea, _ = e.textarea.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		} else {
			e.textarea, _ = e.textarea.Update(tea.KeyMsg{Type: tea.KeyDelete})
		}
	}

	// Mark as modified
	e.modified = true
}

// InsertNewLineBelow inserts a new line below the current line and moves cursor to it.
func (e *Editor) InsertNewLineBelow() {
	// Go to end of current line
	endMsg := tea.KeyMsg{Type: tea.KeyEnd}
	e.textarea, _ = e.textarea.Update(endMsg)

	// Insert newline by simulating Enter key
	enterMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'\n'},
	}
	e.textarea, _ = e.textarea.Update(enterMsg)

	// Mark as modified
	e.modified = true
}
