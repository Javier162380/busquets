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

func int64Ptr(v int64) *int64 { return &v }

var memoryTestTime = time.Date(2026, 9, 2, 14, 2, 0, 0, time.UTC)

// memoryWithEvents builds a memory carrying the given timeline.
func memoryWithEvents(exists bool, events []busquets.MemoryEvent) *busquets.PlanMemory {
	memory := &busquets.PlanMemory{
		PlanID:            1,
		FileName:          "plan.md",
		SyncSource:        "/src",
		PlanTitle:         "Test Plan",
		CoversUpToVersion: busquets.NoMemoryCoverage,
		Events:            events,
	}
	if exists {
		memory.ID = 7
		memory.Content = "# Memory: Test Plan\n\nA summary.\n\n## Timeline\n\n### v0\n\nIt began.\n"
		memory.Summary = "A summary."
		memory.CoversUpToVersion = 0
		memory.GeneratedBy = "ollama"
	}
	return memory
}

func versionEvent(number int64, added, removed int) busquets.MemoryEvent {
	return busquets.MemoryEvent{
		Kind:          busquets.MemoryEventVersion,
		RefID:         100 + number,
		VersionNumber: int64Ptr(number),
		OccurredAt:    memoryTestTime,
		LinesAdded:    added,
		LinesRemoved:  removed,
		WordCount:     120,
	}
}

func newTestMemoryScreen(t *testing.T, memory *busquets.PlanMemory, staleness busquets.MemoryStaleness) *MemoryScreen {
	t.Helper()
	return NewMemoryScreenWithData(
		"plan.md", "/src", busquets.MarkdownThemeASCII, memory, staleness,
		100, 40, false, false, busquets.ScreenOrientationHorizontal,
	)
}

func TestMemoryScreenTimelineRows(t *testing.T) {
	t.Run("labels each event kind from computed facts", func(t *testing.T) {
		restore := versionEvent(2, 4, 96)
		restore.Kind = busquets.MemoryEventRestore
		restore.RestoredFrom = int64Ptr(0)

		comment := busquets.MemoryEvent{
			Kind: busquets.MemoryEventComment, RefID: 5,
			OccurredAt: memoryTestTime, Body: "needs a rethink",
		}

		initial := versionEvent(0, 0, 0)
		changed := versionEvent(1, 38, 12)

		require.Equal(t, "v0", memoryEventTitle(initial))
		require.Equal(t, "initial | 120 words", memoryEventDetail(initial))
		require.Equal(t, "+38 −12", memoryEventDetail(changed))
		require.Equal(t, "restored from v0", memoryEventDetail(restore))
		require.Equal(t, "Comment", memoryEventTitle(comment))
		require.Equal(t, "needs a rethink", memoryEventDetail(comment))
	})

	t.Run("truncates a long comment to one row", func(t *testing.T) {
		long := strings.Repeat("a", 100)
		require.Len(t, firstLine(long), 60)
		require.True(t, strings.HasSuffix(firstLine(long), "..."))
		require.Equal(t, "first line", firstLine("first line\nsecond line"))
	})

	t.Run("populates the list from the timeline", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, []busquets.MemoryEvent{
			versionEvent(0, 0, 0), versionEvent(1, 5, 1),
		}), busquets.MemoryStaleness{HasMemory: true})

		require.Len(t, s.events, 2)
		require.NotNil(t, s.current)
		require.Equal(t, int64(1), *s.current.VersionNumber, "the newest event is selected by default")
	})
}

func TestMemoryScreenOpensWithoutAMemory(t *testing.T) {
	// The deliberate difference from VersionsScreen: a plan with no memory still
	// opens, because the timeline is computed.
	s := newTestMemoryScreen(t, memoryWithEvents(false, []busquets.MemoryEvent{versionEvent(0, 0, 0)}),
		busquets.MemoryStaleness{NewVersions: 1})

	require.False(t, s.memory.Exists())
	require.Len(t, s.events, 1)
	require.Contains(t, s.ShortHelp(), "no memory yet")
	require.NotEmpty(t, s.View())
}

