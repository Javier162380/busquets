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
