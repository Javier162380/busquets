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
