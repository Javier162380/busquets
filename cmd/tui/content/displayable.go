// Package content provides interfaces and types for displayable content in the TUI.
package content

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"charm.land/glamour/v2"
)

const (
	GlamourDarkMode  = "dark"
	GlamourTokyoMode = "tokyo-night"
)

// Displayable is the interface for content that can be displayed in the viewer.
// Both PlanDetail and PlanVersionDetail implement this interface.
type Displayable interface {
	GetTitle() string
	GetContent() string
	GetRenderedHTML() string
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
	*claudeviewer.PlanDetail
	darkModeEnabled bool
	focus           types.Focus
	width           int
}

// NewPlanContent creates a new PlanContent from a PlanDetail.
func NewPlanContent(plan *claudeviewer.PlanDetail, darkModeEnabled bool, focus types.Focus, width int) *PlanContent {
	return &PlanContent{PlanDetail: plan, darkModeEnabled: darkModeEnabled, focus: focus, width: width}
}

func (p *PlanContent) GetTitle() string {
	return p.Title
}

func (p *PlanContent) GetContent() string {
	return p.Content
}

func (p *PlanContent) GetRenderedHTML() string {
	return renderMarkdown(p.Content, p.darkModeEnabled, p.width)
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
	*claudeviewer.PlanVersionDetail
	darkModeEnabled bool
	focus           types.Focus
	width           int
}

// NewVersionContent creates a new VersionContent from a PlanVersionDetail.
func NewVersionContent(version *claudeviewer.PlanVersionDetail, darkModeEnabled bool, focus types.Focus, width int) *VersionContent {
	return &VersionContent{PlanVersionDetail: version, darkModeEnabled: darkModeEnabled, focus: focus, width: width}
}

func (v *VersionContent) GetTitle() string {
	return v.FilePath
}

func (v *VersionContent) GetContent() string {
	return v.Content
}

func (v *VersionContent) GetRenderedHTML() string {
	return renderMarkdown(v.Content, v.darkModeEnabled, v.width)
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

// RenderMarkdown renders markdown text using glamour with the current theme.
func RenderMarkdown(content string, darkModeEnabled bool, width int) string {
	return renderMarkdown(content, darkModeEnabled, width)
}

func renderMarkdown(content string, darkModeEnabled bool, width int) string {
	style := GlamourDarkMode
	if darkModeEnabled {
		style = GlamourTokyoMode
	}
	// Use the actual width provided, or default to 40 if width is too small
	wordWidth := width
	if wordWidth < 40 {
		wordWidth = 40
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(wordWidth),
	)
	rendered, _ := r.Render(content)
	return rendered
}
