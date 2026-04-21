// Package content provides interfaces and types for displayable content in the TUI.
package content

import (
	"fmt"
	"time"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
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
	PrimaryLabel  string
	PrimaryTime   time.Time
	SecondaryInfo string
	TagNames      []string
}

// PlanContent wraps PlanDetail to implement Displayable.
type PlanContent struct {
	*claudeviewer.PlanDetail
}

// NewPlanContent creates a new PlanContent from a PlanDetail.
func NewPlanContent(plan *claudeviewer.PlanDetail) *PlanContent {
	return &PlanContent{PlanDetail: plan}
}

func (p *PlanContent) GetTitle() string {
	return p.Title
}

func (p *PlanContent) GetContent() string {
	return p.Content
}

func (p *PlanContent) GetRenderedHTML() string {
	return p.RenderedHTML
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
		PrimaryLabel:  "Modified",
		PrimaryTime:   p.ModifiedAt,
		SecondaryInfo: fmt.Sprintf("Size: %d bytes", p.FileSize),
		TagNames:      tagNames,
	}
}

func (p *PlanContent) GetIdentifier() string {
	return p.FileName
}

// VersionContent wraps PlanVersionDetail to implement Displayable.
type VersionContent struct {
	*claudeviewer.PlanVersionDetail
}

// NewVersionContent creates a new VersionContent from a PlanVersionDetail.
func NewVersionContent(version *claudeviewer.PlanVersionDetail) *VersionContent {
	return &VersionContent{PlanVersionDetail: version}
}

func (v *VersionContent) GetTitle() string {
	return v.FilePath
}

func (v *VersionContent) GetContent() string {
	return v.Content
}

func (v *VersionContent) GetRenderedHTML() string {
	return v.RenderedHTML
}

func (v *VersionContent) GetReadingTime() int {
	return v.ReadingTime
}

func (v *VersionContent) GetMetadata() Metadata {
	return Metadata{
		PrimaryLabel:  "Created",
		PrimaryTime:   v.CreatedAt,
		SecondaryInfo: fmt.Sprintf("Version: %d", v.VersionNumber),
	}
}

func (v *VersionContent) GetIdentifier() string {
	return fmt.Sprintf("%s@v%d", v.FilePath, v.VersionNumber)
}
