package claudeviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// DumpPlans writes all plans stored in the database back to the source plans directory.
// Useful when plans exist in the database but not on disk (e.g. after a database migration
// or when the source directory has been lost).
func (s *Service) DumpPlans(ctx context.Context) (int, error) {
	summaries, err := s.db.ListAllPlans(ctx, sortKeyToColumn(DefaultPlansSortKey), DefaultSortDir)
	if err != nil {
		return 0, fmt.Errorf("failed to list plans: %w", err)
	}

	dumped := atomic.Int64{}
	errGroup, groupCtx := errgroup.WithContext(ctx)
	errGroup.SetLimit(5)

	for _, summary := range summaries {
		fileName := summary.FileName
		errGroup.Go(func() error {
			plan, err := s.db.GetPlanByFileName(groupCtx, fileName)
			if err != nil {
				return fmt.Errorf("failed to get plan %s: %w", fileName, err)
			}
			if plan.Content == "" {
				return nil
			}
			destPath := filepath.Join(s.sourcePlansDir, fileName)
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				//nolint:gosec // G306: Plans are user-owned markdown files
				if err := os.WriteFile(destPath, []byte(plan.Content), 0o644); err != nil {
					return fmt.Errorf("failed to write %s: %w", fileName, err)
				}
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
