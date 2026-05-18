package claudeviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// SavePlanVersion creates a new version of a plan.
// It writes the version file to disk first, then saves to database.
// If database save fails, it deletes the file to maintain consistency.
func (s *Service) SavePlanVersion(ctx context.Context, planName, content string) error {
	// Get the plan from database to ensure it exists
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	// Create versions directory if it doesn't exist
	versionsDir := filepath.Join(s.viewerDir, "versions", planName)
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(versionsDir, 0o750); err != nil {
			return fmt.Errorf("failed to create versions directory: %w", err)
		}
	}

	// Generate version file with timestamp for uniqueness (handles concurrent writes)
	// Format: planName/versionNumber-timestamp.md where versionNumber is UI hint only
	now := s.nowProvider.Now()
	timestamp := strconv.FormatInt(now.Unix(), 10)

	// Get next version number for UI display (best-effort, not guaranteed unique on concurrent writes)
	lastVersionNum, err := s.db.GetLatestVersionNumber(ctx, plan.ID)
	if err != nil {
		return fmt.Errorf("failed to get latest version number: %w", err)
	}
	nextVersionNum := lastVersionNum + 1

	// Step 1: Write version file to disk FIRST (timestamp ensures filename uniqueness even under contention)
	versionFileName := fmt.Sprintf("%d-%s.md", nextVersionNum, timestamp)
	versionFilePath := filepath.Join(versionsDir, versionFileName)

	if err := os.WriteFile(versionFilePath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	// Step 2: Save to database (timestamp + plan_id ensures uniqueness; race condition safe)
	wordCount := CountWords(content)
	err = s.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
		PlanID:        plan.ID,
		VersionNumber: nextVersionNum,
		FilePath:      versionFilePath,
		Content:       content,
		WordCount:     int64(wordCount),
		CreatedAt:     now,
	})
	// Step 3: If database save fails, delete the file to maintain consistency
	if err != nil {
		deleteErr := os.Remove(versionFilePath)
		if deleteErr != nil {
			// Log the deletion error but prioritize reporting the original DB error
			//nolint:errorlint // Need to include cleanup error as context
			return fmt.Errorf("failed to save version to database: %w (also failed to clean up file: %v)", err, deleteErr)
		}
		return fmt.Errorf("failed to save version to database: %w", err)
	}

	return nil
}

// GetPlanVersionHistory retrieves version history for a plan with pagination.
func (s *Service) GetPlanVersionHistory(ctx context.Context, planName string, offset, limit int64) ([]PlanVersionDetail, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.GetPlanVersionHistory(ctx, dto.VersionHistoryParams{
		PlanID: plan.ID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get version history: %w", err)
	}

	// Get tags for the parent plan
	tags, err := s.db.GetPlanTags(ctx, plan.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tags: %w", err)
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)

	planVersionsDetail := make([]PlanVersionDetail, len(versions))
	for i, version := range versions {
		planVersion := PlanVersion{
			ID:            version.ID,
			PlanID:        version.PlanID,
			VersionNumber: version.VersionNumber,
			FilePath:      version.FilePath,
			Content:       version.Content,
			WordCount:     version.WordCount,
			CreatedAt:     version.CreatedAt,
		}
		renderedHTML, err := s.RenderMarkdown(version.Content)
		if err != nil {
			return nil, err
		}

		planVersionsDetail[i] = PlanVersionDetail{
			PlanVersion:  planVersion,
			RenderedHTML: renderedHTML,
			ReadingTime:  s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM),
			Tags:         tags,
		}
	}

	return planVersionsDetail, nil
}

// GetPlanVersion retrieves a specific version of a plan.
func (s *Service) GetPlanVersion(ctx context.Context, planName string, versionNumber int64) (*PlanVersionDetail, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	version, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, versionNumber)
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

	// Get tags for the parent plan
	tags, err := s.db.GetPlanTags(ctx, plan.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tags: %w", err)
	}

	renderedHTML, err := s.RenderMarkdown(version.Content)
	if err != nil {
		return nil, err
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	readingTime := s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM)

	planVersion := PlanVersion{
		ID:            version.ID,
		PlanID:        version.PlanID,
		VersionNumber: version.VersionNumber,
		FilePath:      version.FilePath,
		Content:       version.Content,
		WordCount:     version.WordCount,
		CreatedAt:     version.CreatedAt,
	}

	return &PlanVersionDetail{
		PlanVersion:  planVersion,
		RenderedHTML: renderedHTML,
		ReadingTime:  readingTime,
		Tags:         tags,
	}, nil
}

