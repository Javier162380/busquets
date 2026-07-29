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

func newLabelModeScreenWithPlans() *PlansScreen {
	s := NewPlansScreen(120, 40, false, false, false, claudeviewer.DisplayModeLabelPlanContent)
	s.allPlans = []claudeviewer.PlanSummary{
		{FileName: "a.md", SyncSource: "/a", SyncLabel: "work", Title: "A"},
		{FileName: "b.md", SyncSource: "/a", SyncLabel: "work", Title: "B"},
		{FileName: "c.md", SyncSource: "/b", SyncLabel: "personal", Title: "C"},
	}
	s.plans = s.allPlans
	s.rebuildLabelPanelEntries()
	return s
}

func TestLabelPanelEntriesAndFilter(t *testing.T) {
	s := newLabelModeScreenWithPlans()

	t.Run("label mode mounts the label panel, not the tag panel", func(t *testing.T) {
		require.NotNil(t, s.labelPanel)
		require.Nil(t, s.tagPanel)
		require.Equal(t, types.FocusLabelPanel, s.focus)
		require.Equal(t, types.LayoutThreePanel, s.layout)
	})

	t.Run("panel lists labels with counts, sorted, under an All header", func(t *testing.T) {
		view := s.labelPanel.View()
		require.Contains(t, view, "All")
		require.Contains(t, view, "personal")
		require.Contains(t, view, "work")
	})

	t.Run("selecting a label filters plans to that label only", func(t *testing.T) {
		cmd := s.applyLabelFilter("work")
		require.NotNil(t, cmd)
		require.Len(t, s.plans, 2)
		for _, p := range s.plans {
			require.Equal(t, "work", p.SyncLabel)
		}
	})

	t.Run("selecting All restores the full plan list", func(t *testing.T) {
		s.applyLabelFilter("")
		require.Len(t, s.plans, 3)
	})
}

func TestLabelFilterPersistsAcrossUnfilteredReload(t *testing.T) {
	s := newLabelModeScreenWithPlans()

	// Navigate to "work", mirroring the down-key path in handleLabelPanelKey.
	require.Equal(t, "personal", s.labelPanel.MoveDown())
	require.Equal(t, "work", s.labelPanel.MoveDown())

	cmd := s.applyLabelFilter(s.labelPanel.SelectedLabel())
	require.NotNil(t, cmd)
	require.Len(t, s.plans, 2)

	// Simulate the unfiltered reload a mutating command (rename/delete/sync/save)
	// sends via LoadPlansCmd — this used to silently drop the active label filter.
	screen, _ := s.Update(messages.PlansLoadedMsg{Plans: s.allPlans, IsFiltered: false})
	ps := screen.(*PlansScreen)

	require.Equal(t, "work", ps.labelPanel.SelectedLabel())
	require.Len(t, ps.plans, 2)
	for _, p := range ps.plans {
		require.Equal(t, "work", p.SyncLabel)
	}
}

func TestLabelModeIgnoresTagPanelMessages(t *testing.T) {
	s := newLabelModeScreenWithPlans()

	require.Equal(t, "personal", s.labelPanel.MoveDown())
	require.Equal(t, "work", s.labelPanel.MoveDown())
	require.NotNil(t, s.applyLabelFilter(s.labelPanel.SelectedLabel()))
	require.Len(t, s.plans, 2)

	// AllTagsForPanelLoadedMsg is tag-panel data. Label mode never requests it and
	// must not be mutated by it if one arrives anyway.
	screen, cmd := s.Update(messages.AllTagsForPanelLoadedMsg{
		Tags:       []claudeviewer.Tag{{Name: "go"}, {Name: "api"}},
		Counts:     map[string]int{"go": 1},
		TagPlanMap: map[string][]claudeviewer.PlanSummary{"go": {{FileName: "a.md"}}},
		AllPlans:   []claudeviewer.PlanSummary{{FileName: "a.md", SyncLabel: "work"}},
	})
	ps := screen.(*PlansScreen)

	require.Nil(t, cmd)
	require.Nil(t, ps.tagPanel)
	require.Nil(t, ps.allTags)
	require.Equal(t, "work", ps.labelPanel.SelectedLabel())
	require.Len(t, ps.plans, 2)
	require.Len(t, ps.allPlans, 3)
}

func TestLabelPanelCreateKeyIsNoop(t *testing.T) {
	s := newLabelModeScreenWithPlans()

	screen, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	ps := screen.(*PlansScreen)

	require.Nil(t, cmd)
	require.Equal(t, types.FocusLabelPanel, ps.focus)
}

