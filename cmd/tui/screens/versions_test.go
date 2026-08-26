package screens

import (
	"testing"
	"time"

	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/types"
	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestVersionsGetViewerWidth(t *testing.T) {
	t.Run("fullscreen returns width minus padding", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, nil, 100, 40, false, false)
		s.layout = types.LayoutFullscreen
		require.Equal(t, 96, s.getViewerWidth())
	})

	t.Run("split layout returns half width minus padding", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, nil, 100, 40, false, false)
		require.Equal(t, 44, s.getViewerWidth())
	})
}

func TestVersionsUpdateListItems(t *testing.T) {
	t.Run("empty versions produces empty list", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, nil, 80, 24, false, false)
		require.Equal(t, 0, s.list.ItemCount())
	})

	t.Run("item count matches versions count", func(t *testing.T) {
		versions := []busquets.PlanVersionDetail{
			{PlanVersion: busquets.PlanVersion{VersionNumber: 1, CreatedAt: time.Now()}},
			{PlanVersion: busquets.PlanVersion{VersionNumber: 2, CreatedAt: time.Now()}},
		}
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 80, 24, false, false)
		require.Equal(t, 2, s.list.ItemCount())
	})

	t.Run("items display version number in title", func(t *testing.T) {
		versions := []busquets.PlanVersionDetail{
			{PlanVersion: busquets.PlanVersion{VersionNumber: 3, CreatedAt: time.Now()}},
		}
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 80, 24, false, false)
		item := s.list.SelectedItem()
		require.NotNil(t, item)
		require.Contains(t, item.Title(), "3")
	})
}

func TestVersionsIsInputMode(t *testing.T) {
	t.Run("false when search bar is inactive", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, nil, 80, 24, false, false)
		require.False(t, s.IsInputMode())
	})
}

func TestVersionCopyKeyEmitsCopyMsg(t *testing.T) {
	versions := []busquets.PlanVersionDetail{
		{PlanVersion: busquets.PlanVersion{VersionNumber: 2, Content: "# v2\n\nbody", CreatedAt: time.Now()}},
	}

	for _, tc := range []struct {
		name  string
		focus types.Focus
	}{
		{"from list", types.FocusList},
		{"from content", types.FocusContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
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

func TestVersionMarkBaseToggle(t *testing.T) {
	versions := []busquets.PlanVersionDetail{
		{PlanVersion: busquets.PlanVersion{VersionNumber: 1, Content: "one", CreatedAt: time.Now()}},
		{PlanVersion: busquets.PlanVersion{VersionNumber: 2, Content: "two", CreatedAt: time.Now()}},
	}

	t.Run("marks the selected version as base", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)

		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		require.Nil(t, cmd)
		require.NotNil(t, s.baseVersion)
		require.Equal(t, int64(1), s.baseVersion.VersionNumber)
		require.Equal(t, "● Version 1", s.list.SelectedItem().Title())
	})

	t.Run("pressing m again on the same version unmarks it", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		require.Nil(t, cmd)
		require.Nil(t, s.baseVersion)
		require.Equal(t, "Version 1", s.list.SelectedItem().Title())
	})

	t.Run("marking a different version moves the mark", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

		s.list.Select(1)
		s.current = &versions[1]
		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

		require.Nil(t, cmd)
		require.NotNil(t, s.baseVersion)
		require.Equal(t, int64(2), s.baseVersion.VersionNumber)
	})
}

func TestVersionDiffAgainstBase(t *testing.T) {
	versions := []busquets.PlanVersionDetail{
		{PlanVersion: busquets.PlanVersion{VersionNumber: 1, Content: "line one\nline two\n", CreatedAt: time.Now()}},
		{PlanVersion: busquets.PlanVersion{VersionNumber: 2, Content: "line one\nline changed\n", CreatedAt: time.Now()}},
	}

	t.Run("d without a marked base emits an error and stays out of diff view", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)

		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		require.NotNil(t, cmd)
		_, ok := cmd().(messages.ErrorMsg)
		require.True(t, ok)
		require.NotEqual(t, types.FocusDiff, s.focus)
	})

	t.Run("d with base equal to current emits an error and stays out of diff view", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		require.NotNil(t, cmd)
		_, ok := cmd().(messages.ErrorMsg)
		require.True(t, ok)
		require.NotEqual(t, types.FocusDiff, s.focus)
	})

	t.Run("d with a valid distinct base and current builds the expected unified diff", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

		s.list.Select(1)
		s.current = &versions[1]
		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

		require.Nil(t, cmd)
		require.Equal(t, types.FocusDiff, s.focus)
		// go-difflib's SplitLines appends a synthetic trailing empty line
		// whenever content ends in "\n" (as real plan files typically do),
		// which is why the hunk covers 3 lines and ends with a blank one.
		expected := "--- Version 1\n+++ Version 2\n@@ -1,3 +1,3 @@\n line one\n-line two\n+line changed\n \n"
		require.Equal(t, expected, s.diffPlain)
	})

	t.Run("esc from diff view returns to the list with the base still marked", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		s.list.Select(1)
		s.current = &versions[1]
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.Nil(t, cmd)
		require.Equal(t, types.FocusList, s.focus)
		require.NotNil(t, s.baseVersion)
		require.Equal(t, int64(1), s.baseVersion.VersionNumber)
	})

	t.Run("c from diff view copies the plain diff to clipboard", func(t *testing.T) {
		s := NewVersionsScreenWithData(1, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		s.list.Select(1)
		s.current = &versions[1]
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
		require.NotNil(t, cmd)
		msg, ok := cmd().(messages.CopyToClipboardMsg)
		require.True(t, ok)
		require.Equal(t, s.diffPlain, msg.Text)
		require.Equal(t, "Diff v1→v2", msg.Label)
	})
}

func TestVersionRestoreKeyEmitsRestoreMsgWithPlanID(t *testing.T) {
	versions := []busquets.PlanVersionDetail{
		{PlanVersion: busquets.PlanVersion{VersionNumber: 2, Content: "# v2\n\nbody", CreatedAt: time.Now()}},
	}

	for _, tc := range []struct {
		name  string
		focus types.Focus
	}{
		{"from list", types.FocusList},
		{"from content", types.FocusContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewVersionsScreenWithData(42, "plan.md", busquets.MarkdownThemeASCII, versions, 100, 40, false, false)
			s.focus = tc.focus

			_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
			require.NotNil(t, cmd)
			msg, ok := cmd().(messages.RestoreVersionMsg)
			require.True(t, ok)
			require.Equal(t, int64(42), msg.PlanID)
			require.Equal(t, "plan.md", msg.PlanName)
			require.Equal(t, int64(2), msg.VersionNumber)
		})
	}
}
