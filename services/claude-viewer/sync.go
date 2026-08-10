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
		planID := summary.ID

		if _, ok := sourceFiles[syncSource][fileName]; ok {
			continue
		}

		errGroup.Go(func() error {
			viewerPath := s.mirrorPathFor(planID, fileName)
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

// DeletePlan removes a plan everywhere it lives. Filesystem artifacts are removed
// first so a delete failure aborts before any DB mutation (and so a surviving source
// file can never resurrect the plan on the next sync). The DB row, its tag/comment
// associations, and all version rows are then removed in a single transaction.
//
// The plan's mirror path is looked up fresh from the DB rather than accepted as a
// parameter — trusting a caller-supplied path would let a mismatched value delete
// an unrelated plan's files.
func (s *Service) DeletePlan(ctx context.Context, fileName, syncSource string) error {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	paths := []string{
		filepath.Join(syncSource, fileName), // source
		plan.FilePath,                       // mirror
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete %s: %w", p, err)
		}
	}
	// os.RemoveAll is a no-op if the dir is absent; only a real failure returns err.
	// filepath.Dir(plan.FilePath) is the plan's id-scoped directory, so its "versions"
	// sibling belongs to this plan alone — no cross-source collision risk.
	versionsDir := filepath.Join(filepath.Dir(plan.FilePath), "versions")
	if err := os.RemoveAll(versionsDir); err != nil {
		return fmt.Errorf("failed to delete version files %s: %w", versionsDir, err)
	}

	if err := s.db.DeletePlan(ctx, fileName, syncSource); err != nil {
		return err
	}

	s.summaryCache.Delete(fileName)
	return nil
}

// fileMove is a single filesystem rename (from -> to), tracked so a failed rename
// partway through RenamePlanFile can undo the moves already performed.
type fileMove struct {
	from string
	to   string
}

// RenamePlanFile renames a plan's file everywhere it lives: the source file, the
// viewer mirror file, and the versions directory, plus the plans row and the stored
// version file paths. Only the file name changes — the plan's title (derived from the
// markdown heading), content, and sync source are untouched.
//
// It is transaction-safe: the plan row, the stored version paths, and the file moves
// all happen inside a single DB transaction — the transaction commits only once the
// files have moved, so a failed move rolls the DB writes back with it (there is no
// separate compensating write that could itself fail). If the commit fails after the
// moves, the moves are undone.
//
// The plan's current mirror path is looked up fresh from the DB rather than accepted
// as a parameter — trusting a caller-supplied path would let a mismatched value rename
// (and thus corrupt the DB row of) an unrelated plan's file.
func (s *Service) RenamePlanFile(ctx context.Context, fileName, syncSource, newFileName string) error {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	// 1. Normalize + validate the new name.
	newFileName = filepath.Base(strings.TrimSpace(newFileName))
	if newFileName == "" || newFileName == "." || newFileName == string(filepath.Separator) {
		return fmt.Errorf("invalid new file name")
	}
	if !strings.EqualFold(filepath.Ext(newFileName), ".md") {
		newFileName += ".md"
	}
	if newFileName == fileName {
		return fmt.Errorf("new file name is the same as the current one")
	}

	// 2. Compute old/new paths for source and mirror. The versions directory
	// (a sibling of the mirror file, under the plan's id-scoped directory) is
	// untouched by a rename — it's keyed by plan id and internal version
	// numbers/timestamps, nothing about it depends on the current file name.
	oldSource := filepath.Join(syncSource, fileName)
	newSource := filepath.Join(syncSource, newFileName)
	oldMirror := plan.FilePath
	newMirror := filepath.Join(filepath.Dir(plan.FilePath), newFileName)

	// 3. Collision checks — before any DB or FS mutation, so we never clobber another
	// plan's file on disk or row in the DB.
	if _, err := os.Stat(newSource); err == nil {
		return fmt.Errorf("a file already exists at %s", newSource)
	}
	if _, err := os.Stat(newMirror); err == nil {
		return fmt.Errorf("a file already exists at %s", newMirror)
	}
	if _, err := s.db.GetPlanByFileName(ctx, newFileName, syncSource); err == nil {
		return fmt.Errorf("a plan named %q already exists in this source", newFileName)
	} else if !dto.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing plan: %w", err)
	}

	// 4. Prepare the file moves. They run inside the DB transaction (step 5), so the
	// commit is gated on them succeeding.
	pending := []fileMove{
		{from: oldSource, to: newSource},
		{from: oldMirror, to: newMirror},
	}

	// 5. Rename in the DB, moving the files as part of the same transaction. Version
	// paths key off the immutable plan_id, so renaming file_name never orphans tags,
	// comments, or version rows.
	var done []fileMove
	err = s.db.RenamePlanFile(ctx, dto.RenamePlanFileParams{
		OldFileName: fileName,
		SyncSource:  syncSource,
		NewFileName: newFileName,
		NewFilePath: newMirror,
	}, func() error {
		for _, m := range pending {
			if renameErr := os.Rename(m.from, m.to); renameErr != nil {
				return fmt.Errorf("failed to move %s to %s: %w", m.from, m.to, renameErr)
			}
			done = append(done, m)
		}
		return nil
	})
	if err != nil {
		// The transaction did not commit (a move failed, or the commit itself failed).
		// Undo any moves that did happen so the filesystem matches the rolled-back DB.
		for i := len(done) - 1; i >= 0; i-- {
			_ = os.Rename(done[i].to, done[i].from)
		}
		return fmt.Errorf("failed to rename plan: %w", err)
	}

	// 6. Invalidate the summary cache for both the old and new file names.
	s.summaryCache.Delete(fileName)
	s.summaryCache.Delete(newFileName)
	return nil
}

