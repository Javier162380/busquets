package busquets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// DumpPlans writes all plans stored in the database back to their source directories.
// Useful when plans exist in the database but not on disk (e.g. after a database migration
// or when the source directory has been lost).
//
// Returns the number of files actually written. Plans already present on disk are left
// untouched and not counted.
func (s *Service) DumpPlans(ctx context.Context) (int, error) {
	summaries, err := s.db.ListAllPlans(ctx, sortKeyToColumn(DefaultPlansSortKey), DefaultSortDir, DefaultReadingSpeedWPM)
	if err != nil {
		return 0, fmt.Errorf("failed to list plans: %w", err)
	}

	dumped := atomic.Int64{}
	errGroup, groupCtx := errgroup.WithContext(ctx)
	errGroup.SetLimit(5)

	for _, summary := range summaries {
		fileName := summary.FileName
		syncSource := summary.SyncSource
		errGroup.Go(func() error {
			plan, err := s.db.GetPlanByFileName(groupCtx, fileName, syncSource)
			if err != nil {
				return fmt.Errorf("failed to get plan %s: %w", fileName, err)
			}
			if plan.Content == "" {
				return nil
			}

			destDir := s.configuredSourcePath(syncSource)
			if !s.isConfiguredSource(destDir) {
				s.logger.Warn("skipping dump: sync source is not a configured plans dir",
					"file", fileName, "dir", destDir)
				return nil
			}
			if err := os.MkdirAll(destDir, 0o750); err != nil {
				return fmt.Errorf("failed to create source directory %s: %w", destDir, err)
			}

			destPath := filepath.Join(destDir, fileName)
			if _, err := os.Stat(destPath); err == nil {
				return nil // already on disk; dump never overwrites
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to stat %s: %w", destPath, err)
			}

			//nolint:gosec // G306: Plans are user-owned markdown files
			if err := os.WriteFile(destPath, []byte(plan.Content), 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", fileName, err)
			}
			dumped.Add(1)
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return 0, fmt.Errorf("failed to dump plans: %w", err)
	}

	return int(dumped.Load()), nil
}
