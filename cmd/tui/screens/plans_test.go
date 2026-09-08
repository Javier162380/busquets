package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/types"
	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestPanelWidths(t *testing.T) {
	t.Run("widths sum to total width", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		tagW, listW, viewW := s.panelWidths()
		require.Equal(t, 100, tagW+listW+viewW+4)
	})

	t.Run("works for odd widths", func(t *testing.T) {
		s := NewPlansScreen(99, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		tagW, listW, viewW := s.panelWidths()
		require.Equal(t, 99, tagW+listW+viewW+4)
	})
}

func TestGetViewerWidth(t *testing.T) {
	t.Run("fullscreen layout returns width minus padding", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.layout = types.LayoutFullscreen
		require.Equal(t, 96, s.getViewerWidth())
	})

	t.Run("split layout returns half width minus padding", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		require.Equal(t, 44, s.getViewerWidth())
	})

	t.Run("three-panel layout uses panelWidths third value", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		_, _, viewW := s.panelWidths()
		require.Equal(t, viewW-4, s.getViewerWidth())
	})

	t.Run("vertical split layout returns full width minus padding", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationVertical)
		require.Equal(t, 94, s.getViewerWidth())
	})

	t.Run("vertical three-panel layout returns full width minus padding", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationVertical)
		require.Equal(t, 94, s.getViewerWidth())
	})
}

// TestRenderOrientation asserts that switching orientation actually changes
// the rendered layout's shape (dimensions, not styled content, which would be
// brittle) — vertical mode should render narrower-or-equal and taller than
// horizontal mode, for both the two-panel and three-panel layouts. Exact
// figures were captured by running the renderers, not hand-derived.
func TestRenderOrientation(t *testing.T) {
	t.Run("two-panel split view", func(t *testing.T) {
		h := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		v := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationVertical)

		hLines := strings.Split(h.renderSplitView(), "\n")
		vLines := strings.Split(v.renderSplitView(), "\n")

		require.Equal(t, 38, len(hLines))
		require.Equal(t, 101, lipgloss.Width(hLines[0]))

		// 38, matching horizontal — not 40. An earlier version of this budget
		// rendered vertical mode 2 rows taller than horizontal, which overflowed
		// the terminal by 1 row once App appends its status-bar line: see
		// TestRenderVerticalNeverOverflowsHeight below.
		require.Equal(t, 38, len(vLines))
		require.Equal(t, 100, lipgloss.Width(vLines[0]))
	})

	t.Run("three-panel view", func(t *testing.T) {
		h := NewPlansScreen(100, 40, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		v := NewPlansScreen(100, 40, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationVertical)

		hLines := strings.Split(h.renderThreePanelView(), "\n")
		vLines := strings.Split(v.renderThreePanelView(), "\n")

		// Horizontal's 102 (2 over the 100-wide terminal) is pre-existing,
		// unchanged behavior in this codebase's original three-panel layout —
		// not introduced or fixed here.
		require.Equal(t, 38, len(hLines))
		require.Equal(t, 102, lipgloss.Width(hLines[0]))

		// Vertical mode renders exactly the terminal's width (unlike
		// horizontal's pre-existing 2-over) — an actual overflow bug here was
		// caught and fixed via this exact assertion during development: the
		// side-panel+list top row was 2 columns too wide, which wraps in a
		// real terminal and corrupts everything below it.
		require.Equal(t, 38, len(vLines))
		require.Equal(t, 100, lipgloss.Width(vLines[0]))
	})
}

// TestRenderThreePanelViewVerticalNeverOverflowsWidth guards against
// regressing the exact bug fixed above: vertical mode's rendered width must
// never exceed the terminal's actual width, at any size — a wider-than-terminal
// line wraps in a real terminal and corrupts the layout below it, which is how
// this was originally reported ("top border of the panel is never
// highlighted" — the wrapped overflow, not a missing style).
func TestRenderThreePanelViewVerticalNeverOverflowsWidth(t *testing.T) {
	for _, w := range []int{60, 80, 100, 120} {
		s := NewPlansScreen(w, 30, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationVertical)
		lines := strings.Split(s.renderThreePanelViewVertical(), "\n")
		require.Equal(t, w, lipgloss.Width(lines[0]), "width=%d", w)
	}
}

