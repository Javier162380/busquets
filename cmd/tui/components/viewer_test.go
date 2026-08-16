package components

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	content_test "github.com/Javier162380/claude-plan-viewer/cmd/tui/content/test"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/golang/mock/gomock"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

// renderedLinesFixture builds n distinct lines joined by "\n", used to give
// Glamour-mode tests a rendered body with a different line count than the
// raw body, mirroring how Glamour reflows real markdown.
func renderedLinesFixture(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("rendered line %d", i+1)
	}
	return strings.Join(lines, "\n")
}

// newMockDisplayable stubs a content.Displayable so header length
// (sourcePath/destinationPath presence), raw body, and rendered body are all
// independently under the test's control — renderedBody deliberately gets a
// different line count than body, mirroring how Glamour reflows real
// markdown. Pass "" for renderedBody in raw-mode-only tests.
func newMockDisplayable(t *testing.T, body, renderedBody, sourcePath, destinationPath string) *content_test.MockDisplayable {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := content_test.NewMockDisplayable(ctrl)
	m.EXPECT().GetContent().Return(body).AnyTimes()
	m.EXPECT().GetRenderedHTML(gomock.Any()).Return(renderedBody).AnyTimes()
	m.EXPECT().GetReadingTime().Return(1).AnyTimes()
	m.EXPECT().GetMetadata().Return(content.Metadata{
		PrimaryLabel:    "Modified",
		PrimaryTime:     time.Now(),
		SecondaryInfo:   "Size: 0 bytes",
		SourcePath:      sourcePath,
		DestinationPath: destinationPath,
	}).AnyTimes()
	return m
}

// TestViewerContentLineSync covers CurrentContentLine and ScrollToContentLine
// together — they're inverses of each other (read vs. write the same
// scroll<->line mapping), so each case's setup either sets the scroll
// position directly (to check CurrentContentLine's read side) or calls
// ScrollToContentLine (to check its write side), then asserts what
// CurrentContentLine reports.
func TestViewerContentLineSync(t *testing.T) {
	body := strings.Join([]string{
		"line 1", "line 2", "line 3", "line 4", "line 5",
		"line 6", "line 7", "line 8", "line 9", "line 10",
	}, "\n")

	tests := []struct {
		name            string
		height          int
		renderedBody    string
		sourcePath      string
		destinationPath string
		glamour         bool
		setup           func(v *Viewer)
		want            int
	}{
		{
			name:   "CurrentContentLine returns 1 with no scroll",
			height: 5,
			setup:  func(v *Viewer) {},
			want:   1,
		},
		{
			name:   "CurrentContentLine accounts for the header when scrolled, no source/destination path",
			height: 5,
			// Header here is: meta line + blank separator = 2 lines.
			setup: func(v *Viewer) { v.viewport.YOffset = 2 + 3 }, // scrolled to content line 4 (1-indexed)
			want:  4,
		},
		{
			name:            "CurrentContentLine accounts for a longer header with source and destination path",
			height:          5,
			sourcePath:      "/src/a.md",
			destinationPath: "/dst/a.md",
			// Header here is: meta line + source + destination + blank = 4 lines.
			setup: func(v *Viewer) { v.viewport.YOffset = 4 + 3 },
			want:  4,
		},
		{
			name:   "CurrentContentLine clamps to 1 while still scrolled within the header",
			height: 5,
			setup:  func(v *Viewer) { v.viewport.YOffset = 0 },
			want:   1,
		},
		{
			name:         "CurrentContentLine approximates via scroll percentage in Glamour mode, using the raw line count",
			height:       4,
			renderedBody: renderedLinesFixture(20),
			glamour:      true,
			// Header (2) + 20 rendered lines = 22 total; height 4 => maxYOffset 18.
			// Halfway scrolled (YOffset 9) should map to roughly the middle of the 10 raw lines.
			setup: func(v *Viewer) { v.viewport.YOffset = 9 },
			want:  6,
		},
		{
			name:   "ScrollToContentLine scrolls so the requested line is at the top",
			height: 5,
			setup:  func(v *Viewer) { v.ScrollToContentLine(6) },
			want:   6,
		},
		{
			name:   "ScrollToContentLine clamps a line below 1 to the top",
			height: 5,
			setup: func(v *Viewer) {
				v.viewport.YOffset = 3
				v.ScrollToContentLine(0)
			},
			want: 1,
		},
		{
			name:         "ScrollToContentLine approximates via scroll percentage in Glamour mode",
			height:       4,
			renderedBody: renderedLinesFixture(20),
			glamour:      true,
			setup:        func(v *Viewer) { v.ScrollToContentLine(6) },
			want:         6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := NewViewer(80, tc.height, "dark")
			v.SetContent(newMockDisplayable(t, body, tc.renderedBody, tc.sourcePath, tc.destinationPath))
			if tc.glamour {
				v.ToggleRenderMode()
			}
			tc.setup(v)
			require.Equal(t, tc.want, v.CurrentContentLine())
		})
	}
}