// rollbackFiles restores source and viewer files to their previous content after a failed restore.
// Errors are intentionally ignored because this is a best-effort cleanup on an already-failing path.
func rollbackFiles(sourcePath string, oldSource []byte, viewerPath string, oldViewer []byte) {
	//nolint:gosec // G703: paths are constructed by the application from trusted config, not user input
	_ = os.WriteFile(sourcePath, oldSource, 0o600)
	//nolint:gosec // G703: paths are constructed by the application from trusted config, not user input
	_ = os.WriteFile(viewerPath, oldViewer, 0o600)
}

// RestorePlanVersion restores a plan to a previous version.
// Files are written first (source before viewer); then UpdatePlan + InsertPlanVersion are
// committed in a single transaction. On any DB failure all file writes are rolled back.
func (s *Service) RestorePlanVersion(ctx context.Context, planName string, versionNumber int64) error {
	// Fetch the plan and target version from DB.
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	version, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, versionNumber)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	sourcePath := filepath.Join(s.sourcePlansDir, planName)
	viewerPath := filepath.Join(s.viewerDir, planName)

	// Read current content so we can roll back files if the DB transaction fails.
	//nolint:gosec // G304: path is constructed by the application from trusted config, not user input
	oldSource, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}
	//nolint:gosec // G304: path is constructed by the application from trusted config, not user input
	oldViewer, err := os.ReadFile(viewerPath)
	if err != nil {
		return fmt.Errorf("failed to read viewer file: %w", err)
	}

	restoredContent := []byte(version.Content)

	// Write source first, then viewer.
	if err := os.WriteFile(sourcePath, restoredContent, 0o600); err != nil {
		return fmt.Errorf("failed to write source file: %w", err)
	}
	if err := os.WriteFile(viewerPath, restoredContent, 0o600); err != nil {
		rollbackFiles(sourcePath, oldSource, viewerPath, oldViewer)
		return fmt.Errorf("failed to write viewer file: %w", err)
	}

	info, err := os.Stat(viewerPath)
	if err != nil {
		rollbackFiles(sourcePath, oldSource, viewerPath, oldViewer)
		return fmt.Errorf("failed to stat restored file: %w", err)
	}

	title := extractTitle(version.Content)
	wordCount := CountWords(version.Content)
	now := s.nowProvider.Now()

	contentToStore := version.Content
	if !s.indexFullContent {
		contentToStore = title
	}

	// Prepare the version file on disk before touching the DB.
	lastVersionNum, err := s.db.GetLatestVersionNumber(ctx, plan.ID)
	if err != nil {
		rollbackFiles(sourcePath, oldSource, viewerPath, oldViewer)
		return fmt.Errorf("failed to get latest version number: %w", err)
	}

	nextVersionNum := lastVersionNum + 1
	timestamp := strconv.FormatInt(now.Unix(), 10)
	versionsDir := filepath.Join(s.viewerDir, "versions", planName)

	if err := os.MkdirAll(versionsDir, 0o750); err != nil {
		rollbackFiles(sourcePath, oldSource, viewerPath, oldViewer)
		return fmt.Errorf("failed to create versions directory: %w", err)
	}

	versionFilePath := filepath.Join(versionsDir, fmt.Sprintf("%d-%s.md", nextVersionNum, timestamp))
	if err := os.WriteFile(versionFilePath, restoredContent, 0o600); err != nil {
		rollbackFiles(sourcePath, oldSource, viewerPath, oldViewer)
		return fmt.Errorf("failed to write version file: %w", err)
	}

	// Atomically update the plan record and insert the new version in the DB.
	err = s.db.RestorePlanVersion(ctx, dto.RestorePlanVersionParams{
		Plan: dto.UpdatePlanParams{
			FileName:   planName,
			Title:      title,
			Content:    contentToStore,
			ModifiedAt: info.ModTime(),
			IndexedAt:  now,
			FileSize:   info.Size(),
			WordCount:  int64(wordCount),
		},
		Version: dto.InsertPlanVersionParams{
			PlanID:        plan.ID,
			VersionNumber: nextVersionNum,
			FilePath:      versionFilePath,
			Content:       version.Content,
			WordCount:     int64(wordCount),
			CreatedAt:     now,
		},
	})
	if err != nil {
		// DB failed — roll back all file changes.
		rollbackFiles(sourcePath, oldSource, viewerPath, oldViewer)
		_ = os.Remove(versionFilePath)
		return fmt.Errorf("failed to restore plan: %w", err)
	}

	return nil
}

