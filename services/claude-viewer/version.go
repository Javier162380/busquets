package claudeviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// SavePlanVersion creates a new version snapshot of a plan.
func (s *Service) SavePlanVersion(ctx context.Context, planName, syncSource, content string) error {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	versionsDir := filepath.Join(s.viewerDir, "versions", planName)
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(versionsDir, 0o750); err != nil {
			return fmt.Errorf("failed to create versions directory: %w", err)
		}
	}

	now := s.nowProvider.Now()
	timestamp := strconv.FormatInt(now.Unix(), 10)

	lastVersionNum, err := s.db.GetLatestVersionNumber(ctx, plan.ID)
	if err != nil {
		return fmt.Errorf("failed to get latest version number: %w", err)
	}
	nextVersionNum := lastVersionNum + 1

	versionFileName := fmt.Sprintf("%d-%s.md", nextVersionNum, timestamp)
	versionFilePath := filepath.Join(versionsDir, versionFileName)

	if err := os.WriteFile(versionFilePath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	wordCount := CountWords(content)
	err = s.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
		PlanID:        plan.ID,
		VersionNumber: nextVersionNum,
		FilePath:      versionFilePath,
		Content:       content,
		WordCount:     int64(wordCount),
		CreatedAt:     now,
	})
	if err != nil {
		deleteErr := os.Remove(versionFilePath)
		if deleteErr != nil {
			//nolint:errorlint // Need to include cleanup error as context
			return fmt.Errorf("failed to save version to database: %w (also failed to clean up file: %v)", err, deleteErr)
		}
		return fmt.Errorf("failed to save version to database: %w", err)
	}

	return nil
}

// GetPlanVersionHistory retrieves version history for a plan with pagination.
func (s *Service) GetPlanVersionHistory(ctx context.Context, planName, syncSource string, offset, limit int64) ([]PlanVersionDetail, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
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
func (s *Service) GetPlanVersion(ctx context.Context, planName, syncSource string, versionNumber int64) (*PlanVersionDetail, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	version, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, versionNumber)
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

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
func rollbackFiles(sourcePath string, oldSource []byte, viewerPath string, oldViewer []byte) {
	//nolint:gosec // G703: paths are constructed by the application from trusted config, not user input
	_ = os.WriteFile(sourcePath, oldSource, 0o600)
	//nolint:gosec // G703: paths are constructed by the application from trusted config, not user input
	_ = os.WriteFile(viewerPath, oldViewer, 0o600)
}

// RestorePlanVersion restores a plan to a previous version.
func (s *Service) RestorePlanVersion(ctx context.Context, planName, syncSource string, versionNumber int64) error {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	version, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, versionNumber)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	sourcePath := filepath.Join(syncSource, planName)
	viewerPath := filepath.Join(s.viewerDir, s.viewerSubdirFor(syncSource), planName)

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

	err = s.db.RestorePlanVersion(ctx, dto.RestorePlanVersionParams{
		Plan: dto.UpdatePlanParams{
			FileName:   planName,
			SyncSource: syncSource,
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
		rollbackFiles(sourcePath, oldSource, viewerPath, oldViewer)
		_ = os.Remove(versionFilePath)
		return fmt.Errorf("failed to restore plan: %w", err)
	}

	return nil
}

// GetVersionCount returns the total number of versions for a plan.
func (s *Service) GetVersionCount(ctx context.Context, planName, syncSource string) (int64, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
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
func (s *Service) CleanupOldVersions(ctx context.Context, planName, syncSource string, maxVersions int64) error {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.GetPlanVersionHistory(ctx, dto.VersionHistoryParams{
		PlanID: plan.ID,
		Limit:  1000,
		Offset: 0,
	})
	if err != nil {
		return fmt.Errorf("failed to get versions for cleanup: %w", err)
	}

	if int64(len(versions)) > maxVersions {
		versionsToDelete := versions[maxVersions:]

		for _, version := range versionsToDelete {
			if err := os.Remove(version.FilePath); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("failed to delete old version file", "path", version.FilePath, "error", err)
			}
		}

		if len(versionsToDelete) > 0 {
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
func (s *Service) SearchVersions(ctx context.Context, planName, syncSource, query string) ([]PlanVersionDetail, error) {
	plan, err := s.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.SearchVersionsByContent(ctx, dto.SearchVersionsParams{
		PlanID: plan.ID,
		Query:  query,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

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
