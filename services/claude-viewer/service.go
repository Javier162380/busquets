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

// Tag management methods

// CreateTag creates a new tag with the given name, description, and color.
func (s *Service) CreateTag(ctx context.Context, name string, description, color *string) (dto.Tag, error) {
	// Normalize tag name
	normalized := NormalizeTags([]string{name})
	if len(normalized) == 0 {
		return dto.Tag{}, fmt.Errorf("invalid tag name: %s", name)
	}

	return s.db.InsertTag(ctx, dto.InsertTagParams{
		Name:        normalized[0],
		Description: description,
		Color:       color,
	})
}

// GetAllTags returns all tags sorted by name.
func (s *Service) GetAllTags(ctx context.Context) ([]dto.Tag, error) {
	return s.db.ListAllTags(ctx)
}

// DeleteTag deletes a tag by ID. This will also remove all plan-tag associations
// within a transaction (first removes associations, then the tag).
func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	return s.db.DeleteTag(ctx, id)
}

// SetPlanTags sets the tags for a plan, replacing any existing tags.
// Tag names will be normalized (lowercase, trimmed).
// If a tag doesn't exist, it will be created automatically.
func (s *Service) SetPlanTags(ctx context.Context, fileName string, tagNames []string) error {
	// Normalize tag names
	tagNames = NormalizeTags(tagNames)

	// Get plan by filename
	plan, err := s.db.GetPlanByFileName(ctx, fileName)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Get or create tags and collect IDs
	tagIDs := make([]int64, 0, len(tagNames))
	for _, tagName := range tagNames {
		tag, err := s.db.GetTagByName(ctx, tagName)
		if dto.IsNotFound(err) {
			// Create new tag
			tag, err = s.db.InsertTag(ctx, dto.InsertTagParams{
				Name: tagName,
			})
		}
		if err != nil {
			return fmt.Errorf("failed to get or create tag %s: %w", tagName, err)
		}
		tagIDs = append(tagIDs, tag.ID)
	}

	// Set plan tags (atomic operation)
	return s.db.SetPlanTags(ctx, plan.ID, tagIDs)
}

// GetPlanTags returns all tags associated with a plan.
func (s *Service) GetPlanTags(ctx context.Context, fileName string) ([]dto.Tag, error) {
	// Get plan by filename
	plan, err := s.db.GetPlanByFileName(ctx, fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	return s.db.GetPlanTags(ctx, plan.ID)
}
