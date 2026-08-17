package busquets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Javier162380/busquets/services/busquets/dto"
)

// UpdatePlanRequest contains the data needed to update a plan.
type UpdatePlanRequest struct {
	FileName         string
	SyncSource       string // source directory path (stored in DB)
	NewContent       string
	LastModifiedTime time.Time // Client's version timestamp
	Force            bool      // If true, skip conflict check
}

// UpdatePlanResult contains the result of an update operation.
type UpdatePlanResult struct {
	Success      bool
	HasConflict  bool
	ConflictInfo *ConflictInfo
	Plan         *PlanDetail
}

// ConflictInfo describes a detected conflict.
type ConflictInfo struct {
	CurrentModTime  time.Time
	ExpectedModTime time.Time
	Message         string
}

// UpdatePlan updates a plan file and syncs it back to the source directory.
// It checks for conflicts by comparing timestamps.
func (s *Service) UpdatePlan(ctx context.Context, req UpdatePlanRequest) (*UpdatePlanResult, error) {
	plan, err := s.db.GetPlanByFileName(ctx, req.FileName, req.SyncSource)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	sourcePath := filepath.Join(req.SyncSource, req.FileName)
	viewerPath := plan.FilePath

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat source file: %w", err)
	}

	if !req.Force && sourceInfo.ModTime().After(req.LastModifiedTime) {
		return &UpdatePlanResult{
			Success:     false,
			HasConflict: true,
			ConflictInfo: &ConflictInfo{
				CurrentModTime:  sourceInfo.ModTime(),
				ExpectedModTime: req.LastModifiedTime,
				Message:         "The source file was modified externally since you started editing",
			},
		}, nil
	}

	title := extractTitle(req.NewContent)
	wordCount := CountWords(req.NewContent)

	contentToStore := req.NewContent
	if !s.indexFullContent {
		contentToStore = title
	}

	now := s.nowProvider.Now()

	updatedPlan, err := s.db.UpdatePlanContent(ctx, dto.UpdatePlanContentParams{
		Plan: dto.UpdatePlanParams{
			FileName:   req.FileName,
			SyncSource: req.SyncSource,
			Title:      title,
			Content:    contentToStore,
			IndexedAt:  now,
			WordCount:  int64(wordCount),
		},
		VersionContent:   req.NewContent,
		VersionCreatedAt: now,
	}, func() (time.Time, int64, error) {
		if err := os.WriteFile(sourcePath, []byte(req.NewContent), 0o600); err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to write to source directory: %w", err)
		}
		if err := os.WriteFile(viewerPath, []byte(req.NewContent), 0o600); err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to write to viewer directory: %w", err)
		}
		info, err := os.Stat(viewerPath)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to stat updated file: %w", err)
		}
		return info.ModTime(), info.Size(), nil
	},
		s.writeVersionFile(now, req.NewContent, wordCount))
	if err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	return s.invalidateCacheAndBuildUpdateResult(ctx, req.FileName, updatedPlan, req.NewContent, wordCount), nil
}

// invalidateCacheAndBuildUpdateResult invalidates the summary cache and
// builds the successful UpdatePlanResult for a plan UpdatePlanContent just
// wrote — shared by UpdatePlan and SavePlanLocal so this post-write
// bookkeeping lives in one place instead of being duplicated in both.
func (s *Service) invalidateCacheAndBuildUpdateResult(ctx context.Context, fileName string, updatedPlan dto.Plan, newContent string, wordCount int) *UpdatePlanResult {
	if s.summaryCache != nil {
		s.summaryCache.Delete(fileName)
	}

	detail := s.buildSavedPlanDetail(ctx, updatedPlan, newContent, wordCount)

	return &UpdatePlanResult{
		Success:     true,
		HasConflict: false,
		Plan:        detail,
	}
}

func (s *Service) writeVersionFile(createdAt time.Time, content string, wordCount int) func(planID, versionNumber int64) (string, int64, error) {
	return func(planID, versionNumber int64) (string, int64, error) {
		versionsDir := s.versionsDirFor(planID)
		if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
			if err := os.MkdirAll(versionsDir, 0o750); err != nil {
				return "", 0, fmt.Errorf("failed to create versions directory: %w", err)
			}
		}

		versionFileName := fmt.Sprintf("%d-%s.md", versionNumber, strconv.FormatInt(createdAt.Unix(), 10))
		versionFilePath := filepath.Join(versionsDir, versionFileName)

		if err := os.WriteFile(versionFilePath, []byte(content), 0o600); err != nil {
			return "", 0, fmt.Errorf("failed to write version file: %w", err)
		}

		return versionFilePath, int64(wordCount), nil
	}
}

// buildSavedPlanDetail turns the plan row UpdatePlanContent's transaction
// just handed back into a PlanDetail via planDetailFromRow. plan.Tags is
// already populated (UpdatePlanContent loads it in the same transaction as
// the write), so this only needs to compute reading time.
func (s *Service) buildSavedPlanDetail(ctx context.Context, plan dto.Plan, content string, wordCount int) *PlanDetail {
	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	readingTime := s.CalculateReadingTimeWithWPM(wordCount, readingSpeedWPM)

	return s.planDetailFromRow(plan, plan.Tags, content, readingTime)
}

// SavePlanLocal saves a plan file to the viewer directory only (no sync to source).
func (s *Service) SavePlanLocal(ctx context.Context, req UpdatePlanRequest) (*UpdatePlanResult, error) {
	plan, err := s.db.GetPlanByFileName(ctx, req.FileName, req.SyncSource)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}
	viewerPath := plan.FilePath

	viewerInfo, err := os.Stat(viewerPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat viewer file: %w", err)
	}

	if !req.Force && err == nil && viewerInfo.ModTime().After(req.LastModifiedTime) {
		return &UpdatePlanResult{
			Success:     false,
			HasConflict: true,
			ConflictInfo: &ConflictInfo{
				CurrentModTime:  viewerInfo.ModTime(),
				ExpectedModTime: req.LastModifiedTime,
				Message:         "The file was modified since you started editing",
			},
		}, nil
	}

	title := extractTitle(req.NewContent)
	wordCount := CountWords(req.NewContent)

	contentToStore := req.NewContent
	if !s.indexFullContent {
		contentToStore = title
	}

	now := s.nowProvider.Now()

	updatedPlan, err := s.db.UpdatePlanContent(ctx, dto.UpdatePlanContentParams{
		Plan: dto.UpdatePlanParams{
			FileName:   req.FileName,
			SyncSource: req.SyncSource,
			Title:      title,
			Content:    contentToStore,
			IndexedAt:  now,
			WordCount:  int64(wordCount),
		},
		VersionContent:   req.NewContent,
		VersionCreatedAt: now,
	}, func() (time.Time, int64, error) {
		if err := os.WriteFile(viewerPath, []byte(req.NewContent), 0o600); err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to write to viewer directory: %w", err)
		}
		info, err := os.Stat(viewerPath)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to stat updated file: %w", err)
		}
		return info.ModTime(), info.Size(), nil
	}, s.writeVersionFile(now, req.NewContent, wordCount))
	if err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	return s.invalidateCacheAndBuildUpdateResult(ctx, req.FileName, updatedPlan, req.NewContent, wordCount), nil
}
