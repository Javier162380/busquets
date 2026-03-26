package claudeviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"
)

// SavePlanVersion creates a new version of a plan.
// It writes the version file to disk first, then saves to database.
// If database save fails, it deletes the file to maintain consistency.
func (s *Service) SavePlanVersion(ctx context.Context, planName string, content string) error {
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
	lastVersionNumVal, err := s.db.GetLatestVersionNumber(ctx, plan.ID)
	if err != nil {
		return fmt.Errorf("failed to get latest version number: %w", err)
	}

	var lastVersionNum int64
	switch v := lastVersionNumVal.(type) {
	case int64:
		lastVersionNum = v
	case int:
		lastVersionNum = int64(v)
	default:
		lastVersionNum = 0
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
	err = s.db.InsertPlanVersion(ctx, repository.InsertPlanVersionParams{
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
func (s *Service) GetPlanVersionHistory(ctx context.Context, planName string, offset, limit int64) ([]repository.PlanVersion, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.GetPlanVersionHistory(ctx, repository.GetPlanVersionHistoryParams{
		PlanID: plan.ID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get version history: %w", err)
	}

	return versions, nil
}

// GetPlanVersion retrieves a specific version of a plan.
func (s *Service) GetPlanVersion(ctx context.Context, planName string, versionNumber int64) (*repository.PlanVersion, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	version, err := s.db.GetPlanVersionByNumber(ctx, repository.GetPlanVersionByNumberParams{
		PlanID:        plan.ID,
		VersionNumber: versionNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

	return &version, nil
}

// RestorePlanVersion restores a plan to a previous version.
// This creates a new version entry with the restored content.
func (s *Service) RestorePlanVersion(ctx context.Context, planName string, versionNumber int64) error {
	// Get the version to restore
	version, err := s.GetPlanVersion(ctx, planName, versionNumber)
	if err != nil {
		return fmt.Errorf("failed to get version to restore: %w", err)
	}

	// Update the current plan with the restored content
	updateReq := UpdatePlanRequest{
		FileName:         planName,
		NewContent:       version.Content,
		LastModifiedTime: time.Now(), // Don't check for conflicts when restoring
		Force:            true,        // Force override
	}

	result, err := s.UpdatePlan(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update plan with restored content: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("failed to restore plan: %w", err)
	}

	// Save the current state as a new version
	plan, err := s.db.GetPlanByFileName(ctx, planName)
	if err != nil {
		return fmt.Errorf("failed to get plan after restore: %w", err)
	}

	if err := s.SavePlanVersion(ctx, planName, plan.Content); err != nil {
		return fmt.Errorf("failed to save version after restore: %w", err)
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
	versions, err := s.db.GetPlanVersionHistory(ctx, repository.GetPlanVersionHistoryParams{
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
				// Log error but continue cleanup
				fmt.Printf("warning: failed to delete old version file: %v\n", err)
			}
		}

		// Delete from database (delete versions older than the cutoff version number)
		if len(versionsToDelete) > 0 {
			// The highest version number to delete (oldest versions we're removing)
			cutoffVersionNum := versionsToDelete[0].VersionNumber
			err = s.db.DeleteVersionsOlderThan(ctx, repository.DeleteVersionsOlderThanParams{
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
func (s *Service) SearchVersions(ctx context.Context, planName string, query string) ([]repository.PlanVersion, error) {
	// Get the plan to verify it exists and get its ID
	plan, err := s.GetPlanByFileName(ctx, planName)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	// Search versions with LIKE query (case-insensitive in SQLite)
	searchPattern := fmt.Sprintf("%%%s%%", query)
	versions, err := s.db.SearchVersionsByContent(ctx, repository.SearchVersionsByContentParams{
		PlanID: plan.ID,
		Content: searchPattern,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return versions, nil
}