func TestMemoryScreenKeys(t *testing.T) {
	press := func(s *MemoryScreen, key string) tea.Cmd {
		var msg tea.KeyMsg
		if len(key) == 1 {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		} else {
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		}
		_, cmd := s.Update(msg)
		return cmd
	}

	t.Run("esc leaves the screen", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil), busquets.MemoryStaleness{HasMemory: true})
		cmd := press(s, "esc")
		require.NotNil(t, cmd)
		require.IsType(t, messages.PopScreenMsg{}, cmd())
	})

	t.Run("tab moves focus to the document and back", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil), busquets.MemoryStaleness{HasMemory: true})
		require.Equal(t, types.FocusTimeline, s.focus)

		s.Update(tea.KeyMsg{Type: tea.KeyTab})
		require.Equal(t, types.FocusContent, s.focus)

		s.Update(tea.KeyMsg{Type: tea.KeyTab})
		require.Equal(t, types.FocusTimeline, s.focus)
	})

	t.Run("r asks to write a memory that does not exist yet", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(false, []busquets.MemoryEvent{versionEvent(0, 0, 0)}),
			busquets.MemoryStaleness{NewVersions: 1})

		cmd := press(s, "r")
		require.NotNil(t, cmd)
		msg, ok := cmd().(messages.GenerateMemoryMsg)
		require.True(t, ok)
		require.Equal(t, busquets.MemoryModeIncremental, msg.Mode)
		require.True(t, s.generating)
	})

	t.Run("r refuses when the memory is already current", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, []busquets.MemoryEvent{versionEvent(0, 0, 0)}),
			busquets.MemoryStaleness{HasMemory: true})

		cmd := press(s, "r")
		require.NotNil(t, cmd)
		errMsg, ok := cmd().(messages.ErrorMsg)
		require.True(t, ok)
		require.Contains(t, errMsg.Error.Error(), "already up to date")
		require.False(t, s.generating)
	})

	t.Run("r refuses while a run is in flight", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(false, []busquets.MemoryEvent{versionEvent(0, 0, 0)}),
			busquets.MemoryStaleness{NewVersions: 1})
		s.generating = true

		cmd := press(s, "r")
		errMsg, ok := cmd().(messages.ErrorMsg)
		require.True(t, ok)
		require.Contains(t, errMsg.Error.Error(), "already being written")
	})

	t.Run("R opens a confirmation rather than rewriting immediately", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil), busquets.MemoryStaleness{HasMemory: true})

		press(s, "R")
		require.True(t, s.confirmDialog.IsActive())
		require.True(t, s.IsInputMode())
	})

	t.Run("d refuses when there is nothing to delete", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(false, nil), busquets.MemoryStaleness{})

		cmd := press(s, "d")
		errMsg, ok := cmd().(messages.ErrorMsg)
		require.True(t, ok)
		require.Contains(t, errMsg.Error.Error(), "no memory to delete")
		require.False(t, s.confirmDialog.IsActive())
	})

	t.Run("c refuses when there is nothing to copy", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(false, nil), busquets.MemoryStaleness{})

		cmd := press(s, "c")
		_, ok := cmd().(messages.ErrorMsg)
		require.True(t, ok)
	})
}

func TestMemoryScreenDiffRequest(t *testing.T) {
	t.Run("enter requests the diff for a changed version", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, []busquets.MemoryEvent{
			versionEvent(0, 0, 0), versionEvent(1, 5, 1),
		}), busquets.MemoryStaleness{HasMemory: true})

		cmd := s.requestDiff()
		require.NotNil(t, cmd)
		msg, ok := cmd().(messages.RequestMemoryDiffMsg)
		require.True(t, ok)
		require.Equal(t, int64(0), msg.FromVersion)
		require.Equal(t, int64(1), msg.ToVersion)
	})

	t.Run("the first version has no diff", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, []busquets.MemoryEvent{versionEvent(0, 0, 0)}),
			busquets.MemoryStaleness{HasMemory: true})

		errMsg, ok := s.requestDiff()().(messages.ErrorMsg)
		require.True(t, ok)
		require.Contains(t, errMsg.Error.Error(), "baseline")
	})

	t.Run("a comment has no diff", func(t *testing.T) {
		comment := busquets.MemoryEvent{
			Kind: busquets.MemoryEventComment, RefID: 5, OccurredAt: memoryTestTime, Body: "note",
		}
		s := newTestMemoryScreen(t, memoryWithEvents(true, []busquets.MemoryEvent{comment}),
			busquets.MemoryStaleness{HasMemory: true})

		errMsg, ok := s.requestDiff()().(messages.ErrorMsg)
		require.True(t, ok)
		require.Contains(t, errMsg.Error.Error(), "comment has no diff")
	})

	t.Run("a loaded diff switches to the diff view", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, []busquets.MemoryEvent{
			versionEvent(0, 0, 0), versionEvent(1, 5, 1),
		}), busquets.MemoryStaleness{HasMemory: true})

		s.Update(messages.MemoryDiffLoadedMsg{Diff: "--- v0\n+++ v1\n+added\n"})
		require.Equal(t, types.FocusDiff, s.focus)
		require.Contains(t, s.ShortHelp(), "copy diff")

		s.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.Equal(t, types.FocusTimeline, s.focus)
	})
}