// TestRenderVerticalNeverOverflowsHeight guards against a second instance of
// the same class of bug: App.View() appends one more line below whatever a
// screen renders (the status bar), so a screen's own rendered height must
// leave at least 1 row of slack under the terminal's actual height — every
// other render function in this codebase already does (contentHeight :=
// s.height-4 -> box height h-2 -> +1 status line -> h-1). An earlier version
// of the vertical-orientation height budget didn't leave that slack, so
// stacking two bordered panels rendered exactly h screen lines; adding the
// status bar then overflowed the terminal by 1 row, scrolling the very first
// line — the top panel's top border — off screen. This is what the original
// "top border of the panel is never highlighted" report actually was.
func TestRenderVerticalNeverOverflowsHeight(t *testing.T) {
	for _, h := range []int{20, 24, 30, 40} {
		twoPanel := NewPlansScreen(100, h, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationVertical)
		twoPanelLines := strings.Split(twoPanel.renderSplitView(), "\n")
		require.LessOrEqual(t, len(twoPanelLines)+1, h, "two-panel split, height=%d", h)

		threePanel := NewPlansScreen(100, h, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationVertical)
		threePanelLines := strings.Split(threePanel.renderThreePanelViewVertical(), "\n")
		require.LessOrEqual(t, len(threePanelLines)+1, h, "three-panel, height=%d", h)
	}
}

