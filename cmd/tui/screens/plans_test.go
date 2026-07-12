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

func TestPanelWidths(t *testing.T) {
	t.Run("widths sum to total width", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content")
		tagW, listW, viewW := s.panelWidths()
		require.Equal(t, 100, tagW+listW+viewW+4)
	})

	t.Run("works for odd widths", func(t *testing.T) {
		s := NewPlansScreen(99, 40, false, false, false, "plan_content")
		tagW, listW, viewW := s.panelWidths()
		require.Equal(t, 99, tagW+listW+viewW+4)
	})
}

func TestOverlayContent(t *testing.T) {
	s := NewPlansScreen(80, 24, false, false, false, "plan_content")

	t.Run("empty overlay returns base unchanged", func(t *testing.T) {
		result := s.overlayContent("base line 1\nbase line 2", "")
		require.Equal(t, "base line 1\nbase line 2", result)
	})

	t.Run("non-empty overlay lines replace base", func(t *testing.T) {
		result := s.overlayContent("base1\nbase2", "overlay1\noverlay2")
		require.Equal(t, "overlay1\noverlay2", result)
	})

	t.Run("empty overlay lines fall through to base", func(t *testing.T) {
		result := s.overlayContent("base1\nbase2", "overlay1\n")
		require.Equal(t, "overlay1\nbase2", result)
	})
}

func TestGetViewerWidth(t *testing.T) {
	t.Run("fullscreen layout returns width minus padding", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content")
		s.layout = types.LayoutFullscreen
		require.Equal(t, 96, s.getViewerWidth())
	})

	t.Run("split layout returns half width minus padding", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content")
		require.Equal(t, 44, s.getViewerWidth())
	})

	t.Run("three-panel layout uses panelWidths third value", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content")
		_, _, viewW := s.panelWidths()
		require.Equal(t, viewW-4, s.getViewerWidth())
	})
}

func TestUpdateListItems(t *testing.T) {
	t.Run("empty plans produces empty list", func(t *testing.T) {
		s := NewPlansScreen(80, 24, false, false, false, "plan_content")
		s.plans = []claudeviewer.PlanSummary{}
		s.updateListItems()
		require.Equal(t, 0, s.list.ItemCount())
	})

	t.Run("items count matches plans count", func(t *testing.T) {
		s := NewPlansScreen(80, 24, false, false, false, "plan_content")
		s.plans = []claudeviewer.PlanSummary{
			{Title: "Plan A", FileName: "a.md"},
			{Title: "Plan B", FileName: "b.md"},
		}
		s.updateListItems()
		require.Equal(t, 2, s.list.ItemCount())
	})

	t.Run("item titles match plan titles", func(t *testing.T) {
		s := NewPlansScreen(80, 24, false, false, false, "plan_content")
		s.plans = []claudeviewer.PlanSummary{
			{Title: "My Plan", FileName: "plan.md", ModifiedAt: time.Now()},
		}
		s.updateListItems()
		item := s.list.SelectedItem()
		require.NotNil(t, item)
		require.Equal(t, "My Plan", item.Title())
	})
}

func TestRebuildTagPanelEntries(t *testing.T) {
	t.Run("does not panic with populated tags and counts", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content")
		s.allTags = []claudeviewer.Tag{
			{Name: "go"},
			{Name: "api"},
		}
		s.tagPlanCounts = map[string]int{"go": 3, "api": 1}
		s.rebuildTagPanelEntries()
		require.Equal(t, "", s.tagPanel.SelectedTag())
	})

	t.Run("counts from tagPlanCounts are used when set", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content")
		s.allTags = []claudeviewer.Tag{{Name: "backend"}}
		s.tagPlanCounts = map[string]int{"backend": 7}
		s.rebuildTagPanelEntries()
		require.Equal(t, "", s.tagPanel.SelectedTag())
	})

	t.Run("tags missing from counts appear without panic", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content")
		s.allTags = []claudeviewer.Tag{{Name: "orphan"}}
		s.tagPlanCounts = map[string]int{}
		s.rebuildTagPanelEntries()
		require.Equal(t, "", s.tagPanel.SelectedTag())
	})
}

func TestDeleteConfirmDialogVisibleFromList(t *testing.T) {
	s := NewPlansScreen(100, 40, false, false, false, "plan_content")
	s.focus = types.FocusList
	s.plans = []claudeviewer.PlanSummary{{FileName: "p.md", SyncSource: "/src", Title: "My Plan"}}
	s.current = &claudeviewer.PlanDetail{
		PlanSummary: claudeviewer.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "My Plan"},
		FilePath:    "/mirror/p.md",
	}
	s.updateListItems()

	screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	ps, ok := screen.(*PlansScreen)
	require.True(t, ok)
	require.True(t, ps.confirmDialog.IsActive())
	require.Contains(t, ps.View(), "delete the plan")
}

func TestCopyKeyEmitsCopyMsg(t *testing.T) {
	newScreen := func(focus types.Focus) *PlansScreen {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content")
		s.focus = focus
		s.plans = []claudeviewer.PlanSummary{{FileName: "p.md", SyncSource: "/src", Title: "My Plan"}}
		s.current = &claudeviewer.PlanDetail{
			PlanSummary: claudeviewer.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "My Plan"},
			FilePath:    "/mirror/p.md",
		}
		s.updateListItems()
		return s
	}

	for _, tc := range []struct {
		name  string
		focus types.Focus
	}{
		{"from list", types.FocusList},
		{"from content", types.FocusContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newScreen(tc.focus)
			_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
			require.NotNil(t, cmd)
			copyMsg, ok := cmd().(messages.CopyPlanContentMsg)
			require.True(t, ok)
			require.Equal(t, "p.md", copyMsg.FileName)
			require.Equal(t, "/src", copyMsg.SyncSource)
		})
	}
}

func TestCtrlLClearsFiltersFromList(t *testing.T) {
	s := NewPlansScreen(100, 40, false, false, false, "plan_content")
	s.focus = types.FocusList
	s.searchQuery = "needle"
	s.plans = []claudeviewer.PlanSummary{{FileName: "p.md", SyncSource: "/src", Title: "My Plan"}}
	s.updateListItems()

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	require.Empty(t, s.searchQuery)
	require.NotNil(t, cmd)
	_, ok := cmd().(messages.ClearSearchMsg)
	require.True(t, ok)
}
