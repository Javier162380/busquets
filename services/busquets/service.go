// Package busquets implements the Busquets plan-indexing service logic.
package busquets

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/Javier162380/busquets/internal/cache"
	"github.com/Javier162380/busquets/internal/clipboard"
	"github.com/Javier162380/busquets/internal/config"
	"github.com/Javier162380/busquets/internal/connectors"
	"github.com/Javier162380/busquets/internal/nowprovider"
	"github.com/Javier162380/busquets/services/busquets/dto"
)

const (
	summaryCacheTTL      = time.Hour
	summaryCacheGCPeriod = 10 * time.Minute
)

// Service represents the Busquets service.
type Service struct {
	db               dto.Repository
	viewerDir        string
	sourcePlansDirs  []config.SyncDir
	indexFullContent bool
	nowProvider      nowprovider.NowProvider
	connectorManager *connectors.Manager
	watchManager     *WatchManager
	summaryCache     *cache.MuxCache[string]
	logger           *slog.Logger
	clipboard        clipboard.Clipboard
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

// New creates a new Busquets service instance.
// The db parameter must implement dto.Repository (sqlite.Repository or postgres.Repository).
func New(ctx context.Context, db dto.Repository, viewerDir string, syncDirs []config.SyncDir, indexFullContent bool) (*Service, error) {
	svc := &Service{
		db:               db,
		viewerDir:        viewerDir,
		sourcePlansDirs:  syncDirs,
		indexFullContent: indexFullContent,
		nowProvider:      nowprovider.SystemTimeProvider{},
		summaryCache:     cache.New[string](summaryCacheTTL, summaryCacheGCPeriod),
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		clipboard:        clipboard.SystemClipboard{},
	}

	// Initialize watch manager
	svc.watchManager = NewWatchManager(svc)

	// Reconcile any file_path rows still pointing at the pre-rebrand
	// ~/.claude-viewer location after config.MigrateLegacyViewerDir has
	// already renamed the directory on disk. Runs on every construction —
	// see MigrateLegacyFilePathPrefix's own settings-flag gate for why this
	// is cheap once done.
	//
	// Gated on viewerDir matching the default, exactly like
	// config.MigrateLegacyViewerDir gates its own directory rename on
	// cfg.Paths.ViewerDir matching the default. Without this check, a caller
	// running with a custom viewer_dir/BUSQUETS_DIR — which the directory
	// migration correctly declines to touch — would still have this rewrite
	// any lingering .claude-viewer-prefixed rows into that custom dir and
	// regenerate files there, silently doing exactly what the directory-level
	// migration explicitly opted out of.
	if homeDir, err := os.UserHomeDir(); err == nil {
		oldDir := filepath.Join(homeDir, config.LegacyViewerDirName)
		defaultViewerDir := config.DefaultConfig().Paths.ViewerDir
		if viewerDir == defaultViewerDir {
			if _, err := svc.MigrateLegacyFilePathPrefix(ctx, oldDir, viewerDir); err != nil {
				return nil, fmt.Errorf("failed to migrate legacy file paths: %w", err)
			}
		}
	}

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

// planDirFor returns a plan's id-scoped storage directory, which holds both its
// mirror file and its versions/ subdirectory. Keyed by plan.id (immutable,
// globally unique) — never depends on sync label or filename collisions across
// sources, and never on a path read back out of the database.
func (s *Service) planDirFor(planID int64) string {
	return filepath.Join(s.viewerDir, "plans", strconv.FormatInt(planID, 10))
}

// mirrorPathFor returns the on-disk path for a plan's current-content mirror.
func (s *Service) mirrorPathFor(planID int64, fileName string) string {
	return filepath.Join(s.planDirFor(planID), fileName)
}

// versionsDirFor returns a plan's versions directory, keyed by plan.id.
func (s *Service) versionsDirFor(planID int64) string {
	return filepath.Join(s.planDirFor(planID), "versions")
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
