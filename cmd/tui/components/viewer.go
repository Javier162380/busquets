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
	RenderModeHTML
)

// Viewer displays scrollable content.
type Viewer struct {
	viewport   viewport.Model
	content    content.Displayable
	renderMode RenderMode
	width      int
	height     int
}

// NewViewer creates a new viewer component.
func NewViewer(width, height int) *Viewer {
	vp := viewport.New(width, height)
	vp.SetContent("")

	return &Viewer{
		viewport:   vp,
		width:      width,
		height:     height,
		renderMode: RenderModeRaw,
	}
}

// SetContent sets the content to display.
func (v *Viewer) SetContent(c content.Displayable) {
	v.content = c
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
	if v.renderMode == RenderModeRaw {
		v.renderMode = RenderModeHTML
	} else {
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

	// Add title.
	lines = append(lines, styles.TitleStyle.Render(v.content.GetTitle()))

	// Add metadata.
	meta := v.content.GetMetadata()
	metaLine := fmt.Sprintf("%s: %s | Reading Time: %d min | %s",
		meta.PrimaryLabel,
		meta.PrimaryTime.Format("2006-01-02 15:04"),
		v.content.GetReadingTime(),
		meta.SecondaryInfo)
	lines = append(lines, styles.MetaStyle.Render(metaLine))
	lines = append(lines, "")

	// Add content based on render mode.
	var contentText string
	if v.renderMode == RenderModeHTML {
		contentText = stripHTMLTags(v.content.GetRenderedHTML())
	} else {
		contentText = v.content.GetContent()
	}

	lines = append(lines, strings.Split(contentText, "\n")...)

	v.viewport.SetContent(strings.Join(lines, "\n"))
}

// stripHTMLTags removes HTML tags from a string, leaving only the content.
func stripHTMLTags(html string) string {
	replacements := map[string]string{
		"<p>":           "",
		"</p>":          "\n",
		"<br>":          "\n",
		"<br/>":         "\n",
		"<br />":        "\n",
		"<strong>":      "",
		"</strong>":     "",
		"<b>":           "",
		"</b>":          "",
		"<em>":          "",
		"</em>":         "",
		"<i>":           "",
		"</i>":          "",
		"<u>":           "",
		"</u>":          "",
		"<code>":        "",
		"</code>":       "",
		"<pre>":         "",
		"</pre>":        "",
		"<h1>":          "\n",
		"</h1>":         "\n",
		"<h2>":          "\n",
		"</h2>":         "\n",
		"<h3>":          "\n",
		"</h3>":         "\n",
		"<h4>":          "\n",
		"</h4>":         "\n",
		"<h5>":          "\n",
		"</h5>":         "\n",
		"<h6>":          "\n",
		"</h6>":         "\n",
		"<ul>":          "",
		"</ul>":         "",
		"<ol>":          "",
		"</ol>":         "",
		"<li>":          "  - ",
		"</li>":         "\n",
		"<table>":       "",
		"</table>":      "",
		"<tr>":          "",
		"</tr>":         "\n",
		"<td>":          "",
		"</td>":         " | ",
		"<th>":          "",
		"</th>":         " | ",
		"<thead>":       "",
		"</thead>":      "",
		"<tbody>":       "",
		"</tbody>":      "",
		"<blockquote>":  "> ",
		"</blockquote>": "\n",
		"<div>":         "",
		"</div>":        "\n",
		"<span>":        "",
		"</span>":       "",
		"</a>":          "",
		"&lt;":          "<",
		"&gt;":          ">",
		"&amp;":         "&",
		"&quot;":        "\"",
		"&#39;":         "'",
		"<hr>":          "---",
		"<hr/>":         "---",
		"<hr />":        "---",
	}

	result := html
	for tag, replacement := range replacements {
		result = strings.ReplaceAll(result, tag, replacement)
	}

	// Remove any remaining tags (like <a href="...">).
	// Simple approach: just remove them.
	for strings.Contains(result, "<") && strings.Contains(result, ">") {
		start := strings.Index(result, "<")
		end := strings.Index(result, ">")
		if start < end {
			result = result[:start] + result[end+1:]
		} else {
			break
		}
	}

	return result
}