// syncSinglePlan copies and indexes a single plan file from the given source directory.
// Returns true if the file was updated, false if skipped (no changes).
func (s *Service) syncSinglePlan(ctx context.Context, dir config.SyncDir, fileName string) (bool, error) {
	sourcePath := filepath.Join(dir.Path, fileName)

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

	//nolint:gosec // G304: Path is controlled by application, not user input
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return false, fmt.Errorf("failed to read source file: %w", err)
	}

	title := extractTitle(string(content))
	wordCount := CountWords(string(content))

	contentToStore := string(content)
	if !s.indexFullContent {
		contentToStore = title
	}

	now := s.nowProvider.Now()

	if planExists {
		if _, err := s.db.UpdatePlan(ctx, dto.UpdatePlanParams{
			FileName:   fileName,
			SyncSource: dir.Path,
			Title:      title,
			Content:    contentToStore,
			ModifiedAt: sourceInfo.ModTime(),
			IndexedAt:  now,
			FileSize:   sourceInfo.Size(),
			WordCount:  int64(wordCount),
		}); err != nil {
			return false, fmt.Errorf("failed to update plan: %w", err)
		}

		destPath := s.mirrorPathFor(existingPlan.ID, fileName)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
			return false, fmt.Errorf("failed to create plan directory: %w", err)
		}
		if err := copyFile(sourcePath, destPath); err != nil {
			return false, fmt.Errorf("failed to copy file: %w", err)
		}
		return true, nil
	}

	_, err = s.db.InsertPlan(ctx, dto.InsertPlanParams{
		FileName:   fileName,
		SyncSource: dir.Path,
		Title:      title,
		Content:    contentToStore,
		CreatedAt:  sourceInfo.ModTime(),
		ModifiedAt: sourceInfo.ModTime(),
		IndexedAt:  now,
		FileSize:   sourceInfo.Size(),
		WordCount:  int64(wordCount),
	}, func(id int64) (string, error) {
		destPath := s.mirrorPathFor(id, fileName)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
			return "", err
		}
		if err := copyFile(sourcePath, destPath); err != nil {
			return "", err
		}
		return destPath, nil
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
