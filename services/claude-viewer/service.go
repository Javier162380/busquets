// Package claudeviewer implements the Claude Plan Viewer service logic.
package claudeviewer

import (
	"context"
	_ "embed"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed sqlc/schema.sql
var schema string

type NowProvider interface {
	Now() time.Time
}

type systemTimeProvider struct{}

func (s systemTimeProvider) Now() time.Time {
	return time.Now()
}

// Service represents the Claude Plan Viewer service.
type Service struct {
	db               *repository.Queries
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

// DB returns the database queries instance for external use.
func (s *Service) DB() *repository.Queries {
	return s.db
}

// NewRepository creates a new repository with database initialization.
func NewRepository(ctx context.Context, dbPath string) (*repository.Queries, error) {
	db, err := storage.NewSQLiteClient(ctx, dbPath, &schema)
	if err != nil {
		return nil, err
	}
	return repository.New(db), nil
}

// New creates a new Claude Plan Viewer service instance.
func New(db *repository.Queries, viewerDir, sourcePlansDir string, indexFullContent bool) (*Service, error) {
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
