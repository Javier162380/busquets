package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// RenderMode determines how content is displayed.
type RenderMode int

const (
	RenderModeRaw RenderMode = iota
	RenderModeGlamour
)

// Viewer displays scrollable content.
type Viewer struct {
	viewport        viewport.Model
	content         content.Displayable
	renderMode      RenderMode
	markdownTheme   string
	showLineNumbers bool
	headerLineCount int
	totalLineCount  int
	width           int
	height          int
}

// NewViewer creates a new viewer component.
func NewViewer(width, height int, markdownRenderedTheme string) *Viewer {
	vp := viewport.New(width, height)
	vp.SetContent("")

	return &Viewer{
		viewport:      vp,
		width:         width,
		height:        height,
		renderMode:    RenderModeRaw,
		markdownTheme: markdownRenderedTheme,
	}
}

// SetContent sets the content to display.
func (v *Viewer) SetContent(c content.Displayable) {
	v.content = c
	v.updateViewportContent()
}

func (v *Viewer) SetRenderMode(m RenderMode) { v.renderMode = m }

func (v *Viewer) GetRenderMode() RenderMode {
	return v.renderMode
}

// SetMarkdownTheme changes the glamour theme and re-renders.
func (v *Viewer) SetMarkdownTheme(theme string) {
	v.markdownTheme = theme
	v.updateViewportContent()
}

// SetSize updates the viewer dimensions.
func (v *Viewer) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.viewport.Width = width
	v.viewport.Height = height
	v.updateViewportContent()
}

// ToggleRenderMode switches between raw and HTML rendering.
func (v *Viewer) ToggleRenderMode() {
	switch v.renderMode {
	case RenderModeRaw:
		v.renderMode = RenderModeGlamour
	case RenderModeGlamour:
		v.renderMode = RenderModeRaw
	}
	v.updateViewportContent()
}

// RenderMode returns the current render mode.
func (v *Viewer) RenderMode() RenderMode {
	return v.renderMode
}

// ToggleLineNumbers switches the line-number gutter on/off.
func (v *Viewer) ToggleLineNumbers() {
	v.showLineNumbers = !v.showLineNumbers
	v.updateViewportContent()
}

// ShowLineNumbers returns whether the line-number gutter is currently shown.
func (v *Viewer) ShowLineNumbers() bool {
	return v.showLineNumbers
}

// Update handles viewport updates.
func (v *Viewer) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.viewport, cmd = v.viewport.Update(msg)
	return cmd
}

// View renders the viewer.
func (v *Viewer) View() string {
	return v.viewport.View()
}

// GotoTop scrolls to the top.
func (v *Viewer) GotoTop() {
	v.viewport.GotoTop()
}

// GotoBottom scrolls to the bottom.
func (v *Viewer) GotoBottom() {
	v.viewport.GotoBottom()
}

// ScrollPercent returns the current scroll percentage.
func (v *Viewer) ScrollPercent() float64 {
	return v.viewport.ScrollPercent()
}

// CurrentContentLine returns the 1-indexed source line currently scrolled to
// the top of the viewport. In raw mode this is exact — each viewer line maps
// 1:1 to a line in the underlying markdown source. In Glamour mode, rendering
// reflows the content (headers, lists, wrapping change line counts), so
// there's no exact mapping back to source lines; this instead approximates
// it from the viewport's scroll percentage against the raw line count.
func (v *Viewer) CurrentContentLine() int {
	if v.content == nil {
		return 1
	}
	if v.renderMode == RenderModeRaw {
		line := v.viewport.YOffset - v.headerLineCount + 1
		if line < 1 {
			line = 1
		}
		return line
	}
	return v.lineFromScrollPercent(v.viewport.ScrollPercent())
}

// ScrollToContentLine scrolls the viewport so the given 1-indexed source
// line sits at the top, mirroring CurrentContentLine. In raw mode this is
// exact. In Glamour mode, since there's no exact source-line mapping, it
// approximates by converting the target line's fraction of the raw file into
// the equivalent scroll position in the rendered view.
func (v *Viewer) ScrollToContentLine(line int) {
	if v.content == nil {
		return
	}
	if line < 1 {
		line = 1
	}
	if v.renderMode == RenderModeRaw {
		v.viewport.SetYOffset(v.headerLineCount + line - 1)
		return
	}

	total := v.rawLineCount()
	percent := 0.0
	if total > 1 {
		percent = float64(line-1) / float64(total-1)
	}
	maxOffset := v.totalLineCount - v.viewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	v.viewport.SetYOffset(int(percent*float64(maxOffset) + 0.5))
}

// rawLineCount returns the number of lines in the underlying markdown
// source, independent of how it's currently rendered.
func (v *Viewer) rawLineCount() int {
	return len(strings.Split(v.content.GetContent(), "\n"))
}

// lineFromScrollPercent maps a viewport scroll percentage onto an
// approximate 1-indexed line in the raw source.
func (v *Viewer) lineFromScrollPercent(percent float64) int {
	total := v.rawLineCount()
	line := int(percent*float64(total-1)+0.5) + 1
	if line < 1 {
		line = 1
	}
	if line > total {
		line = total
	}
	return line
}

// updateViewportContent updates the viewport with formatted content.
func (v *Viewer) updateViewportContent() {
	if v.content == nil {
		v.viewport.SetContent("No content to display")
		return
	}

	var lines []string

	// Add metadata.
	meta := v.content.GetMetadata()
	metaLine := fmt.Sprintf("%s: %s | Reading Time: %d min | %s",
		meta.PrimaryLabel,
		meta.PrimaryTime.Format("2006-01-02 15:04"),
		v.content.GetReadingTime(),
		meta.SecondaryInfo)
	if len(meta.TagNames) <= 2 && len(meta.TagNames) > 0 {
		metaLine += fmt.Sprintf(" | Tags: %s", strings.Join(meta.TagNames, ","))
	}
	if len(meta.TagNames) > 2 {
		metaLine += fmt.Sprintf(" | Tags: %s+%d", strings.Join(meta.TagNames[:2], ","), len(meta.TagNames)-2)
	}
	lines = append(lines, styles.MetaStyle.Render(metaLine))

	if meta.SourcePath != "" {
		lines = append(lines, styles.MetaStyle.Render("Source Path: "+meta.SourcePath))
	}
	if meta.DestinationPath != "" {
		lines = append(lines, styles.MetaStyle.Render("Destination Path: "+meta.DestinationPath))
	}

	lines = append(lines, "")
	v.headerLineCount = len(lines)

	// Add content based on render mode.
	var contentText string
	if v.renderMode == RenderModeGlamour {
		contentText = v.content.GetRenderedHTML(v.markdownTheme)
	} else {
		contentText = v.content.GetContent()
	}

	contentLines := strings.Split(contentText, "\n")
	if v.showLineNumbers {
		contentLines = numberLines(contentLines)
	}
	lines = append(lines, contentLines...)
	v.totalLineCount = len(lines)

	v.viewport.SetContent(strings.Join(lines, "\n"))
}

// numberLines prefixes each line with a right-aligned line number, matching
// the gutter style of the editor's textarea.
func numberLines(contentLines []string) []string {
	width := len(strconv.Itoa(len(contentLines)))
	numbered := make([]string, len(contentLines))
	for i, line := range contentLines {
		gutter := styles.LineNumberStyle.Render(fmt.Sprintf("%*d │ ", width, i+1))
		numbered[i] = gutter + line
	}
	return numbered
}
