package screens

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOverlayContent covers the package-level overlayContent helper shared
// by PlansScreen and VersionsScreen — moved here (out of plans_test.go) when
// overlayContent stopped being a PlansScreen method, since it no longer
// needs a screen instance to test.
func TestOverlayContent(t *testing.T) {
	t.Run("empty overlay returns base unchanged", func(t *testing.T) {
		result := overlayContent("base line 1\nbase line 2", "")
		require.Equal(t, "base line 1\nbase line 2", result)
	})

	t.Run("non-empty overlay lines replace base", func(t *testing.T) {
		result := overlayContent("base1\nbase2", "overlay1\noverlay2")
		require.Equal(t, "overlay1\noverlay2", result)
	})

	t.Run("empty overlay lines fall through to base", func(t *testing.T) {
		result := overlayContent("base1\nbase2", "overlay1\n")
		require.Equal(t, "overlay1\nbase2", result)
	})
}
