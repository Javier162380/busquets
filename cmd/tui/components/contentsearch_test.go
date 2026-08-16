package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentSearch(t *testing.T) {
	t.Run("empty query has no matches", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("line one\nline two\nline three")
		require.False(t, c.HasMatches())
		require.Equal(t, 0, c.MatchCount())
		require.Equal(t, 0, c.CurrentLine())
		require.Equal(t, 0, c.CurrentIndex())
	})

	t.Run("query with no matches", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("line one\nline two\nline three")
		c.SetQuery("nonexistent", 1)
		require.False(t, c.HasMatches())
		require.Equal(t, 0, c.MatchCount())
	})

	t.Run("case-insensitive matching", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("TODO: fix this\nanother todo here")
		c.SetQuery("todo", 1)
		require.True(t, c.HasMatches())
		require.Equal(t, 2, c.MatchCount())
	})

	t.Run("single match on first line", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("hello world\nsecond line\nthird line")
		c.SetQuery("world", 1)
		require.True(t, c.HasMatches())
		require.Equal(t, 1, c.MatchCount())
		require.Equal(t, 1, c.CurrentLine())
		require.Equal(t, 1, c.CurrentIndex())
	})

	t.Run("match on last line", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("first line\nsecond line\nfind me here")
		c.SetQuery("find me", 1)
		require.True(t, c.HasMatches())
		require.Equal(t, 3, c.CurrentLine())
	})

	t.Run("multiple matches on one line", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("cat sat on the cat mat with a cat")
		c.SetQuery("cat", 1)
		require.Equal(t, 3, c.MatchCount())
		ranges := c.MatchesOnLine(1)
		require.Len(t, ranges, 3)
	})

	t.Run("match spanning right up to a line boundary", func(t *testing.T) {
		// "end" ends exactly at the newline that starts line 2.
		c := NewContentSearch()
		c.SetContent("line one end\nline two")
		c.SetQuery("end", 1)
		require.True(t, c.HasMatches())
		require.Equal(t, 1, c.CurrentLine())
		ranges := c.MatchesOnLine(1)
		require.Len(t, ranges, 1)
		require.Equal(t, [2]int{9, 12}, ranges[0])
	})

	t.Run("SetQuery selects first match at or after fromLine", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("match one\nno match\nmatch two\nmatch three")
		c.SetQuery("match", 3)
		require.Equal(t, 3, c.CurrentLine())
	})

	t.Run("SetQuery wraps to top when no match at or after fromLine", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("match one\nno match here\nnothing")
		c.SetQuery("match", 3)
		require.Equal(t, 1, c.CurrentLine())
	})

	t.Run("Next and Prev wrap around at both ends", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("alpha\nalpha\nalpha")
		c.SetQuery("alpha", 1)
		require.Equal(t, 3, c.MatchCount())
		require.Equal(t, 1, c.CurrentLine())

		require.Equal(t, 2, c.Next())
		require.Equal(t, 3, c.Next())
		require.Equal(t, 1, c.Next(), "Next should wrap from the last match back to the first")

		require.Equal(t, 3, c.Prev(), "Prev should wrap from the first match back to the last")
		require.Equal(t, 2, c.Prev())
		require.Equal(t, 1, c.Prev())
	})

	t.Run("Next and Prev on empty matches are no-ops", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("nothing here")
		c.SetQuery("missing", 1)
		require.Equal(t, 0, c.Next())
		require.Equal(t, 0, c.Prev())
	})

	t.Run("SetContent discards previous query's matches rather than mixing offsets", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("short content with target")
		c.SetQuery("target", 1)
		require.Equal(t, 1, c.MatchCount())
		require.Equal(t, 1, c.CurrentLine())

		// Switching to shorter content that still contains the query re-applies
		// it cleanly rather than reusing stale byte offsets from the old content.
		c.SetContent("target")
		require.True(t, c.HasMatches())
		require.Equal(t, 1, c.MatchCount())
		require.Equal(t, 1, c.CurrentLine())

		// Switching to content with no match at all clears matches entirely.
		c.SetContent("nothing relevant")
		require.False(t, c.HasMatches())
	})

	t.Run("MatchesOnLine returns nothing for a line with no matches", func(t *testing.T) {
		c := NewContentSearch()
		c.SetContent("line one\nline two\nline three")
		c.SetQuery("one", 1)
		require.Empty(t, c.MatchesOnLine(2))
		require.Empty(t, c.MatchesOnLine(3))
	})

	t.Run("lineStart returns end of content past the last valid line", func(t *testing.T) {
		// Regression guard for the off-by-one this plan explicitly reasoned
		// through: line == len(lineOffsets) is the last real line, not
		// out-of-range — only line > len(lineOffsets) should fall back to
		// len(raw).
		c := NewContentSearch()
		c.SetContent("ab\ncd")
		c.SetQuery("cd", 1)
		require.Equal(t, 2, c.CurrentLine(), "match on the last line must resolve to line 2, not fall back past it")
	})
}
