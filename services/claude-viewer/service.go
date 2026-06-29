// Package claudeviewer implements the Claude Plan Viewer service logic.
package claudeviewer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/cache"
	"github.com/Javier162380/claude-plan-viewer/internal/config"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	"github.com/Javier162380/claude-plan-viewer/internal/nowprovider"
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
	sourcePlansDirs      []config.SyncDir
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
func New(db dto.Repository, viewerDir string, syncDirs []config.SyncDir, indexFullContent bool) (*Service, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	svc := &Service{
		db:                   db,
		viewerDir:            viewerDir,
		sourcePlansDirs:      syncDirs,
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

// LabelForSource maps a stored sync_source path to a display label.
// Falls back to the last path component if no configured label matches.
func (s *Service) LabelForSource(syncSource string) string {
	return s.labelForSource(syncSource)
}

// SourcePathForLabel returns the sync_source path for a given label.
// Mirrors the fallback in labelForSource: a dir with no explicit label is
// matched by filepath.Base(path). Returns empty string if not found.
func (s *Service) SourcePathForLabel(label string) string {
	for _, d := range s.sourcePlansDirs {
		effective := d.Label
		if effective == "" {
			effective = filepath.Base(d.Path)
		}
		if effective == label {
			return d.Path
		}
	}
	return ""
}

func (s *Service) labelForSource(syncSource string) string {
	for _, d := range s.sourcePlansDirs {
		if d.Path == syncSource {
			if d.Label != "" {
				return d.Label
			}
			return filepath.Base(syncSource)
		}
	}
	return filepath.Base(syncSource)
}

// viewerSubdirFor returns the viewer subdirectory label (slugified) for a sync source path.
func (s *Service) viewerSubdirFor(syncSource string) string {
	label := s.labelForSource(syncSource)
	return slugify(label)
}

// slugify converts a label to a safe directory name.
// Only characters in [a-z0-9-_] are kept; spaces become hyphens; all other
// characters (including path separators) are stripped.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// Close stops background goroutines started by New (cache GC and watch manager).
func (s *Service) Close() {
	if s.watchManager != nil {
		s.watchManager.Stop()
	}
	if s.summaryCache != nil {
		s.summaryCache.Stop()
	}
	if s.connectorManager != nil {
		s.connectorManager.Close()
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
