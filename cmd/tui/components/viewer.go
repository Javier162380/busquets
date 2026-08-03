package components

import (
	"fmt"
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
	viewport      viewport.Model
	content       content.Displayable
	renderMode    RenderMode
	markdownTheme string
	width         int
	height        int
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

	// Glamour output is ANSI, not HTML — used verbatim. Stripping "tags" from it
	// would eat literal angle brackets it preserved (`Vec<String>`, `a < b`).
	var contentText string
	if v.renderMode == RenderModeGlamour {
		contentText = v.content.GetRenderedHTML(v.markdownTheme)
	} else {
		contentText = v.content.GetContent()
	}

	lines = append(lines, strings.Split(contentText, "\n")...)

	v.viewport.SetContent(strings.Join(lines, "\n"))
}