// GetVersionCount returns the total number of versions for a plan.
func (s *Service) GetVersionCount(ctx context.Context, planName string) (int64, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return 0, fmt.Errorf("plan not found: %w", err)
	}

	count, err := s.db.GetVersionCount(ctx, plan.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get version count: %w", err)
	}

	return count, nil
}

// CleanupOldVersions deletes versions beyond the limit (keeping only the most recent ones).
// maxVersions specifies the maximum number of versions to keep per plan.
func (s *Service) CleanupOldVersions(ctx context.Context, planName string, maxVersions int64) error {
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	// Get all versions
	versions, err := s.db.GetPlanVersionHistory(ctx, dto.VersionHistoryParams{
		PlanID: plan.ID,
		Limit:  1000, // Get all versions
		Offset: 0,
	})
	if err != nil {
		return fmt.Errorf("failed to get versions for cleanup: %w", err)
	}

	// If we have more than maxVersions, delete the oldest ones
	if int64(len(versions)) > maxVersions {
		versionsToDelete := versions[maxVersions:] // Oldest versions come after (since sorted DESC)

		// Delete files
		for _, version := range versionsToDelete {
			if err := os.Remove(version.FilePath); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("failed to delete old version file", "path", version.FilePath, "error", err)
			}
		}

		// Delete from database (delete versions older than the cutoff version number)
		if len(versionsToDelete) > 0 {
			// The highest version number to delete (oldest versions we're removing)
			cutoffVersionNum := versionsToDelete[0].VersionNumber
			err = s.db.DeleteVersionsOlderThan(ctx, dto.DeleteVersionsParams{
				PlanID:        plan.ID,
				VersionNumber: cutoffVersionNum,
			})
			if err != nil {
				return fmt.Errorf("failed to delete old versions from database: %w", err)
			}
		}
	}

	return nil
}

// SearchVersions searches across all versions of a plan for matching content.
// Returns all versions that contain the search term (case-insensitive).
func (s *Service) SearchVersions(ctx context.Context, planName, query string) ([]PlanVersionDetail, error) {
	// Get the plan to verify it exists and get its ID
	plan, err := s.GetPlanByFileName(ctx, planName)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	// Search versions with LIKE query (case-insensitive in SQLite)
	versions, err := s.db.SearchVersionsByContent(ctx, dto.SearchVersionsParams{
		PlanID: plan.ID,
		Query:  query,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Get tags for the parent plan
	tags, err := s.db.GetPlanTags(ctx, plan.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tags: %w", err)
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)

	planVersionsDetail := make([]PlanVersionDetail, len(versions))
	for i, version := range versions {
		planVersion := PlanVersion{
			ID:            version.ID,
			PlanID:        version.PlanID,
			VersionNumber: version.VersionNumber,
			FilePath:      version.FilePath,
			Content:       version.Content,
			WordCount:     version.WordCount,
			CreatedAt:     version.CreatedAt,
		}
		renderedHTML, err := s.RenderMarkdown(version.Content)
		if err != nil {
			return nil, err
		}

		planVersionsDetail[i] = PlanVersionDetail{
			PlanVersion:  planVersion,
			RenderedHTML: renderedHTML,
			ReadingTime:  s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM),
			Tags:         tags,
		}
	}

	return planVersionsDetail, nil
}
