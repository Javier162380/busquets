package screens

import (
	"strings"
	"testing"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"

	"github.com/stretchr/testify/require"
)

// TestHelpContentTracksClipboardKeys guards the AGENTS.md requirement that the
// help screen and the status-bar ShortHelp stay in sync with the keybindings.
func TestHelpContentTracksClipboardKeys(t *testing.T) {
	t.Run("help overlay documents copy and the relocated clear-filters key", func(t *testing.T) {
		require.Contains(t, helpContent, "Copy plan content to clipboard")
		require.Contains(t, helpContent, "Ctrl+L")
		// The old bare-c clear-filters line must be gone.
		require.NotContains(t, helpContent, "  c              Clear active search")
	})

	t.Run("ShortHelp lists copy in both focuses and ctrl+l on the list", func(t *testing.T) {
		s := NewPlansScreen(120, 40, false, false, false, "plan_content")

		s.focus = types.FocusList
		s.searchQuery = "needle" // makes the clear-filters hint visible
		list := s.ShortHelp()
		require.Contains(t, list, "c: copy")
		require.Contains(t, list, "ctrl+l: clear")
		require.False(t, strings.Contains(list, "| c: clear"))

		s.focus = types.FocusContent
		require.Contains(t, s.ShortHelp(), "c: copy")
	})
}
