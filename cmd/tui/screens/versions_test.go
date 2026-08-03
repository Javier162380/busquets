package screens

import (
	"testing"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestVersionsGetViewerWidth(t *testing.T) {
	t.Run("fullscreen returns width minus padding", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", claudeviewer.MarkdownThemeASCII, nil, 100, 40, false, false)
		s.layout = types.LayoutFullscreen
		require.Equal(t, 96, s.getViewerWidth())
	})

	t.Run("split layout returns half width minus padding", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", claudeviewer.MarkdownThemeASCII, nil, 100, 40, false, false)
		require.Equal(t, 44, s.getViewerWidth())
	})
}

func TestVersionsUpdateListItems(t *testing.T) {
	t.Run("empty versions produces empty list", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", claudeviewer.MarkdownThemeASCII, nil, 80, 24, false, false)
		require.Equal(t, 0, s.list.ItemCount())
	})

	t.Run("item count matches versions count", func(t *testing.T) {
		versions := []claudeviewer.PlanVersionDetail{
			{PlanVersion: claudeviewer.PlanVersion{VersionNumber: 1, CreatedAt: time.Now()}},
			{PlanVersion: claudeviewer.PlanVersion{VersionNumber: 2, CreatedAt: time.Now()}},
		}
		s := NewVersionsScreenWithData("plan.md", claudeviewer.MarkdownThemeASCII, versions, 80, 24, false, false)
		require.Equal(t, 2, s.list.ItemCount())
	})

	t.Run("items display version number in title", func(t *testing.T) {
		versions := []claudeviewer.PlanVersionDetail{
			{PlanVersion: claudeviewer.PlanVersion{VersionNumber: 3, CreatedAt: time.Now()}},
		}
		s := NewVersionsScreenWithData("plan.md", claudeviewer.MarkdownThemeASCII, versions, 80, 24, false, false)
		item := s.list.SelectedItem()
		require.NotNil(t, item)
		require.Contains(t, item.Title(), "3")
	})
}

func TestVersionsIsInputMode(t *testing.T) {
	t.Run("false when search bar is inactive", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", claudeviewer.MarkdownThemeASCII, nil, 80, 24, false, false)
		require.False(t, s.IsInputMode())
	})
}

func TestVersionCopyKeyEmitsCopyMsg(t *testing.T) {
	versions := []claudeviewer.PlanVersionDetail{
		{PlanVersion: claudeviewer.PlanVersion{VersionNumber: 2, Content: "# v2\n\nbody", CreatedAt: time.Now()}},
	}

	for _, tc := range []struct {
		name  string
		focus types.Focus
	}{
		{"from list", types.FocusList},
		{"from content", types.FocusContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewVersionsScreenWithData("plan.md", claudeviewer.MarkdownThemeASCII, versions, 100, 40, false, false)
			s.focus = tc.focus

			_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
			require.NotNil(t, cmd)
			msg, ok := cmd().(messages.CopyToClipboardMsg)
			require.True(t, ok)
			require.Equal(t, "Version 2", msg.Label)
			require.Equal(t, "# v2\n\nbody", msg.Text)
		})
	}
}