func TestMemoryScreenStatusText(t *testing.T) {
	t.Run("reports how many events are pending", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil),
			busquets.MemoryStaleness{HasMemory: true, NewVersions: 2, NewComments: 1})
		require.Contains(t, s.ShortHelp(), "3 new events")
	})

	t.Run("reports an up-to-date memory", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil), busquets.MemoryStaleness{HasMemory: true})
		require.Contains(t, s.ShortHelp(), "up to date")
	})

	t.Run("reports a run in flight", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil), busquets.MemoryStaleness{HasMemory: true})
		s.generating = true
		require.Contains(t, s.ShortHelp(), "writing...")
	})
}

func TestMemoryScreenGeneratedMsgClearsFlag(t *testing.T) {
	s := newTestMemoryScreen(t, memoryWithEvents(false, nil), busquets.MemoryStaleness{})
	s.generating = true

	s.Update(messages.MemoryGeneratedMsg{
		Memory:    memoryWithEvents(true, []busquets.MemoryEvent{versionEvent(0, 0, 0)}),
		Staleness: busquets.MemoryStaleness{HasMemory: true},
	})

	require.False(t, s.generating)
	require.True(t, s.memory.Exists())
	require.Len(t, s.events, 1)
}

func TestMemoryScreenGetViewerWidth(t *testing.T) {
	t.Run("fullscreen returns width minus padding", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil), busquets.MemoryStaleness{})
		s.layout = types.LayoutFullscreen
		require.Equal(t, 96, s.getViewerWidth())
	})

	t.Run("split layout returns half width minus padding", func(t *testing.T) {
		s := newTestMemoryScreen(t, memoryWithEvents(true, nil), busquets.MemoryStaleness{})
		require.Equal(t, 44, s.getViewerWidth())
	})

	t.Run("vertical split returns full width minus padding", func(t *testing.T) {
		s := NewMemoryScreenWithData("plan.md", "/src", busquets.MarkdownThemeASCII,
			memoryWithEvents(true, nil), busquets.MemoryStaleness{},
			100, 40, false, false, busquets.ScreenOrientationVertical)
		require.Equal(t, 94, s.getViewerWidth())
	})
}

// TestMemoryScreenRenderNeverOverflowsHeight mirrors the versions and plans
// screens: App.View() appends a status-bar row below whatever a screen renders,
// so the screen must leave at least one row of slack.
func TestMemoryScreenRenderNeverOverflowsHeight(t *testing.T) {
	for _, orientation := range []string{busquets.ScreenOrientationHorizontal, busquets.ScreenOrientationVertical} {
		for _, height := range []int{20, 24, 30, 40} {
			s := NewMemoryScreenWithData("plan.md", "/src", busquets.MarkdownThemeASCII,
				memoryWithEvents(true, []busquets.MemoryEvent{versionEvent(0, 0, 0)}),
				busquets.MemoryStaleness{HasMemory: true},
				100, height, false, false, orientation)

			lines := strings.Split(s.renderSplitView(), "\n")
			require.LessOrEqual(t, len(lines)+1, height, "orientation=%s height=%d", orientation, height)
			require.Positive(t, lipgloss.Width(lines[0]))
		}
	}
}
