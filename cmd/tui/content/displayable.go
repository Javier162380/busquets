// Package content provides interfaces and types for displayable content in the TUI.
package content

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Javier162380/busquets/services/busquets"

	"charm.land/glamour/v2"
)

// Displayable is the interface for content that can be displayed in the viewer.
// Both PlanDetail and PlanVersionDetail implement this interface.
//
//go:generate mockgen -package content_test -destination ./test/displayable_stub.go . Displayable
type Displayable interface {
	GetTitle() string
	GetContent() string
	GetRenderedHTML(markdownTheme string) string
	GetReadingTime() int
	GetMetadata() Metadata
	GetIdentifier() string
}

// Metadata contains display metadata for content.
type Metadata struct {
	PrimaryLabel    string
	PrimaryTime     time.Time
	SecondaryInfo   string
	TagNames        []string
	SourcePath      string
	DestinationPath string
}

// PlanContent wraps PlanDetail to implement Displayable.
type PlanContent struct {
	*busquets.PlanDetail
	width int
}

// NewPlanContent creates a new PlanContent from a PlanDetail.
func NewPlanContent(plan *busquets.PlanDetail, width int) *PlanContent {
	return &PlanContent{PlanDetail: plan, width: width}
}

func (p *PlanContent) GetTitle() string {
	return p.Title
}

func (p *PlanContent) GetContent() string {
	return p.Content
}

func (p *PlanContent) GetRenderedHTML(markdownTheme string) string {
	return renderMarkdown(p.Content, markdownTheme, p.width)
}

func (p *PlanContent) GetReadingTime() int {
	return p.ReadingTime
}

func (p *PlanContent) GetMetadata() Metadata {
	tagNames := make([]string, len(p.Tags))
	for i, tag := range p.Tags {
		tagNames[i] = tag.Name
	}
	return Metadata{
		PrimaryLabel:    "Modified",
		PrimaryTime:     p.ModifiedAt,
		SecondaryInfo:   fmt.Sprintf("Size: %d bytes", p.FileSize),
		TagNames:        tagNames,
		SourcePath:      filepath.Join(p.SyncSource, p.FileName),
		DestinationPath: p.FilePath,
	}
}

func (p *PlanContent) GetIdentifier() string {
	return p.FileName
}

// VersionContent wraps PlanVersionDetail to implement Displayable.
type VersionContent struct {
	*busquets.PlanVersionDetail
	width int
}

// NewVersionContent creates a new VersionContent from a PlanVersionDetail.
func NewVersionContent(version *busquets.PlanVersionDetail, width int) *VersionContent {
	return &VersionContent{PlanVersionDetail: version, width: width}
}

func (v *VersionContent) GetTitle() string {
	return v.FilePath
}

func (v *VersionContent) GetContent() string {
	return v.Content
}

func (v *VersionContent) GetRenderedHTML(markdownTheme string) string {
	return renderMarkdown(v.Content, markdownTheme, v.width)
}

func (v *VersionContent) GetReadingTime() int {
	return v.ReadingTime
}

func (v *VersionContent) GetMetadata() Metadata {
	tagNames := make([]string, len(v.Tags))
	for i, tag := range v.Tags {
		tagNames[i] = tag.Name
	}
	return Metadata{
		PrimaryLabel:  "Created",
		PrimaryTime:   v.CreatedAt,
		SecondaryInfo: fmt.Sprintf("Version: %d", v.VersionNumber),
		SourcePath:    v.PlanVersion.FilePath,
		TagNames:      tagNames,
	}
}

func (v *VersionContent) GetIdentifier() string {
	return fmt.Sprintf("%s@v%d", v.FilePath, v.VersionNumber)
}

// MemoryContent wraps a PlanMemory to implement Displayable.
//
// A memory with nothing written yet still displays: the timeline beside it is
// computed, so the viewer shows a prompt to generate rather than an empty pane.
type MemoryContent struct {
	*busquets.PlanMemory
	width int
}

// NewMemoryContent creates a MemoryContent from a PlanMemory.
func NewMemoryContent(memory *busquets.PlanMemory, width int) *MemoryContent {
	return &MemoryContent{PlanMemory: memory, width: width}
}

func (m *MemoryContent) GetTitle() string {
	return fmt.Sprintf("Memory: %s", m.PlanTitle)
}

func (m *MemoryContent) GetContent() string {
	if !m.Exists() {
		return fmt.Sprintf(
			"# Memory: %s\n\nNo memory has been written yet.\n\n"+
				"The timeline beside this pane is computed from the plan's versions and "+
				"comments, so it is already accurate. Press `r` to write the narrative "+
				"for it using the configured summary connector.\n",
			m.PlanTitle)
	}
	return m.Content
}

func (m *MemoryContent) GetRenderedHTML(markdownTheme string) string {
	return renderMarkdown(m.GetContent(), markdownTheme, m.width)
}

func (m *MemoryContent) GetReadingTime() int {
	return busquets.CalculateReadingTime(busquets.CountWords(m.GetContent()))
}

func (m *MemoryContent) GetMetadata() Metadata {
	label, when := "Updated", m.UpdatedAt
	secondary := fmt.Sprintf("Covers up to v%d | by %s", m.CoversUpToVersion, m.GeneratedBy)

	if !m.Exists() {
		// There is no UpdatedAt yet, and the viewer always formats the time, so
		// a zero value would render as 0001-01-01. Show the newest event instead.
		label, secondary = "Last event", "not generated yet"
		if count := len(m.Events); count > 0 {
			when = m.Events[count-1].OccurredAt
		}
	}

	return Metadata{
		PrimaryLabel:    label,
		PrimaryTime:     when,
		SecondaryInfo:   secondary,
		SourcePath:      filepath.Join(m.SyncSource, m.FileName),
		DestinationPath: m.FilePath,
	}
}

func (m *MemoryContent) GetIdentifier() string {
	return fmt.Sprintf("%s@memory", m.FileName)
}

// RenderMarkdown renders markdown text using glamour with the given theme.
func RenderMarkdown(content, markdownTheme string, width int) string {
	return renderMarkdown(content, markdownTheme, width)
}

// renderMarkdown renders content with the named glamour style, falling back to
// the default theme and then to the unrendered text. glamour returns a nil
// renderer alongside its error for an empty or unknown style name, so these
// errors cannot be discarded.
func renderMarkdown(content, markdownTheme string, width int) string {
	// Use the actual width provided, or default to 40 if width is too small
	wordWidth := width
	if wordWidth < 40 {
		wordWidth = 40
	}

	r, err := newTermRenderer(markdownTheme, wordWidth)
	if err != nil {
		r, err = newTermRenderer(busquets.DefaultMarkdownTheme, wordWidth)
		if err != nil {
			return content
		}
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content
	}
	return rendered
}

func newTermRenderer(markdownTheme string, wordWidth int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle(markdownTheme),
		glamour.WithWordWrap(wordWidth),
	)
}
