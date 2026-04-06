// Package claudeviewer implements the Claude Plan Viewer service logic.
package claudeviewer

import (
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

type NowProvider interface {
	Now() time.Time
}

type systemTimeProvider struct{}

func (s systemTimeProvider) Now() time.Time {
	return time.Now()
}

// Service represents the Claude Plan Viewer service.
type Service struct {
	db               dto.Repository
	viewerDir        string
	sourcePlansDir   string
	indexFullContent bool
	markdown         goldmark.Markdown
	nowProvider      NowProvider
	connectorManager *connectors.Manager
}

// SetConnectorManager sets the connector manager for the service.
func (s *Service) SetConnectorManager(manager *connectors.Manager) {
	s.connectorManager = manager
}

// DB returns the database repository for external use.
func (s *Service) DB() dto.Repository {
	return s.db
}

// New creates a new Claude Plan Viewer service instance.
// The db parameter must implement dto.Repository (sqlite.Repository or postgres.Repository).
func New(db dto.Repository, viewerDir, sourcePlansDir string, indexFullContent bool) (*Service, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	return &Service{
		db:               db,
		viewerDir:        viewerDir,
		sourcePlansDir:   sourcePlansDir,
		indexFullContent: indexFullContent,
		markdown:         md,
		nowProvider:      systemTimeProvider{},
	}, nil
}