func TestUpdateListItems(t *testing.T) {
	t.Run("empty plans produces empty list", func(t *testing.T) {
		s := NewPlansScreen(80, 24, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.plans = []busquets.PlanSummary{}
		s.updateListItems()
		require.Equal(t, 0, s.list.ItemCount())
	})

	t.Run("items count matches plans count", func(t *testing.T) {
		s := NewPlansScreen(80, 24, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.plans = []busquets.PlanSummary{
			{Title: "Plan A", FileName: "a.md"},
			{Title: "Plan B", FileName: "b.md"},
		}
		s.updateListItems()
		require.Equal(t, 2, s.list.ItemCount())
	})

	t.Run("item titles match plan titles", func(t *testing.T) {
		s := NewPlansScreen(80, 24, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.plans = []busquets.PlanSummary{
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
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.allTags = []busquets.Tag{
			{Name: "go"},
			{Name: "api"},
		}
		s.tagPlanCounts = map[string]int{"go": 3, "api": 1}
		s.rebuildTagPanelEntries()
		require.Equal(t, "", s.tagPanel.SelectedTag())
	})

	t.Run("counts from tagPlanCounts are used when set", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.allTags = []busquets.Tag{{Name: "backend"}}
		s.tagPlanCounts = map[string]int{"backend": 7}
		s.rebuildTagPanelEntries()
		require.Equal(t, "", s.tagPanel.SelectedTag())
	})

	t.Run("tags missing from counts appear without panic", func(t *testing.T) {
		s := NewPlansScreen(100, 40, false, false, false, "tag_plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.allTags = []busquets.Tag{{Name: "orphan"}}
		s.tagPlanCounts = map[string]int{}
		s.rebuildTagPanelEntries()
		require.Equal(t, "", s.tagPanel.SelectedTag())
	})
}

func newLabelModeScreenWithPlans() *PlansScreen {
	s := NewPlansScreen(120, 40, false, false, false, busquets.DisplayModeLabelPlanContent, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
	s.allPlans = []busquets.PlanSummary{
		{FileName: "a.md", SyncSource: "/srv/work", SyncLabel: "work", Title: "A"},
		{FileName: "b.md", SyncSource: "/srv/work", SyncLabel: "work", Title: "B"},
		{FileName: "c.md", SyncSource: "/srv/mine", SyncLabel: "personal", Title: "C"},
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

	t.Run("each label shows its source path, the All row does not", func(t *testing.T) {
		view := s.labelPanel.View()
		require.Contains(t, view, "/srv/work")
		require.Contains(t, view, "/srv/mine")

		// One label row + one path row per label, plus the pathless All row.
		require.Equal(t, 3, strings.Count(view, "(")) // All(3), personal(1), work(2)
		require.Equal(t, 2, strings.Count(view, "/srv/"))
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
		Tags:       []busquets.Tag{{Name: "go"}, {Name: "api"}},
		Counts:     map[string]int{"go": 1},
		TagPlanMap: map[string][]busquets.PlanSummary{"go": {{FileName: "a.md"}}},
		AllPlans:   []busquets.PlanSummary{{FileName: "a.md", SyncLabel: "work"}},
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
		{busquets.DisplayModeTagPlanContent, types.FocusTagPanel},
		{busquets.DisplayModeLabelPlanContent, types.FocusLabelPanel},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Run("shift+tab from the list reaches the side panel", func(t *testing.T) {
				s := NewPlansScreen(120, 40, false, false, false, tc.mode, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
				s.focus = types.FocusList

				screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
				require.Equal(t, tc.focus, screen.(*PlansScreen).focus)
			})

			t.Run("tab from content reaches the side panel", func(t *testing.T) {
				s := NewPlansScreen(120, 40, false, false, false, tc.mode, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
				s.focus = types.FocusContent

				screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyTab})
				require.Equal(t, tc.focus, screen.(*PlansScreen).focus)
			})
		})
	}

	t.Run("plan_content has no side panel to reach", func(t *testing.T) {
		s := NewPlansScreen(120, 40, false, false, false, busquets.DisplayModePlanContent, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.focus = types.FocusList

		screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		require.Equal(t, types.FocusList, screen.(*PlansScreen).focus)
	})
}

func TestSetDisplayModeMountsOnePanel(t *testing.T) {
	s := NewPlansScreen(120, 40, false, false, false, busquets.DisplayModePlanContent, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)

	s.SetDisplayMode(busquets.DisplayModeTagPlanContent)
	require.NotNil(t, s.tagPanel)
	require.Nil(t, s.labelPanel)
	require.Equal(t, types.FocusTagPanel, s.focus)
	require.Equal(t, types.LayoutThreePanel, s.layout)

	s.SetDisplayMode(busquets.DisplayModeLabelPlanContent)
	require.Nil(t, s.tagPanel)
	require.NotNil(t, s.labelPanel)
	require.Equal(t, types.FocusLabelPanel, s.focus)
	require.Equal(t, types.LayoutThreePanel, s.layout)

	s.SetDisplayMode(busquets.DisplayModePlanContent)
	require.Nil(t, s.tagPanel)
	require.Nil(t, s.labelPanel)
	require.Equal(t, types.FocusList, s.focus)
	require.Equal(t, types.LayoutSplit, s.layout)
}

func TestLeavingFullscreenRestoresThreePanelLayout(t *testing.T) {
	for _, mode := range []string{
		busquets.DisplayModeTagPlanContent,
		busquets.DisplayModeLabelPlanContent,
	} {
		t.Run(mode, func(t *testing.T) {
			s := NewPlansScreen(120, 40, false, false, false, mode, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
			s.focus = types.FocusList
			s.current = &busquets.PlanDetail{
				PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "P"},
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
	s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
	s.focus = types.FocusList
	s.plans = []busquets.PlanSummary{{FileName: "p.md", SyncSource: "/src", Title: "My Plan"}}
	s.current = &busquets.PlanDetail{
		PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "My Plan"},
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
		s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.focus = focus
		s.plans = []busquets.PlanSummary{{FileName: "p.md", SyncSource: "/src", Title: "My Plan"}}
		s.current = &busquets.PlanDetail{
			PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "My Plan"},
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
		busquets.DisplayModePlanContent,
		busquets.DisplayModeTagPlanContent,
		busquets.DisplayModeLabelPlanContent,
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			s := NewPlansScreen(120, 40, false, false, false, mode, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
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
		busquets.DisplayModePlanContent,
		busquets.DisplayModeTagPlanContent,
		busquets.DisplayModeLabelPlanContent,
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			s := NewPlansScreen(120, 40, false, false, false, mode, busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
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
	s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
	s.focus = types.FocusList
	s.searchQuery = "needle"
	s.plans = []busquets.PlanSummary{{FileName: "p.md", SyncSource: "/src", Title: "My Plan"}}
	s.updateListItems()

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Empty(t, s.searchQuery)
	require.NotNil(t, cmd)
	_, ok := cmd().(messages.ClearSearchMsg)
	require.True(t, ok)
}

func TestSaveResultMsgRefreshesContent(t *testing.T) {
	newScreen := func() *PlansScreen {
		s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
		s.current = &busquets.PlanDetail{
			PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "Old Title"},
			Content:     "old content",
		}
		s.editor.SetContent("old content")
		s.editor.Focus()
		s.focus = types.FocusEditor
		s.layout = types.LayoutFullscreen // matches the real "e" edit-entry path
		return s
	}

	t.Run("populated Plan updates viewer and current but stays in the editor", func(t *testing.T) {
		s := newScreen()
		newPlan := &busquets.PlanDetail{
			PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "New Title"},
			Content:     "new saved content",
		}

		screen, cmd := s.Update(messages.SaveResultMsg{
			Result: &busquets.UpdatePlanResult{Success: true},
			Plan:   newPlan,
		})
		ps := screen.(*PlansScreen)

		require.Nil(t, cmd)
		require.Equal(t, newPlan, ps.current)
		// A save must not redirect the user out of the editor — only esc does.
		require.Equal(t, types.FocusEditor, ps.focus)
		require.True(t, ps.editor.Focused())
		require.Equal(t, "old content", ps.editor.Content()) // untouched by the save
		// The viewer is refreshed in the background so it's correct whenever
		// the user does leave the editor, even though it isn't shown yet.
		require.Contains(t, ps.viewer.View(), "new saved content")
	})

	t.Run("esc after a save resets to the saved content, not the pre-save content", func(t *testing.T) {
		s := newScreen()
		newPlan := &busquets.PlanDetail{
			PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "New Title"},
			Content:     "old content", // what was actually on disk when the save fired
		}
		screen, _ := s.Update(messages.SaveResultMsg{
			Result: &busquets.UpdatePlanResult{Success: true},
			Plan:   newPlan,
		})
		ps := screen.(*PlansScreen)
		ps.editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" plus unsaved edit")})
		require.True(t, ps.editor.IsModified())

		screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyEsc})
		ps = screen.(*PlansScreen)

		require.Equal(t, types.FocusContent, ps.focus)
		require.Equal(t, "old content", ps.editor.Content())
		require.False(t, ps.editor.IsModified())
	})

	t.Run("nil Plan is a no-op", func(t *testing.T) {
		s := newScreen()

		screen, cmd := s.Update(messages.SaveResultMsg{
			Result: &busquets.UpdatePlanResult{Success: true},
			Plan:   nil,
		})
		ps := screen.(*PlansScreen)

		require.Nil(t, cmd)
		require.Equal(t, "Old Title", ps.current.Title)
		require.Equal(t, "old content", ps.editor.Content())
		require.Equal(t, types.FocusEditor, ps.focus)
	})
}

func TestEditorModeToggleAndEscReturnsToViewer(t *testing.T) {
	s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
	s.current = &busquets.PlanDetail{
		PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "My Plan"},
		Content:     "some content",
	}
	s.editor.SetContent("some content")
	s.editor.Focus()
	s.focus = types.FocusEditor
	s.layout = types.LayoutFullscreen

	require.Equal(t, types.EditorModeNavigation, s.editor.EditorMode())

	screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ps := screen.(*PlansScreen)
	require.Equal(t, types.EditorModeInsert, ps.editor.EditorMode())

	screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyEsc})
	ps = screen.(*PlansScreen)
	require.Equal(t, types.FocusContent, ps.focus)

	screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	ps = screen.(*PlansScreen)
	require.Equal(t, types.FocusEditor, ps.focus)
	require.Equal(t, types.EditorModeNavigation, ps.editor.EditorMode())
}

func TestStashPreservesEditAcrossEscAndBackToEditor(t *testing.T) {
	s := NewPlansScreen(100, 40, false, false, false, "plan_content", busquets.MarkdownThemeASCII, busquets.ScreenOrientationHorizontal)
	s.current = &busquets.PlanDetail{
		PlanSummary: busquets.PlanSummary{FileName: "p.md", SyncSource: "/src", Title: "My Plan"},
		Content:     "some content",
	}
	s.editor.SetContent("some content")
	s.editor.Focus()
	s.focus = types.FocusEditor
	s.layout = types.LayoutFullscreen

	// Enter insert mode and make an edit.
	screen, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ps := screen.(*PlansScreen)
	// Focus() leaves the cursor at the start of the document, so this prepends.
	ps.editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("EDITED: ")})
	require.True(t, ps.editor.IsModified())

	// Back to navigation mode, then stash the edit before leaving.
	screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ps = screen.(*PlansScreen)
	require.Equal(t, types.EditorModeNavigation, ps.editor.EditorMode())

	screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	ps = screen.(*PlansScreen)

	// esc: without a stash this would discard the edit via Reset().
	screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyEsc})
	ps = screen.(*PlansScreen)
	require.Equal(t, types.FocusContent, ps.focus)

	// Back into the editor: the stashed edit — and its modified state — must survive.
	screen, _ = ps.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	ps = screen.(*PlansScreen)
	require.Equal(t, types.FocusEditor, ps.focus)
	require.Equal(t, "EDITED: some content", ps.editor.Content())
	require.True(t, ps.editor.IsModified())
}
