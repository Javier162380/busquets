package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchBar(t *testing.T) {
	t.Run("Value returns empty string after construction", func(t *testing.T) {
		sb := NewSearchBar(80)
		require.Equal(t, "", sb.Value())
	})

	t.Run("SetValue stores the value", func(t *testing.T) {
		sb := NewSearchBar(80)
		sb.SetValue("hello")
		require.Equal(t, "hello", sb.Value())
	})

	t.Run("Reset clears the value", func(t *testing.T) {
		sb := NewSearchBar(80)
		sb.SetValue("hello")
		sb.Reset()
		require.Equal(t, "", sb.Value())
	})

	t.Run("IsActive is false before Focus", func(t *testing.T) {
		sb := NewSearchBar(80)
		require.False(t, sb.IsActive())
	})

	t.Run("IsActive is true after Focus", func(t *testing.T) {
		sb := NewSearchBar(80)
		_ = sb.Focus()
		require.True(t, sb.IsActive())
	})

	t.Run("IsActive is false after Blur", func(t *testing.T) {
		sb := NewSearchBar(80)
		_ = sb.Focus()
		sb.Blur()
		require.False(t, sb.IsActive())
	})
}
