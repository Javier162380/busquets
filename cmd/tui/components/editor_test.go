package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditorJumpToLine(t *testing.T) {
	content := strings.Join([]string{
		"line 1", "line 2", "line 3", "line 4", "line 5",
		"line 6", "line 7", "line 8", "line 9", "line 10",
	}, "\n")

	newEditorAt := func(t *testing.T, startLine int) *Editor {
		t.Helper()
		e := NewEditor(80, 10)
		e.SetContent(content)
		e.Focus() // matches the real FocusAtLine call path; JumpToLine's viewport reposition is a no-op while unfocused
		e.JumpToLine(startLine)
		require.Equal(t, startLine-1, e.textarea.Line())
		return e
	}

	t.Run("jumps forward from the current line", func(t *testing.T) {
		e := newEditorAt(t, 2)
		e.JumpToLine(7)
		require.Equal(t, 6, e.textarea.Line())
	})

	t.Run("jumps backward from the current line", func(t *testing.T) {
		e := newEditorAt(t, 8)
		e.JumpToLine(3)
		require.Equal(t, 2, e.textarea.Line())
	})

	t.Run("jumps to the first line", func(t *testing.T) {
		e := newEditorAt(t, 5)
		e.JumpToLine(1)
		require.Equal(t, 0, e.textarea.Line())
	})

	t.Run("clamps a target past the last line", func(t *testing.T) {
		e := newEditorAt(t, 1)
		e.JumpToLine(9999)
		require.Equal(t, e.LineCount()-1, e.textarea.Line())
	})

	t.Run("clamps a target below the first line", func(t *testing.T) {
		e := newEditorAt(t, 5)
		e.JumpToLine(0)
		require.Equal(t, 0, e.textarea.Line())
	})

	t.Run("CurrentLine round-trips with JumpToLine", func(t *testing.T) {
		e := newEditorAt(t, 1)
		e.JumpToLine(6)
		require.Equal(t, 6, e.CurrentLine())
	})
}

// TestEditorJumpToLineScrollsViewport guards against a regression where the
// cursor's logical row (Line()) moved correctly but the textarea's own
// rendered viewport never scrolled to show it — because bubbles' textarea
// only populates its internal viewport line buffer as a side effect of
// View(), never Update(), so a stale/empty buffer silently swallowed the
// scroll. Line()-only assertions (like the rest of this file) can't catch
// that; this checks the actual rendered output.
func TestEditorJumpToLineScrollsViewport(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("content of line %02d", i+1)
	}
	body := strings.Join(lines, "\n")

	e := NewEditor(40, 6) // viewport much shorter than the content
	e.SetContent(body)
	e.FocusAtLine(30)

	want := "┃  25 content of line 25                \n" +
		"┃  26 content of line 26                \n" +
		"┃  27 content of line 27                \n" +
		"┃  28 content of line 28                \n" +
		"┃  29 content of line 29                \n" +
		"┃  30 content of line 30                "
	require.Equal(t, want, e.View())
}

// TestEditorDeleteCurrentLine guards against a regression where a single
// KeyDelete from Home only removed the line break — which fully deletes an
// already-empty line (nothing else to remove) but, on a line with content,
// only ate the first character and left the rest of the line behind merged
// into the next one.
func TestEditorDeleteCurrentLine(t *testing.T) {
	newEditorAt := func(t *testing.T, content string, line int) *Editor {
		t.Helper()
		e := NewEditor(80, 10)
		e.SetContent(content)
		e.Focus()
		e.JumpToLine(line)
		return e
	}

	t.Run("deletes a non-empty middle line, content included", func(t *testing.T) {
		e := newEditorAt(t, "line 1\nline 2\nline 3", 2)
		e.DeleteCurrentLine()
		require.Equal(t, "line 1\nline 3", e.Content())
	})

	t.Run("deletes an already-empty line", func(t *testing.T) {
		e := newEditorAt(t, "line 1\n\nline 3", 2)
		e.DeleteCurrentLine()
		require.Equal(t, "line 1\nline 3", e.Content())
	})

	t.Run("deletes the last line by merging into the previous one", func(t *testing.T) {
		e := newEditorAt(t, "line 1\nline 2", 2)
		e.DeleteCurrentLine()
		require.Equal(t, "line 1", e.Content())
	})

	t.Run("deletes the only line, leaving the document empty", func(t *testing.T) {
		e := newEditorAt(t, "only line", 1)
		e.DeleteCurrentLine()
		require.Equal(t, "", e.Content())
	})
}

