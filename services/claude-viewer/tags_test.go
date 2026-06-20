package claudeviewer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeTags(t *testing.T) {
	t.Run("lowercases input", func(t *testing.T) {
		require.Equal(t, []string{"backend"}, NormalizeTags([]string{"BACKEND"}))
	})

	t.Run("deduplicates", func(t *testing.T) {
		require.Equal(t, []string{"api"}, NormalizeTags([]string{"api", "API", "Api"}))
	})

	t.Run("drops tags over maxTagLength", func(t *testing.T) {
		long := strings.Repeat("a", maxTagLength+1)
		require.Empty(t, NormalizeTags([]string{long}))
	})

	t.Run("allows tag exactly at maxTagLength", func(t *testing.T) {
		exact := strings.Repeat("a", maxTagLength)
		require.Equal(t, []string{exact}, NormalizeTags([]string{exact}))
	})

	t.Run("drops empty strings", func(t *testing.T) {
		require.Empty(t, NormalizeTags([]string{"", "  ", "\t"}))
	})

	t.Run("strips special characters", func(t *testing.T) {
		require.Equal(t, []string{"my-tag", "my_tag"}, NormalizeTags([]string{"my-tag", "my_tag"}))
		require.Equal(t, []string{"mytag"}, NormalizeTags([]string{"my/tag"}))
	})

	t.Run("output is sorted alphabetically", func(t *testing.T) {
		got := NormalizeTags([]string{"zzz", "aaa", "mmm"})
		require.Equal(t, []string{"aaa", "mmm", "zzz"}, got)
	})

	t.Run("output is deterministic across calls", func(t *testing.T) {
		input := []string{"charlie", "alpha", "bravo"}
		first := NormalizeTags(input)
		second := NormalizeTags(input)
		require.Equal(t, first, second)
	})
}
