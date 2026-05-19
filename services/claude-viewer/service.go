// Package claudeviewer implements the Claude Plan Viewer service logic.
package claudeviewer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/cache"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	"github.com/Javier162380/claude-plan-viewer/internal/nowprovider"
	"github.com/Javier162380/claude-plan-viewer/internal/retrier"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

const (
	summaryCacheTTL      = time.Hour
	summaryCacheGCPeriod = 10 * time.Minute
)

// Service represents the Claude Plan Viewer service.
type Service struct {
	db                   dto.Repository
	viewerDir            string
	sourcePlansDir       string
	indexFullContent     bool
	markdownHTMLRendered goldmark.Markdown
	nowProvider          nowprovider.NowProvider
	connectorManager     *connectors.Manager
	watchManager         *WatchManager
	summaryCache         *cache.MuxCache[string]
	logger               *slog.Logger
}

// SetLogger replaces the logger used for non-fatal internal warnings.
// The default logger discards all output; call this to route warnings
// to an appropriate destination (file for TUI, stderr for CLI/MCP).
func (s *Service) SetLogger(l *slog.Logger) {
	s.logger = l
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
		db:                   db,
		viewerDir:            viewerDir,
		sourcePlansDir:       sourcePlansDir,
		indexFullContent:     indexFullContent,
		markdownHTMLRendered: md,
		nowProvider:          nowprovider.SystemTimeProvider{},
		summaryCache:         cache.New[string](summaryCacheTTL, summaryCacheGCPeriod),
		logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Initialize watch manager
	svc.watchManager = NewWatchManager(svc)

	return svc, nil
}

// Close stops background goroutines started by New (cache GC and watch manager).
func (s *Service) Close() {
	if s.watchManager != nil {
		s.watchManager.Stop()
	}
	if s.summaryCache != nil {
		s.summaryCache.Stop()
	}
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
	normalized := NormalizeTags([]string{name})
	if len(normalized) == 0 {
		return dto.Tag{}, fmt.Errorf("invalid tag name: %s", name)
	}

	now := s.nowProvider.Now()
	return s.db.InsertTag(ctx, dto.InsertTagParams{
		Name:        normalized[0],
		Description: description,
		Color:       color,
		CreatedAt:   now,
		ModifiedAt:  now,
	})
}

// GetAllTags returns all tags sorted by name.
func (s *Service) GetAllTags(ctx context.Context) ([]dto.Tag, error) {
	return s.db.ListAllTags(ctx)
}

// GetTagPlanCounts returns a map of tag name → number of plans tagged with it.
func (s *Service) GetTagPlanCounts(ctx context.Context) (map[string]int, error) {
	return s.db.GetTagPlanCounts(ctx)
}

// GetUntaggedPlanCount returns the number of plans with no tags.
func (s *Service) GetUntaggedPlanCount(ctx context.Context) (int64, error) {
	return s.db.GetUntaggedPlanCount(ctx)
}

// ListPlansWithTags returns all plans with their tags reliably populated via a single JOIN query.
// Plans with no tags have an empty Tags slice.
func (s *Service) ListPlansWithTags(ctx context.Context) ([]PlanSummary, error) {
	plans, err := s.db.ListPlansWithTags(ctx)
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}

// BuildTagPlanMap returns a map of tag name → plans carrying that tag.
// Plans with no tags are stored under the empty string key "".
func (s *Service) BuildTagPlanMap(ctx context.Context, sortKey, sortDir string) (map[string][]PlanSummary, error) {
	plans, err := s.ListPlansWithTags(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string][]PlanSummary)
	for _, p := range plans {
		if len(p.Tags) == 0 {
			m[""] = append(m[""], p)
		}
		for _, t := range p.Tags {
			if t.Name != "" {
				m[t.Name] = append(m[t.Name], p)
			}
		}
	}
	for tag := range m {
		sortPlanSummaries(m[tag], sortKey, sortDir)
	}
	return m, nil
}

// ListUntaggedPlansWithReadingTime returns plans with no tags, including reading time.
func (s *Service) ListUntaggedPlansWithReadingTime(ctx context.Context, sortKey, sortDir string) ([]PlanSummary, error) {
	plans, err := s.db.ListUntaggedPlans(ctx, sortKeyToColumn(sortKey), sortDir)
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}

// sortPlanSummaries sorts a slice of PlanSummary in place by the given sort key and direction.
func sortPlanSummaries(plans []PlanSummary, key, dir string) {
	desc := dir != SortDirAsc
	sort.Slice(plans, func(i, j int) bool {
		switch key {
		case SortKeyCreatedAt:
			if desc {
				return plans[i].CreatedAt.After(plans[j].CreatedAt)
			}
			return plans[i].CreatedAt.Before(plans[j].CreatedAt)
		case SortKeyReadingTime:
			if desc {
				return plans[i].ReadingTime > plans[j].ReadingTime
			}
			return plans[i].ReadingTime < plans[j].ReadingTime
		case SortKeySize:
			if desc {
				return plans[i].FileSize > plans[j].FileSize
			}
			return plans[i].FileSize < plans[j].FileSize
		default: // updated_at
			if desc {
				return plans[i].ModifiedAt.After(plans[j].ModifiedAt)
			}
			return plans[i].ModifiedAt.Before(plans[j].ModifiedAt)
		}
	})
}

// DeleteTag deletes a tag by ID. This will also remove all plan-tag associations
// within a transaction (first removes associations, then the tag).
func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	return s.db.DeleteTag(ctx, id)
}

// resolveTagIDs converts tag names to IDs, creating any that don't exist.
// Retries handle the concurrent sync race where two goroutines try to create the same tag simultaneously.
func (s *Service) resolveTagIDs(ctx context.Context, tagNames []string, now time.Time) ([]int64, error) {
	r := retrier.NewRetrier(3, 50*time.Millisecond)
	tagIDs := make([]int64, 0, len(tagNames))

	for _, tagName := range tagNames {
		var tag dto.Tag
		err := r.Do(ctx, func(ctx context.Context) error {
			t, err := s.db.GetTagByName(ctx, tagName)
			if dto.IsNotFound(err) {
				t, err = s.db.InsertTag(ctx, dto.InsertTagParams{
					Name:       tagName,
					CreatedAt:  now,
					ModifiedAt: now,
				})
			}
			if err != nil {
				return err
			}
			tag = t
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get or create tag %s: %w", tagName, err)
		}
		tagIDs = append(tagIDs, tag.ID)
	}
	return tagIDs, nil
}

// SetPlanTags sets the tags for a plan, replacing any existing tags.
// Tag names will be normalized (lowercase, trimmed).
// If a tag doesn't exist, it will be created automatically.
func (s *Service) SetPlanTags(ctx context.Context, fileName string, tagNames []string) error {
	tagNames = NormalizeTags(tagNames)

	plan, err := s.db.GetPlanByFileName(ctx, fileName)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	now := s.nowProvider.Now()
	tagIDs, err := s.resolveTagIDs(ctx, tagNames, now)
	if err != nil {
		return err
	}

	return s.db.SetPlanTags(ctx, plan.ID, tagIDs, now)
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
