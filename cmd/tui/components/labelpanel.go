package components

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
)

const (
	// allLabelsRow is the display text of the header row that clears the label filter.
	allLabelsRow = "All"
	// pathIndent prefixes each wrapped line of a label's source path.
	pathIndent = "    "
)

// LabelPanelEntry represents one entry in the label panel. An entry renders as a
// label row plus, when Path is set, a muted source-path row beneath it.
type LabelPanelEntry struct {
	Name      string // sync label; "" is the "All" header row
	Path      string // the label's source directory; empty for the "All" row
	PlanCount int    // plans carrying this label
}

// LabelPanel is a scrollable list of sync labels used to filter the plan list.
// Labels are read-only in the TUI, so the panel takes no input and emits no messages.
type LabelPanel struct {
	entries []LabelPanelEntry
	cursor  int
	width   int
	height  int
	focused bool
}

// NewLabelPanel creates a new LabelPanel sized to the given dimensions.
func NewLabelPanel(width, height int) *LabelPanel {
	return &LabelPanel{
		entries: []LabelPanelEntry{{Name: "", PlanCount: 0}},
		width:   width,
		height:  height,
	}
}

// SetEntries replaces the label list, prepending the "All" row with totalPlans as its count.
func (l *LabelPanel) SetEntries(entries []LabelPanelEntry, totalPlans int) {
	l.entries = make([]LabelPanelEntry, 0, len(entries)+1)
	l.entries = append(l.entries, LabelPanelEntry{Name: "", PlanCount: totalPlans})
	l.entries = append(l.entries, entries...)
	if l.cursor >= len(l.entries) {
		l.cursor = 0
	}
}

// Focus marks the panel as focused.
func (l *LabelPanel) Focus() {
	l.focused = true
}

// Blur marks the panel as unfocused.
func (l *LabelPanel) Blur() {
	l.focused = false
}

// SelectedLabel returns the label under the cursor; "" means "All".
func (l *LabelPanel) SelectedLabel() string {
	if l.cursor < 0 || l.cursor >= len(l.entries) {
		return ""
	}
	return l.entries[l.cursor].Name
}

// SetSize updates panel dimensions.
func (l *LabelPanel) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// MoveUp moves the cursor up one entry and returns the newly selected label.
func (l *LabelPanel) MoveUp() string {
	if l.cursor > 0 {
		l.cursor--
	}
	return l.SelectedLabel()
}

// MoveDown moves the cursor down one entry and returns the newly selected label.
func (l *LabelPanel) MoveDown() string {
	if l.cursor < len(l.entries)-1 {
		l.cursor++
	}
	return l.SelectedLabel()
}

// View renders the label panel as a scrollable list. Entries are variable height
// — a label row plus an optional path row — so the window is measured in rows,
// not entries.
func (l *LabelPanel) View() string {
	var sb strings.Builder

	maxRows := max(l.height, 1)
	used := 0

	for i := l.windowStart(maxRows); i < len(l.entries) && used < maxRows; i++ {
		entry := l.entries[i]

		sb.WriteString(l.renderRow(entry, i == l.cursor))
		sb.WriteString("\n")
		used++

		// Drop trailing path lines rather than the next label when space runs out.
		for _, line := range l.pathLines(entry.Path) {
			if used >= maxRows {
				break
			}
			sb.WriteString(styles.SubtleStyle.Render(pathIndent + line))
			sb.WriteString("\n")
			used++
		}
	}

	return sb.String()
}

// windowStart returns the first entry to render so that the entry under the
// cursor stays visible, filling upwards from it until maxRows is exhausted.
func (l *LabelPanel) windowStart(maxRows int) int {
	if l.cursor < 0 || l.cursor >= len(l.entries) {
		return 0
	}

	start := l.cursor
	used := l.rowsFor(l.entries[l.cursor])
	for i := l.cursor - 1; i >= 0; i-- {
		rows := l.rowsFor(l.entries[i])
		if used+rows > maxRows {
			break
		}
		used += rows
		start = i
	}
	return start
}

// rowsFor returns how many terminal rows an entry occupies: its label row plus
// however many lines its path wraps to.
func (l *LabelPanel) rowsFor(entry LabelPanelEntry) int {
	return 1 + len(l.pathLines(entry.Path))
}

// pathLines returns the wrapped source-path lines shown beneath a label.
func (l *LabelPanel) pathLines(path string) []string {
	if path == "" {
		return nil
	}
	return wrapPath(path, max(l.width-len(pathIndent), 8))
}

// wrapPath breaks a path across lines no wider than width. It splits on path
// separators so each line stays a readable fragment, keeping the separator at
// the end of the line it belongs to; a single segment too long to fit is
// hard-split.
func wrapPath(path string, width int) []string {
	var (
		lines   []string
		current string
	)

	flush := func() {
		if current != "" {
			lines = append(lines, current)
			current = ""
		}
	}

	for _, segment := range strings.SplitAfter(path, "/") {
		if segment == "" {
			continue
		}
		for len(segment) > width {
			flush()
			lines = append(lines, segment[:width])
			segment = segment[width:]
		}
		if len(current)+len(segment) > width {
			flush()
		}
		current += segment
	}
	flush()

	return lines
}

// renderRow renders one row. All row formatting lives here so the panel's
// appearance can be made configurable without touching View or any caller.
func (l *LabelPanel) renderRow(entry LabelPanelEntry, selected bool) string {
	maxLabel := max(l.width-9, 4)

	label := entry.Name
	if label == "" {
		label = allLabelsRow
	}
	if len(label) > maxLabel {
		label = label[:maxLabel-1] + "…"
	}

	line := fmt.Sprintf("%-*s (%d)", maxLabel, label, entry.PlanCount)

	switch {
	case selected && l.focused:
		return styles.ActiveStyle.Render("❯ " + line)
	case selected:
		return styles.MutedStyle.Render("❯ " + line)
	default:
		return styles.InactiveStyle.Render("  " + line)
	}
}
