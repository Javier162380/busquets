package components

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	content_test "github.com/Javier162380/claude-plan-viewer/cmd/tui/content/test"

	"github.com/golang/mock/gomock"
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
