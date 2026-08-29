// Package screens provides screen implementations for the TUI.
package screens

import (
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Screen is the interface that all screens must implement.
type Screen interface {
	// Init initializes the screen and returns an initial command.
	Init() tea.Cmd

	// Update handles messages and returns the updated screen and a command.
	Update(msg tea.Msg) (Screen, tea.Cmd)

	// View renders the screen content.
	View() string

	// SetSize updates the screen dimensions.
	SetSize(width, height int)

	// ShortHelp returns key binding help text for the status bar.
	ShortHelp() string

	// IsInputMode returns true when the screen is capturing text input.
	IsInputMode() bool

	// EditorMode returns true when the screen is on an edit focus.
	EditorMode() bool
}

// overlayContent overlays a modal's raw (unpadded) view onto base within a
// width x height canvas, positioned as lipgloss.Place(width, height, hPos,
// vPos, overlay) would — but only the columns/rows the content actually
// occupies are replaced, so surrounding padding doesn't blank the base.
func overlayContent(base string, width, height int, hPos, vPos lipgloss.Position, overlay string) string {
	if overlay == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	contentWidth := 0
	for _, l := range overlayLines {
		if w := ansi.StringWidth(l); w > contentWidth {
			contentWidth = w
			if contentWidth >= width { // gap can't go positive again past here
				break
			}
		}
	}

	// PlaceVertical no-ops (top-aligned, unpadded) once content fills height.
	vGap := height - len(overlayLines)
	yOffset := 0
	if vGap > 0 {
		yOffset = placePad(vGap, vPos)
	}

	// PlaceHorizontal no-ops (left-aligned, unpadded) once content fills width.
	hGap := width - contentWidth
	noHPad := hGap <= 0

	maxLines := max(len(baseLines), height)
	result := make([]string, maxLines)
	for i := range maxLines {
		var baseLine string
		if i < len(baseLines) {
			baseLine = baseLines[i]
		}

		row := i - yOffset
		if row < 0 || row >= len(overlayLines) {
			result[i] = baseLine
			continue
		}

		overlayLine := overlayLines[row]
		lineWidth := ansi.StringWidth(overlayLine)

		xOffset := 0
		if !noHPad {
			short := max(0, contentWidth-lineWidth)
			xOffset = placePad(hGap+short, hPos)
		}

		result[i] = spliceRow(baseLine, xOffset, overlayLine, lineWidth)
	}

	return strings.Join(result, "\n")
}

// spliceRow replaces base's columns [xOffset, xOffset+overlayWidth) with
// overlayLine, keeping the rest of base untouched. base is right-padded
// first if too short to reach xOffset, so the splice stays column-aligned.
func spliceRow(base string, xOffset int, overlayLine string, overlayWidth int) string {
	baseWidth := ansi.StringWidth(base)
	if baseWidth < xOffset {
		base += strings.Repeat(" ", xOffset-baseWidth)
		baseWidth = xOffset
	}

	left := ansi.Cut(base, 0, xOffset)

	var right string
	if cut := xOffset + overlayWidth; cut < baseWidth {
		right = ansi.Cut(base, cut, baseWidth)
	}

	return left + overlayLine + right
}

// placePad returns the lead padding for pos, matching lipgloss's own
// PlaceHorizontal/PlaceVertical math bit-for-bit.
func placePad(totalGap int, pos lipgloss.Position) int {
	switch pos {
	case lipgloss.Left: // == lipgloss.Top (0.0)
		return 0
	case lipgloss.Right: // == lipgloss.Bottom (1.0)
		return totalGap
	default:
		v := math.Min(1, math.Max(0, float64(pos)))
		split := int(math.Round(float64(totalGap) * v))
		return totalGap - split
	}
}
