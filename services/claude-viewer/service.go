// Package claudeviewer implements the Claude Plan Viewer service logic.
package claudeviewer

import (
	"context"
	"fmt"
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
	watchManager     *WatchManager
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

	svc := &Service{
		db:               db,
		viewerDir:        viewerDir,
		sourcePlansDir:   sourcePlansDir,
		indexFullContent: indexFullContent,
		markdown:         md,
		nowProvider:      systemTimeProvider{},
	}

	// Initialize watch manager
	svc.watchManager = NewWatchManager(svc)

	return svc, nil
}

// StartWatchMode enables background syncing at the specified interval.
func (s *Service) StartWatchMode(ctx context.Context, intervalSeconds float64) error {
	if intervalSeconds <= 0 {
		intervalSeconds = 5 // default
	}
	interval := time.Duration(intervalSeconds) * time.Second

	// Persist to settings
	trueVal := true
	if err := s.SetSetting(ctx, SettingWatchModeEnabled, SettingValues{BooleanValue: &trueVal}); err != nil {
		return fmt.Errorf("failed to save watch mode setting: %w", err)
	}
	if err := s.SetSetting(ctx, SettingWatchIntervalSeconds, SettingValues{NumberValue: &intervalSeconds}); err != nil {
		return fmt.Errorf("failed to save interval setting: %w", err)
	}

	return s.watchManager.Start(ctx, interval)
}

// StopWatchMode disables background syncing.
func (s *Service) StopWatchMode(ctx context.Context) error {
	s.watchManager.Stop()

	// Persist to settings
	falseVal := false
	return s.SetSetting(ctx, SettingWatchModeEnabled, SettingValues{BooleanValue: &falseVal})
}

// GetWatchResultChannel returns the channel for receiving watch sync results.
func (s *Service) GetWatchResultChannel() <-chan WatchResult {
	return s.watchManager.ResultChannel()
}

// IsWatchModeRunning returns true if watch mode is currently active.
func (s *Service) IsWatchModeRunning() bool {
	return s.watchManager.IsRunning()
}

// UpdateWatchInterval changes the sync interval (takes effect on next tick).
func (s *Service) UpdateWatchInterval(intervalSeconds float64) {
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}
	interval := time.Duration(intervalSeconds) * time.Second
	s.watchManager.UpdateInterval(interval)
}
