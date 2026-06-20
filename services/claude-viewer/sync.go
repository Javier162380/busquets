package claudeviewer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/Javier162380/claude-plan-viewer/internal/config"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"golang.org/x/sync/errgroup"
)

// SyncPlans syncs plans from all configured source directories concurrently.
func (s *Service) SyncPlans(ctx context.Context) (int, error) {
	if _, err := os.Stat(s.viewerDir); os.IsNotExist(err) {
		if err := os.MkdirAll(s.viewerDir, 0o750); err != nil {
			return 0, fmt.Errorf("failed to create viewer directory: %w", err)
		}
	}

	versionsDir := filepath.Join(s.viewerDir, "versions")
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(versionsDir, 0o750); err != nil {
			return 0, fmt.Errorf("failed to create versions directory: %w", err)
		}
	}

	syncPlans := atomic.Int64{}
	errGroup, groupCtx := errgroup.WithContext(ctx)
	errGroup.SetLimit(len(s.sourcePlansDirs))

	for _, dir := range s.sourcePlansDirs {
		errGroup.Go(func() error {
			count, err := s.syncDirectory(groupCtx, dir)
			if err != nil {
				return err
			}
			syncPlans.Add(int64(count))
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return 0, fmt.Errorf("failed to sync plans: %w", err)
	}

	return int(syncPlans.Load()), nil
}

// syncDirectory syncs all .md files from a single source directory.
func (s *Service) syncDirectory(ctx context.Context, dir config.SyncDir) (int, error) {
	destSubdir := filepath.Join(s.viewerDir, s.viewerSubdirFor(dir.Path))
	if err := os.MkdirAll(destSubdir, 0o750); err != nil {
		return 0, fmt.Errorf("failed to create viewer subdir for %s: %w", dir.Label, err)
	}

	entries, err := os.ReadDir(dir.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to read plans directory %s: %w", dir.Path, err)
	}

	syncPlans := atomic.Int64{}
	errGroup, groupCtx := errgroup.WithContext(ctx)
	errGroup.SetLimit(5)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		entryName := entry.Name()
		errGroup.Go(func() error {
			updated, err := s.syncSinglePlan(groupCtx, dir, entryName)
			if err != nil {
				return fmt.Errorf("failed to sync %s: %w", entryName, err)
			}
			if updated {
				syncPlans.Add(1)
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return 0, fmt.Errorf("failed to sync plans from %s: %w", dir.Path, err)
	}

	return int(syncPlans.Load()), nil
}

// RSyncPlans copies indexed plans back to their source directories.
// Only plans missing from their source directory are written.
func (s *Service) RSyncPlans(ctx context.Context) (int, error) {
	for _, dir := range s.sourcePlansDirs {
		if err := os.MkdirAll(dir.Path, 0o750); err != nil {
			return 0, fmt.Errorf("failed to create source plans directory %s: %w", dir.Path, err)
		}
	}

	summaries, err := s.db.ListAllPlans(ctx, sortKeyToColumn(DefaultPlansSortKey), DefaultSortDir, DefaultReadingSpeedWPM)
	if err != nil {
		return 0, fmt.Errorf("failed to list plans: %w", err)
	}

	// Pre-build a per-source set of files already on disk — one ReadDir per unique
	// source directory rather than one per plan.
	sourceFiles := make(map[string]map[string]struct{})
	for _, summary := range summaries {
		if _, seen := sourceFiles[summary.SyncSource]; seen {
			continue
		}
		entries, readErr := os.ReadDir(summary.SyncSource)
		if readErr != nil {
			s.logger.Warn("failed to read source dir for rsync", "dir", summary.SyncSource, "error", readErr)
			sourceFiles[summary.SyncSource] = map[string]struct{}{} // empty — nothing to skip
			continue
		}
		set := make(map[string]struct{}, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				set[entry.Name()] = struct{}{}
			}
		}
		sourceFiles[summary.SyncSource] = set
	}

	rsyncPlans := atomic.Int64{}
	errGroup := errgroup.Group{}
	errGroup.SetLimit(5)

	for _, summary := range summaries {
		fileName := summary.FileName
		syncSource := summary.SyncSource
		viewerSubdir := s.viewerSubdirFor(syncSource)

		if _, ok := sourceFiles[syncSource][fileName]; ok {
			continue
		}

		errGroup.Go(func() error {
			viewerPath := filepath.Join(s.viewerDir, viewerSubdir, fileName)
			if _, statErr := os.Stat(viewerPath); os.IsNotExist(statErr) {
				return nil
			} else if statErr != nil {
				return statErr
			}

			destPath := filepath.Join(syncSource, fileName)
			if err := copyFile(viewerPath, destPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", fileName, err)
			}
			rsyncPlans.Add(1)
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return 0, fmt.Errorf("failed to rsync plans: %w", err)
	}

	return int(rsyncPlans.Load()), nil
}

// syncSinglePlan copies and indexes a single plan file from the given source directory.
// Returns true if the file was updated, false if skipped (no changes).
func (s *Service) syncSinglePlan(ctx context.Context, dir config.SyncDir, fileName string) (bool, error) {
	sourcePath := filepath.Join(dir.Path, fileName)
	destSubdir := filepath.Join(s.viewerDir, s.viewerSubdirFor(dir.Path))
	destPath := filepath.Join(destSubdir, fileName)

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat source file: %w", err)
	}

	existingPlan, err := s.db.GetPlanByFileName(ctx, fileName, dir.Path)
	planExists := err == nil

	if planExists {
		if !sourceInfo.ModTime().After(existingPlan.ModifiedAt) {
			return false, nil
		}
	} else if !dto.IsNotFound(err) {
		return false, fmt.Errorf("failed to check existing plan: %w", err)
	}

	if err := copyFile(sourcePath, destPath); err != nil {
		return false, fmt.Errorf("failed to copy file: %w", err)
	}

	//nolint:gosec // G304: Path is controlled by application, not user input
	content, err := os.ReadFile(destPath)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	title := extractTitle(string(content))
	wordCount := CountWords(string(content))

	contentToStore := string(content)
	if !s.indexFullContent {
		contentToStore = title
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	now := s.nowProvider.Now()

	if planExists {
		err = s.db.UpdatePlan(ctx, dto.UpdatePlanParams{
			FileName:   fileName,
			SyncSource: dir.Path,
			Title:      title,
			Content:    contentToStore,
			ModifiedAt: info.ModTime(),
			IndexedAt:  now,
			FileSize:   info.Size(),
			WordCount:  int64(wordCount),
		})
		if err != nil {
			return false, fmt.Errorf("failed to update plan: %w", err)
		}
		return true, nil
	}

	err = s.db.InsertPlan(ctx, dto.InsertPlanParams{
		FileName:   fileName,
		SyncSource: dir.Path,
		FilePath:   destPath,
		Title:      title,
		Content:    contentToStore,
		CreatedAt:  info.ModTime(),
		ModifiedAt: info.ModTime(),
		IndexedAt:  now,
		FileSize:   info.Size(),
		WordCount:  int64(wordCount),
	})
	if err != nil {
		return false, fmt.Errorf("failed to insert plan: %w", err)
	}

	return true, nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	//nolint:gosec // G304: Paths are controlled by application, not user input
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	//nolint:gosec // G304: Paths are controlled by application, not user input
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// extractTitle extracts the first # heading from markdown content.
func extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return "Untitled Plan"
}
