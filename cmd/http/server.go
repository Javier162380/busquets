// Package http contains the HTTP server for Claude Plan Viewer.
package http

import (
	"embed"
	"html/template"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/labstack/echo/v4"
)

//go:embed templates
var templates embed.FS

//go:embed static
var static embed.FS

// Server handles HTTP requests for plan viewing.
type Server struct {
	service   *claudeviewer.Service
	echo      *echo.Echo
	templates *template.Template
}

// NewServer creates a new HTTP server.
func NewServer(service *claudeviewer.Service, addr string) (*Server, error) {
	e := echo.New()
	e.HidePort = true
	e.HideBanner = true

	// Parse and cache templates during initialization
	tmpl, err := parseTemplates()
	if err != nil {
		return nil, err
	}

	return &Server{
		service:   service,
		echo:      e,
		templates: tmpl,
	}, nil
}

// parseTemplates loads and parses all templates once during initialization.
func parseTemplates() (*template.Template, error) {
	tmpl := template.New("")

	// Parse index.html
	indexHTML, err := templates.ReadFile("templates/index.html")
	if err != nil {
		return nil, err
	}
	if _, err := tmpl.New("index").Parse(string(indexHTML)); err != nil {
		return nil, err
	}

	// Parse view.html
	viewHTML, err := templates.ReadFile("templates/view.html")
	if err != nil {
		return nil, err
	}
	if _, err := tmpl.New("view").Parse(string(viewHTML)); err != nil {
		return nil, err
	}

	// Parse versions.html for version history
	versionsHTML, err := templates.ReadFile("templates/versions.html")
	if err != nil {
		return nil, err
	}
	if _, err := tmpl.New("versions").Parse(string(versionsHTML)); err != nil {
		return nil, err
	}

	// Parse plan-list-partial.html for HTMX pagination
	partialHTML, err := templates.ReadFile("templates/plan-list-partial.html")
	if err != nil {
		return nil, err
	}
	if _, err := tmpl.New("plan-list-partial").Parse(string(partialHTML)); err != nil {
		return nil, err
	}

	return tmpl, nil
}

// Server returns the Echo server with routes registered.
func (s *Server) Server() *echo.Echo {
	s.echo.GET("/", s.handleIndex)
	s.echo.GET("/plan/:filename", s.handleViewPlan)
	s.echo.GET("/plan/:planName/versions", s.handleViewPlanVersions)
	s.echo.POST("/plan/:filename", s.handleUpdatePlan)
	s.echo.POST("/plan/:filename/save-local", s.handleSavePlanLocal)
	s.echo.POST("/sync", s.handleSync)
	s.echo.GET("/api/setting/:variableName", s.handleSetting)
	s.echo.POST("/api/setting/:variableName", s.handleSetting)
	s.echo.GET("/api/plans", s.handleListPlansWithPagination)

	// Version control API endpoints
	s.echo.GET("/api/plan/:planName/versions", s.handleGetPlanVersionHistory)
	s.echo.GET("/api/plan/:planName/versions/:versionNumber", s.handleGetPlanVersion)
	s.echo.POST("/api/plan/:planName/restore/:versionNumber", s.handleRestorePlanVersion)
	s.echo.GET("/api/plan/:planName/versions/search", s.handleSearchVersions)

	// Serve static assets
	s.echo.FileFS("/static", "static", static)

	return s.echo
}
