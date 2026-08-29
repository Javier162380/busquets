package screens

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

// TestOverlayContent covers the package-level overlayContent helper shared
// by PlansScreen and VersionsScreen — moved here (out of plans_test.go) when
// overlayContent stopped being a PlansScreen method, since it no longer
// needs a screen instance to test.
func TestOverlayContent(t *testing.T) {
	t.Run("empty overlay returns base unchanged", func(t *testing.T) {
		result := overlayContent("base line 1\nbase line 2", 11, 2, lipgloss.Center, lipgloss.Center, "")
		require.Equal(t, "base line 1\nbase line 2", result)
	})

	t.Run("overlay as wide as canvas replaces base line entirely", func(t *testing.T) {
		result := overlayContent("base1\nbase2", 8, 2, lipgloss.Center, lipgloss.Center, "overlay1\noverlay2")
		require.Equal(t, "overlay1\noverlay2", result)
	})

	t.Run("empty overlay line falls through to base", func(t *testing.T) {
		result := overlayContent("base1\nbase2", 8, 2, lipgloss.Left, lipgloss.Top, "overlay1\n")
		require.Equal(t, "overlay1\nbase2", result)
	})

	t.Run("narrow centered overlay only replaces its own columns", func(t *testing.T) {
		result := overlayContent("XXXXXXXXXX", 10, 1, lipgloss.Center, lipgloss.Center, "OO")
		require.Equal(t, "XXXXOOXXXX", result)
	})

	t.Run("left-aligned overlay leaves the right side of the base untouched", func(t *testing.T) {
		result := overlayContent("XXXXXXXXXX", 10, 1, lipgloss.Left, lipgloss.Top, "OO")
		require.Equal(t, "OOXXXXXXXX", result)
	})

	t.Run("right-aligned overlay leaves the left side of the base untouched", func(t *testing.T) {
		result := overlayContent("XXXXXXXXXX", 10, 1, lipgloss.Right, lipgloss.Top, "OO")
		require.Equal(t, "XXXXXXXXOO", result)
	})

	t.Run("vertically centered overlay leaves rows above and below untouched", func(t *testing.T) {
		base := "row0\nrow1\nrow2\nrow3"
		result := overlayContent(base, 4, 4, lipgloss.Center, lipgloss.Center, "OO")
		require.Equal(t, "row0\nrOO1\nrow2\nrow3", result)
	})
}
