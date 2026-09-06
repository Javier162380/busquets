package busquets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Javier162380/busquets/internal/gitdiff"
	"github.com/Javier162380/busquets/internal/retrier"
	"github.com/Javier162380/busquets/services/busquets/dto"
)

// SavePlanVersion creates a new version snapshot of a plan.
func (s *Service) SavePlanVersion(ctx context.Context, planName, syncSource, content string) error {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	versionsDir := s.versionsDirFor(plan.ID)
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

	wordCount := CountWords(content)
	err = s.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
		PlanID:        plan.ID,
		VersionNumber: nextVersionNum,
		FilePath:      versionFilePath,
		Content:       content,
		WordCount:     int64(wordCount),
		CreatedAt:     now,
	}, func() error {
		if err = os.WriteFile(versionFilePath, []byte(content), 0o600); err != nil {
			r := retrier.NewRetrier(3, 1*time.Second)
			deleteErr := r.Do(ctx, func(ctx context.Context) error {
				removeErr := os.Remove(versionFilePath)
				switch {
				case removeErr == nil:
					return nil
				case os.IsNotExist(removeErr):
					return retrier.ErrNonRetriableError
				default:
					return removeErr
				}
			})
			if deleteErr != nil {
				//nolint:errorlint // Need to include cleanup error as context
				return fmt.Errorf("failed to write version file: %w (also failed to clean up file: %v)", err, deleteErr)
			}
			return fmt.Errorf("failed to write version file: %w", err)
		}
		return nil
	})
	if err != nil {
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
		planVersionsDetail[i] = PlanVersionDetail{
			PlanVersion: planVersion,
			ReadingTime: s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM),
			Tags:        toTags(tags),
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

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	readingTime := s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM)

	return &PlanVersionDetail{
		PlanVersion: toPlanVersion(version),
		ReadingTime: readingTime,
		Tags:        toTags(tags),
	}, nil
}

// toPlanVersion maps a dto.PlanVersion row to the domain PlanVersion type.
func toPlanVersion(v dto.PlanVersion) PlanVersion {
	return PlanVersion{
		ID:            v.ID,
		PlanID:        v.PlanID,
		VersionNumber: v.VersionNumber,
		FilePath:      v.FilePath,
		Content:       v.Content,
		WordCount:     v.WordCount,
		CreatedAt:     v.CreatedAt,
	}
}

// DiffPlanVersions returns a unified diff of fromVersion against toVersion,
// plus both versions' metadata.
func (s *Service) DiffPlanVersions(ctx context.Context, fileName, syncSource string, fromVersion, toVersion int64) (VersionDiff, error) {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return VersionDiff{}, fmt.Errorf("plan not found: %w", err)
	}

	from, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, fromVersion)
	if err != nil {
		return VersionDiff{}, fmt.Errorf("version %d not found: %w", fromVersion, err)
	}
	to, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, toVersion)
	if err != nil {
		return VersionDiff{}, fmt.Errorf("version %d not found: %w", toVersion, err)
	}

	diff, err := gitdiff.Diff(from.Content, to.Content,
		fmt.Sprintf("Version %d", from.VersionNumber), fmt.Sprintf("Version %d", to.VersionNumber))
	if err != nil {
		return VersionDiff{}, fmt.Errorf("failed to build diff: %w", err)
	}

	return VersionDiff{Diff: diff, From: toPlanVersion(from), To: toPlanVersion(to)}, nil
}

// RestorePlanVersion restores a plan to a previous version: writes that
// version's content back to the source and viewer files, updates the plan
// row, and records the restore itself as a new version — atomically via
// UpdatePlanContent, same as UpdatePlan/SavePlanLocal.
func (s *Service) RestorePlanVersion(ctx context.Context, planID, versionNumber int64) error {
	plan, err := s.db.GetPlanByID(ctx, planID)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	version, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, versionNumber)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	sourcePath := filepath.Join(plan.SyncSource, plan.FileName)
	viewerPath := plan.FilePath

	title := extractTitle(version.Content)
	wordCount := CountWords(version.Content)

	contentToStore := version.Content
	if !s.indexFullContent {
		contentToStore = title
	}

	now := s.nowProvider.Now()

	_, err = s.db.UpdatePlanContent(ctx, dto.UpdatePlanContentParams{
		Plan: dto.UpdatePlanParams{
			FileName:   plan.FileName,
			SyncSource: plan.SyncSource,
			Title:      title,
			Content:    contentToStore,
			IndexedAt:  now,
			WordCount:  int64(wordCount),
		},
		VersionContent:   version.Content,
		VersionCreatedAt: now,
	}, func() (time.Time, int64, error) {
		if err := os.WriteFile(sourcePath, []byte(version.Content), 0o600); err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to write source file: %w", err)
		}
		if err := os.WriteFile(viewerPath, []byte(version.Content), 0o600); err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to write viewer file: %w", err)
		}
		info, err := os.Stat(viewerPath)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to stat restored file: %w", err)
		}
		return info.ModTime(), info.Size(), nil
	}, s.writeVersionFile(now, version.Content, wordCount))
	if err != nil {
		return fmt.Errorf("failed to restore plan: %w", err)
	}

	return nil
}

// RestorePlanVersionByFileName resolves a plan by fileName+syncSource and restores it to the
// given version — same as RestorePlanVersion, but callers (like the MCP layer) don't need to
// already know the plan's internal ID.
func (s *Service) RestorePlanVersionByFileName(ctx context.Context, fileName, syncSource string, versionNumber int64) error {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}
	return s.RestorePlanVersion(ctx, plan.ID, versionNumber)
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
		planVersionsDetail[i] = PlanVersionDetail{
			PlanVersion: planVersion,
			ReadingTime: s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM),
			Tags:        toTags(plan.Tags),
		}
	}

	return planVersionsDetail, nil
}