func TestEditorInsertNewLineBelow(t *testing.T) {
	newEditorAt := func(t *testing.T, content string, line int) *Editor {
		t.Helper()
		e := NewEditor(80, 10)
		e.SetContent(content)
		e.Focus()
		e.JumpToLine(line)
		return e
	}

	t.Run("inserts a blank line below a non-empty line", func(t *testing.T) {
		e := newEditorAt(t, "line 1\nline 2", 1)
		e.InsertNewLineBelow()
		require.Equal(t, "line 1\n\nline 2", e.Content())
	})

	t.Run("inserts a blank line below an already-blank line", func(t *testing.T) {
		e := newEditorAt(t, "line 1\n\nline 3", 2)
		e.InsertNewLineBelow()
		require.Equal(t, "line 1\n\n\nline 3", e.Content())
	})

	t.Run("inserts a blank line below the last line", func(t *testing.T) {
		e := newEditorAt(t, "only line", 1)
		e.InsertNewLineBelow()
		require.Equal(t, "only line\n", e.Content())
	})
}

func TestEditorMarkSaved(t *testing.T) {
	t.Run("does not touch the current textarea value", func(t *testing.T) {
		e := NewEditor(80, 10)
		e.SetContent("v1")
		e.textarea.SetValue("v1 plus more typing")

		e.MarkSaved("v1")

		require.Equal(t, "v1 plus more typing", e.Content())
	})

	t.Run("clears modified when the save matches the current value", func(t *testing.T) {
		e := NewEditor(80, 10)
		e.SetContent("v1")
		e.textarea.SetValue("v2")
		e.modified = true // what Editor.Update would have set after a real keystroke
		require.True(t, e.IsModified())

		e.MarkSaved("v2")

		require.False(t, e.IsModified())
	})

	t.Run("keeps modified true when typing continued past the saved snapshot", func(t *testing.T) {
		e := NewEditor(80, 10)
		e.SetContent("v1")
		e.textarea.SetValue("v2 plus more")

		e.MarkSaved("v2") // the save only captured "v2", user kept typing after

		require.True(t, e.IsModified())
	})

	t.Run("Reset after MarkSaved discards only edits made since the save, not the whole save", func(t *testing.T) {
		e := NewEditor(80, 10)
		e.SetContent("v1")        // pre-save baseline
		e.textarea.SetValue("v2") // saved value
		e.MarkSaved("v2")
		e.textarea.SetValue("v2 plus unsaved edit")

		e.Reset()

		require.Equal(t, "v2", e.Content())
		require.False(t, e.IsModified())
	})
}

func TestEditorStash(t *testing.T) {
	t.Run("RestoreStash brings back the stashed edit and reports modified", func(t *testing.T) {
		e := NewEditor(80, 10)
		e.SetContent("original")
		e.textarea.SetValue("original plus edit")

		e.Stash()
		e.Reset() // what the esc handler does before restoring the stash
		require.Equal(t, "original", e.Content())
		require.False(t, e.IsModified())

		e.RestoreStash()

		require.Equal(t, "original plus edit", e.Content())
		require.True(t, e.IsModified())
	})

	t.Run("ClearStash makes RestoreStash a no-op", func(t *testing.T) {
		e := NewEditor(80, 10)
		e.SetContent("original")
		e.textarea.SetValue("original plus edit")
		e.Stash()

		e.ClearStash()
		e.Reset()
		e.RestoreStash()

		require.Equal(t, "original", e.Content())
		require.False(t, e.IsModified())
	})

	t.Run("RestoreStash without a prior Stash is a no-op", func(t *testing.T) {
		e := NewEditor(80, 10)
		e.SetContent("original")

		e.RestoreStash()

		require.Equal(t, "original", e.Content())
		require.False(t, e.IsModified())
	})
}