func TestSidePanelFocusCycle(t *testing.T) {
	for _, tc := range []struct {
		mode  string
		focus types.Focus
	}{
		{claudeviewer.DisplayModeTagPlanContent, types.FocusTagPanel},
		{claudeviewer.DisplayModeLabelPlanContent, types.FocusLabelPanel},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Run("shift+tab from the list reaches the side panel", func(t *testing.T) {
				s := NewPlansScreen(120, 40, false, false, false, tc.mode)
				s.focus = types.FocusList

				screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
				require.Equal(t, tc.focus, screen.(*PlansScreen).focus)
			})

			t.Run("tab from content reaches the side panel", func(t *testing.T) {
				s := NewPlansScreen(120, 40, false, false, false, tc.mode)
				s.focus = types.FocusContent

				screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyTab})
				require.Equal(t, tc.focus, screen.(*PlansScreen).focus)
			})
		})
	}

	t.Run("plan_content has no side panel to reach", func(t *testing.T) {
		s := NewPlansScreen(120, 40, false, false, false, claudeviewer.DisplayModePlanContent)
		s.focus = types.FocusList

		screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		require.Equal(t, types.FocusList, screen.(*PlansScreen).focus)
	})
}

func TestSetDisplayModeMountsOnePanel(t *testing.T) {
	s := NewPlansScreen(120, 40, false, false, false, claudeviewer.DisplayModePlanContent)

	s.SetDisplayMode(claudeviewer.DisplayModeTagPlanContent)
	require.NotNil(t, s.tagPanel)
	require.Nil(t, s.labelPanel)
	require.Equal(t, types.FocusTagPanel, s.focus)
	require.Equal(t, types.LayoutThreePanel, s.layout)

	s.SetDisplayMode(claudeviewer.DisplayModeLabelPlanContent)
	require.Nil(t, s.tagPanel)
	require.NotNil(t, s.labelPanel)
	require.Equal(t, types.FocusLabelPanel, s.focus)
	require.Equal(t, types.LayoutThreePanel, s.layout)

	s.SetDisplayMode(claudeviewer.DisplayModePlanContent)
	require.Nil(t, s.tagPanel)
	require.Nil(t, s.labelPanel)
	require.Equal(t, types.FocusList, s.focus)
	require.Equal(t, types.LayoutSplit, s.layout)
}

func TestLeavingFullscreenRestoresThreePanelLayout(t *testing.T) {
	for _, mode := range []string{
		claudeviewer.DisplayModeTagPlanContent,
		claudeviewer.DisplayModeLabelPlanContent,
	} {
		t.Run(mode, func(t *testing.T) {
			s := NewPlansScreen(120, 40, false, false, false, mode)
			s.focus = types.FocusList
			s.current = &claudeviewer.PlanDetail{
				PlanSummary: claudeviewer.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "P"},
			}

			screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
			ps := screen.(*PlansScreen)
			require.Equal(t, types.LayoutFullscreen, ps.layout)

			screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyEsc})
			ps = screen.(*PlansScreen)
			require.Equal(t, types.LayoutThreePanel, ps.layout)
		})
	}
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
			Content:     "# My Plan\n\nbody",
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
			copyMsg, ok := cmd().(messages.CopyToClipboardMsg)
			require.True(t, ok)
			require.Equal(t, "p.md", copyMsg.Label)
			require.Equal(t, "# My Plan\n\nbody", copyMsg.Text)
		})
	}
}

func TestTagFilterVisibleInBothDisplayModes(t *testing.T) {
	modes := []string{
		claudeviewer.DisplayModePlanContent,
		claudeviewer.DisplayModeTagPlanContent,
		claudeviewer.DisplayModeLabelPlanContent,
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			s := NewPlansScreen(120, 40, false, false, false, mode)
			s.focus = types.FocusList

			screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
			ps := screen.(*PlansScreen)

			require.True(t, ps.tagFilter.IsActive())
			require.Equal(t, types.FocusTagFilter, ps.focus)
			require.Contains(t, ps.View(), "[OR]")
		})
	}
}

func TestSearchBarVisibleInBothDisplayModes(t *testing.T) {
	modes := []string{
		claudeviewer.DisplayModePlanContent,
		claudeviewer.DisplayModeTagPlanContent,
		claudeviewer.DisplayModeLabelPlanContent,
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			s := NewPlansScreen(120, 40, false, false, false, mode)
			s.focus = types.FocusList

			screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
			ps := screen.(*PlansScreen)

			require.True(t, ps.searchBar.IsActive())
			require.Equal(t, types.FocusSearch, ps.focus)
			require.Contains(t, ps.View(), "Type to")
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