// TestViewerSearch covers the viewer-level search wrappers around
// ContentSearch — ContentSearch's own matching/indexing logic is covered in
// contentsearch_test.go, so these tests focus on what's specific to Viewer:
// wiring Search/SearchNext/SearchPrev/ClearSearch to scroll position and
// highlight rendering, and confirming search survives a re-render that
// isn't a content change (the SetContent-vs-updateViewportContent split
// this plan called out as a staleness risk).
func TestViewerSearch(t *testing.T) {
	// Padded with trailing filler lines so a height-5 viewport has scroll
	// headroom to reach line 5 (bubbles' viewport clamps YOffset to
	// totalLines-height; without padding, an 8-line total for a height-5
	// viewport leaves too little room and every scroll silently clamps).
	body := strings.Join([]string{
		"line one", "target here", "line three",
		"line four", "target again", "line six",
		"line seven", "line eight", "line nine", "line ten",
	}, "\n")

	t.Run("Search jumps to the first match", func(t *testing.T) {
		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, "", "", ""))

		v.Search("target")
		require.True(t, v.HasMatches())
		current, total := v.SearchStatus()
		require.Equal(t, 1, current)
		require.Equal(t, 2, total)
		require.Equal(t, 2, v.CurrentContentLine())
	})

	t.Run("SearchNext and SearchPrev cycle and wrap", func(t *testing.T) {
		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, "", "", ""))
		v.Search("target")

		v.SearchNext()
		require.Equal(t, 5, v.CurrentContentLine())
		v.SearchNext()
		require.Equal(t, 2, v.CurrentContentLine(), "SearchNext should wrap from the last match back to the first")

		v.SearchPrev()
		require.Equal(t, 5, v.CurrentContentLine(), "SearchPrev should wrap from the first match back to the last")
	})

	t.Run("query with no matches", func(t *testing.T) {
		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, "", "", ""))

		v.Search("nonexistent")
		require.False(t, v.HasMatches())
		current, total := v.SearchStatus()
		require.Equal(t, 0, current)
		require.Equal(t, 0, total)
	})

	t.Run("ClearSearch drops the highlight", func(t *testing.T) {
		// Force a color profile that actually emits ANSI codes — lipgloss
		// renders plain text with no codes at all outside a TTY, which is
		// the default in `go test`, and would make every Render() call
		// below indistinguishable from unstyled text (see
		// TestLabelPanelFollowsTheTheme for the same pattern).
		previousProfile := lipgloss.ColorProfile()
		lipgloss.SetColorProfile(termenv.ANSI256)
		t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, "", "", ""))
		// Match Search("target")'s eventual scroll position up front, so the
		// only difference between captures below is the highlight itself,
		// not scroll position (Search scrolls to the first match; ClearSearch
		// does not scroll back).
		v.ScrollToContentLine(2)
		unstyledView := v.View()

		v.Search("target")
		require.True(t, v.HasMatches())
		require.NotEqual(t, unstyledView, v.View(), "a highlighted match must render differently from the unstyled view")

		v.ClearSearch()
		require.False(t, v.HasMatches())
		require.Equal(t, unstyledView, v.View(), "clearing the search must restore exactly the unstyled rendering")
	})

	t.Run("raw mode highlights the active match distinctly from other matches", func(t *testing.T) {
		previousProfile := lipgloss.ColorProfile()
		lipgloss.SetColorProfile(termenv.ANSI256)
		t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, "", "", ""))
		v.Search("target")

		rendered := v.View()
		require.Contains(t, rendered, styles.SearchCurrentMatchStyle.Render("target"))
		require.Contains(t, rendered, styles.SearchMatchStyle.Render("target"))
		require.NotEqual(t,
			styles.SearchCurrentMatchStyle.Render("target"),
			styles.SearchMatchStyle.Render("target"),
			"the active and non-active match styles must actually differ, or this test can't tell them apart",
		)
	})

	t.Run("search survives a re-render that is not a content change", func(t *testing.T) {
		// Regression guard: updateViewportContent runs on SetSize too, and
		// must not rebuild the search index (only SetContent may) or an
		// in-progress SearchNext position would silently reset to the first
		// match on every resize.
		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, "", "", ""))
		v.Search("target")
		v.SearchNext()
		require.Equal(t, 5, v.CurrentContentLine())

		v.SetSize(100, 6)

		require.True(t, v.HasMatches())
		current, _ := v.SearchStatus()
		require.Equal(t, 2, current, "SearchNext's position must survive a resize-triggered re-render")
	})

	t.Run("Glamour mode: search still finds matches and HasMatches is accurate", func(t *testing.T) {
		// Exact line-jump isn't meaningful in Glamour mode (reflowed), but
		// matching itself must still work since the index is built from raw
		// content regardless of render mode.
		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, renderedLinesFixture(6), "", ""))
		v.ToggleRenderMode()

		v.Search("target")
		require.True(t, v.HasMatches())
		_, total := v.SearchStatus()
		require.Equal(t, 2, total)
	})

	t.Run("Glamour mode highlights the whole line even when Glamour resets styling mid-line", func(t *testing.T) {
		// Glamour emits a full SGR reset between nearly every styled span
		// (bold, code, links). Wrapping that line as-is in a background
		// style would only highlight the prefix before the first such
		// reset, so applyHighlight must strip ANSI and highlight the plain
		// text instead of the original styled line.
		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, "target", "", "", ""))
		v.renderMode = RenderModeGlamour
		v.search.SetQuery("target", 1)
		require.True(t, v.search.HasMatches())

		styledLine := "before \x1b[m\x1b[1mtarget\x1b[m after"
		out := v.applyHighlight([]string{styledLine})

		require.Len(t, out, 1)
		require.Equal(t, styles.SearchMatchStyle.Render(ansi.Strip(styledLine)), out[0])
		require.NotEqual(t, styles.SearchMatchStyle.Render(styledLine), out[0],
			"highlighting the original styled line (not the stripped text) would leak Glamour's embedded reset and only cover the prefix before it")
	})

	t.Run("Glamour mode marks the current match distinctly, approximated from its raw line position", func(t *testing.T) {
		body := strings.Join([]string{
			"target line one", "filler", "filler", "filler", "target line two",
		}, "\n")
		rendered := []string{
			"target line one", "filler", "filler", "filler", "target line two",
		}

		v := NewViewer(80, 5, "dark")
		v.SetContent(newMockDisplayable(t, body, "", "", ""))
		v.renderMode = RenderModeGlamour
		v.search.SetQuery("target", 1)
		require.Equal(t, 1, v.search.CurrentLine())

		out := v.applyHighlight(append([]string(nil), rendered...))
		require.Equal(t, styles.SearchCurrentMatchStyle.Render("target line one"), out[0])
		require.Equal(t, styles.SearchMatchStyle.Render("target line two"), out[4])

		v.search.Next()
		require.Equal(t, 5, v.search.CurrentLine())

		out = v.applyHighlight(append([]string(nil), rendered...))
		require.Equal(t, styles.SearchMatchStyle.Render("target line one"), out[0])
		require.Equal(t, styles.SearchCurrentMatchStyle.Render("target line two"), out[4])
	})
}
