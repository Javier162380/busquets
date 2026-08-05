package components

import (
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
}
