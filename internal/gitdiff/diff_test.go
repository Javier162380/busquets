package gitdiff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiff(t *testing.T) {
	t.Run("identical content produces no diff", func(t *testing.T) {
		out, err := Diff("same\n", "same\n", "Version 1", "Version 2")
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("changed line produces a unified diff", func(t *testing.T) {
		out, err := Diff("line one\nline two\n", "line one\nline TWO\n", "Version 1", "Version 2")
		require.NoError(t, err)
		require.Equal(t, "--- Version 1\n+++ Version 2\n@@ -1,3 +1,3 @@\n line one\n-line two\n+line TWO\n \n", out)
	})

	t.Run("empty from is a pure addition", func(t *testing.T) {
		out, err := Diff("", "new content\n", "Version 0", "Version 1")
		require.NoError(t, err)
		require.Equal(t, "--- Version 0\n+++ Version 1\n@@ -1 +1,2 @@\n+new content\n \n", out)
	})

	t.Run("single line without trailing newline", func(t *testing.T) {
		out, err := Diff("Content v1.", "Content v2.", "Version 1", "Version 2")
		require.NoError(t, err)
		require.Equal(t, "--- Version 1\n+++ Version 2\n@@ -1 +1 @@\n-Content v1.\n+Content v2.\n", out)
	})
}

func TestStats(t *testing.T) {
	t.Run("identical content counts nothing", func(t *testing.T) {
		added, removed := Stats("same\nlines\n", "same\nlines\n")
		require.Equal(t, 0, added)
		require.Equal(t, 0, removed)
	})

	t.Run("both empty counts nothing", func(t *testing.T) {
		added, removed := Stats("", "")
		require.Equal(t, 0, added)
		require.Equal(t, 0, removed)
	})

	t.Run("empty from is a pure addition", func(t *testing.T) {
		added, removed := Stats("", "one\ntwo\nthree\n")
		require.Equal(t, 3, added)
		require.Equal(t, 0, removed)
	})

	t.Run("empty to is a pure deletion", func(t *testing.T) {
		added, removed := Stats("one\ntwo\n", "")
		require.Equal(t, 0, added)
		require.Equal(t, 2, removed)
	})

	t.Run("appended lines count as additions only", func(t *testing.T) {
		added, removed := Stats("one\ntwo\n", "one\ntwo\nthree\nfour\n")
		require.Equal(t, 2, added)
		require.Equal(t, 0, removed)
	})

	t.Run("replaced line counts on both sides", func(t *testing.T) {
		added, removed := Stats("one\ntwo\nthree\n", "one\nTWO\nthree\n")
		require.Equal(t, 1, added)
		require.Equal(t, 1, removed)
	})

	t.Run("trailing newline is not counted as a line", func(t *testing.T) {
		// "one" and "one\n" are the same single line; only the newline differs.
		added, removed := Stats("one", "one\n")
		require.Equal(t, 0, added)
		require.Equal(t, 0, removed)
	})

	t.Run("content lines starting with plus or minus are counted correctly", func(t *testing.T) {
		// The reason Stats reads op-codes instead of parsing rendered diff text:
		// these are ordinary content lines, not diff markers.
		from := "intro\n+ not a diff marker\n- also not one\n"
		to := "intro\n+ not a diff marker\n- also not one\n+ a genuinely new line\n"
		added, removed := Stats(from, to)
		require.Equal(t, 1, added)
		require.Equal(t, 0, removed)
	})
}
