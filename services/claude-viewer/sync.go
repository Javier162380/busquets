package claudeviewer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"golang.org/x/sync/errgroup"
)

// SyncPlans copies and indexes all plans from source directory.
func (s *Service) SyncPlans(ctx context.Context) (int, error) {
	if _, err := os.Stat(s.viewerDir); os.IsNotExist(err) {
		if err := os.MkdirAll(s.viewerDir, 0o750); err != nil {
			return 0, fmt.Errorf("failed to create viewer directory: %w", err)
		}
	}

	// Create versions directory structure
	versionsDir := filepath.Join(s.viewerDir, "versions")
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(versionsDir, 0o750); err != nil {
			return 0, fmt.Errorf("failed to create versions directory: %w", err)
		}
	}

	entries, err := os.ReadDir(s.sourcePlansDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read plans directory: %w", err)
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
			updated, err := s.syncSinglePlan(groupCtx, entryName)
			if err != nil {
				return fmt.Errorf("failed to sync %s: %w", entryName, err)
			}
			if updated {
				syncPlans.Add(int64(1))
			}
			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return 0, fmt.Errorf("failed to sync plans: %w", err)
	}

	syncDeRef := int(syncPlans.Load())

	return syncDeRef, nil
}

// RSyncPlans is the reverse of SyncPlans: it copies indexed plans from viewerDir
// back to sourcePlansDir. Only plans already in the DB are candidates, and only
// those missing from sourcePlansDir are written.
func (s *Service) RSyncPlans(ctx context.Context) (int, error) {
	if err := os.MkdirAll(s.sourcePlansDir, 0o750); err != nil {
		return 0, fmt.Errorf("failed to create source plans directory: %w", err)
	}

	// Build a set of files already present in sourcePlansDir to avoid overwriting them.
	sourceEntries, err := os.ReadDir(s.sourcePlansDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read source plans directory: %w", err)
	}
	sourcePlanFiles := make(map[string]struct{}, len(sourceEntries))
	for _, entry := range sourceEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			sourcePlanFiles[entry.Name()] = struct{}{}
		}
	}

	// The DB is the authoritative list of indexed plans — iterate it, not the filesystem.
	// This ensures plans deleted from sourcePlansDir but still in viewerDir are found.
	summaries, err := s.db.ListAllPlans(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list plans: %w", err)
	}

	rsyncPlans := atomic.Int64{}
	errGroup := errgroup.Group{}
	errGroup.SetLimit(5)

	for _, summary := range summaries {
		fileName := summary.FileName
		if _, ok := sourcePlanFiles[fileName]; ok {
			continue // already in sourcePlansDir, nothing to do
		}
		errGroup.Go(func() error {
			viewerPath := filepath.Join(s.viewerDir, fileName)
			if _, statErr := os.Stat(viewerPath); os.IsNotExist(statErr) {
				return nil // file gone from viewerDir too; nothing to restore
			} else if statErr != nil {
				return statErr
			}

			destPath := filepath.Join(s.sourcePlansDir, fileName)
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

// syncSinglePlan copies and indexes a single plan file.
// Returns true if the file was updated, false if skipped (no changes).
func (s *Service) syncSinglePlan(ctx context.Context, fileName string) (bool, error) {
	sourcePath := filepath.Join(s.sourcePlansDir, fileName)
	destPath := filepath.Join(s.viewerDir, fileName)

	// Get source file modification time
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat source file: %w", err)
	}

	// Check if plan exists in DB and compare modification times
	existingPlan, err := s.db.GetPlanByFileName(ctx, fileName)
	planExists := err == nil

	if planExists {
		// Plan exists - check if source file is newer than indexed version
		if !sourceInfo.ModTime().After(existingPlan.ModifiedAt) {
			// File hasn't changed since last sync, skip
			return false, nil
		}
	} else if !dto.IsNotFound(err) {
		// Unexpected database error
		return false, fmt.Errorf("failed to check existing plan: %w", err)
	}
	// If dto.IsNotFound(err), this is a new file, proceed with sync

	// Copy file
	if err := copyFile(sourcePath, destPath); err != nil {
		return false, fmt.Errorf("failed to copy file: %w", err)
	}

	// Read content and extract title
	//nolint:gosec // G304: Path is controlled by application, not user input
	content, err := os.ReadFile(destPath)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	title := extractTitle(string(content))
	wordCount := CountWords(string(content))

	// Extract tags from content
	tags := ExtractTagsFromContent(string(content))
	normalizedTags := NormalizeTags(tags)

	// Determine what to store in content field
	contentToStore := string(content)
	if !s.indexFullContent {
		// Only store title if full content indexing is disabled
		contentToStore = title
	}

	// Get file stats
	info, err := os.Stat(destPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	now := s.nowProvider.Now()

	// Resolve tag names to IDs before the transaction (creates tags if needed).
	tagIDs, err := s.resolveTagIDs(ctx, normalizedTags, now)
	if err != nil {
		return false, fmt.Errorf("failed to resolve tag IDs: %w", err)
	}

	if planExists {
		err = s.db.UpdatePlanWithTags(ctx, dto.UpdatePlanWithTagsParams{
			Plan: dto.UpdatePlanParams{
				FileName:   fileName,
				Title:      title,
				Content:    contentToStore,
				ModifiedAt: info.ModTime(),
				IndexedAt:  now,
				FileSize:   info.Size(),
				WordCount:  int64(wordCount),
			},
			TagIDs:     tagIDs,
			AssignedAt: now,
		})
		if err != nil {
			return false, fmt.Errorf("failed to update plan with tags: %w", err)
		}
		return true, nil
	}

	err = s.db.InsertPlanWithTags(ctx, dto.InsertPlanWithTagsParams{
		Plan: dto.InsertPlanParams{
			FileName:   fileName,
			FilePath:   destPath,
			Title:      title,
			Content:    contentToStore,
			CreatedAt:  info.ModTime(),
			ModifiedAt: info.ModTime(),
			IndexedAt:  now,
			FileSize:   info.Size(),
			WordCount:  int64(wordCount),
		},
		TagIDs:     tagIDs,
		AssignedAt: now,
	})
	if err != nil {
		return false, fmt.Errorf("failed to insert plan with tags: %w", err)
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
