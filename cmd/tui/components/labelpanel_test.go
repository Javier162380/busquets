package components

import (
	"strings"
	"testing"

	"github.com/Javier162380/busquets/cmd/tui/styles"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

func newTestLabelPanel(height int) *LabelPanel {
	p := NewLabelPanel(40, height)
	p.SetEntries([]LabelPanelEntry{
		{Name: "personal", Path: "/srv/mine", PlanCount: 1},
		{Name: "work", Path: "/srv/work", PlanCount: 2},
	}, 3)
	return p
}

func TestLabelPanelRendersPathUnderEachLabel(t *testing.T) {
	view := newTestLabelPanel(20).View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")

	require.Len(t, lines, 5) // All + (personal, path) + (work, path)
	require.Contains(t, lines[0], "All")
	require.Contains(t, lines[1], "personal")
	require.Contains(t, lines[2], "/srv/mine")
	require.Contains(t, lines[3], "work")
	require.Contains(t, lines[4], "/srv/work")
}

func TestWrapPath(t *testing.T) {
	t.Run("a path that fits stays on one line", func(t *testing.T) {
		require.Equal(t, []string{"/srv/work"}, wrapPath("/srv/work", 20))
	})

	t.Run("breaks on separators, keeping each on the line it ends", func(t *testing.T) {
		require.Equal(t,
			[]string{"/Users/javier/", ".claude/plans"},
			wrapPath("/Users/javier/.claude/plans", 16),
		)
	})

	t.Run("a segment too long for one line is hard-split", func(t *testing.T) {
		lines := wrapPath("/a/averyveryverylongdirectoryname/b", 10)
		for _, line := range lines {
			require.LessOrEqual(t, len(line), 10)
		}
		require.Equal(t, "/a/averyveryverylongdirectoryname/b", strings.Join(lines, ""))
	})
}

func TestLabelPanelWrapsLongPathAcrossLines(t *testing.T) {
	const path = "/Users/someone/very/deep/nested/docs/plans"

	p := NewLabelPanel(24, 20)
	p.SetEntries([]LabelPanelEntry{{Name: "work", Path: path, PlanCount: 1}}, 1)

	view := p.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	require.Greater(t, len(lines), 3, "expected the path to wrap over several lines")

	// Every character of the path survives — nothing is truncated away.
	// lines[0] is the "All" row and lines[1] the label row; the rest is the path.
	var rebuilt strings.Builder
	for _, line := range lines[2:] {
		rebuilt.WriteString(strings.TrimSpace(line))
	}
	require.Equal(t, path, rebuilt.String())
	require.NotContains(t, view, "…")
}

func TestLabelPanelScrollWindowCountsRowsNotEntries(t *testing.T) {
	// Three rows fits the All row plus one label with its path.
	p := newTestLabelPanel(3)
	require.Equal(t, 3, strings.Count(strings.TrimRight(p.View(), "\n"), "\n")+1)

	// Moving to the last entry scrolls it into view; the All row drops off.
	p.MoveDown()
	require.Equal(t, "work", p.MoveDown())

	view := p.View()
	require.Contains(t, view, "work")
	require.Contains(t, view, "/srv/work")
	require.NotContains(t, view, "All")
}

// ansi256 is the escape-sequence fragment lipgloss emits for a 256-colour
// foreground, e.g. "38;5;238".
func ansi256(c lipgloss.Color) string {
	return "38;5;" + string(c)
}

func TestLabelPanelFollowsTheTheme(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
		styles.SetDarkMode(true)
	})

	// Returns the unselected label row and the path row beneath it.
	render := func() (string, string) {
		p := NewLabelPanel(30, 10)
		p.SetEntries([]LabelPanelEntry{{Name: "work", Path: "/srv/work", PlanCount: 2}}, 2)
		lines := strings.Split(strings.TrimRight(p.View(), "\n"), "\n")
		require.Len(t, lines, 3)
		return lines[1], lines[2]
	}

	pathByTheme := map[string]string{}

	for _, tc := range []struct {
		name string
		dark bool
	}{
		{"dark", true},
		{"light", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			styles.SetDarkMode(tc.dark)
			labelLine, pathLine := render()

			// The panel reads styles at render time, so it tracks the active theme.
			require.Contains(t, labelLine, ansi256(styles.InactiveColor))
			require.Contains(t, pathLine, ansi256(styles.SubtleColor))

			// The path must recede behind the label it belongs to, not out-shout it
			// — SubtleColor exists precisely because MetaStyle (ForegroundColor)
			// is brighter than the row text in dark and darker in light.
			require.NotEqual(t, styles.SubtleColor, styles.ForegroundColor)
			require.NotEqual(t, styles.SubtleColor, styles.InactiveColor)

			pathByTheme[tc.name] = pathLine
		})
	}

	require.NotEqual(t, pathByTheme["dark"], pathByTheme["light"],
		"switching the theme must change how the path renders")
}

func TestLabelPanelSelection(t *testing.T) {
	p := newTestLabelPanel(20)

	require.Equal(t, "", p.SelectedLabel()) // All
	require.Equal(t, "personal", p.MoveDown())
	require.Equal(t, "work", p.MoveDown())
	require.Equal(t, "work", p.MoveDown()) // clamped at the end
	require.Equal(t, "personal", p.MoveUp())
	require.Equal(t, "", p.MoveUp())
	require.Equal(t, "", p.MoveUp()) // clamped at the start
}
