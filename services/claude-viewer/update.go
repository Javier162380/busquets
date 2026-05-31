package claudeviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
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
	sourcePath := filepath.Join(req.SyncSource, req.FileName)
	viewerPath := filepath.Join(s.viewerDir, s.viewerSubdirFor(req.SyncSource), req.FileName)

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

	if err := os.WriteFile(sourcePath, []byte(req.NewContent), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write to source directory: %w", err)
	}

	if err := os.WriteFile(viewerPath, []byte(req.NewContent), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write to viewer directory: %w", err)
	}

	title := extractTitle(req.NewContent)
	wordCount := CountWords(req.NewContent)

	contentToStore := req.NewContent
	if !s.indexFullContent {
		contentToStore = title
	}

	info, err := os.Stat(viewerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat updated file: %w", err)
	}

	err = s.db.UpdatePlan(ctx, dto.UpdatePlanParams{
		FileName:   req.FileName,
		SyncSource: req.SyncSource,
		Title:      title,
		Content:    contentToStore,
		ModifiedAt: info.ModTime(),
		IndexedAt:  s.nowProvider.Now(),
		FileSize:   info.Size(),
		WordCount:  int64(wordCount),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update database: %w", err)
	}

	if err := s.SavePlanVersion(ctx, req.FileName, req.SyncSource, req.NewContent); err != nil {
		s.logger.Warn("failed to save plan version", "file", req.FileName, "error", err)
	}

	if s.summaryCache != nil {
		s.summaryCache.Delete(req.FileName)
	}

	return &UpdatePlanResult{
		Success:     true,
		HasConflict: false,
	}, nil
}

// SavePlanLocal saves a plan file to the viewer directory only (no sync to source).
func (s *Service) SavePlanLocal(ctx context.Context, req UpdatePlanRequest) (*UpdatePlanResult, error) {
	viewerPath := filepath.Join(s.viewerDir, s.viewerSubdirFor(req.SyncSource), req.FileName)

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

	if err := os.WriteFile(viewerPath, []byte(req.NewContent), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write to viewer directory: %w", err)
	}

	title := extractTitle(req.NewContent)
	wordCount := CountWords(req.NewContent)

	contentToStore := req.NewContent
	if !s.indexFullContent {
		contentToStore = title
	}

	info, err := os.Stat(viewerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat updated file: %w", err)
	}

	err = s.db.UpdatePlan(ctx, dto.UpdatePlanParams{
		FileName:   req.FileName,
		SyncSource: req.SyncSource,
		Title:      title,
		Content:    contentToStore,
		ModifiedAt: info.ModTime(),
		IndexedAt:  s.nowProvider.Now(),
		FileSize:   info.Size(),
		WordCount:  int64(wordCount),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update database: %w", err)
	}

	if err := s.SavePlanVersion(ctx, req.FileName, req.SyncSource, req.NewContent); err != nil {
		s.logger.Warn("failed to save plan version", "file", req.FileName, "error", err)
	}

	if s.summaryCache != nil {
		s.summaryCache.Delete(req.FileName)
	}

	return &UpdatePlanResult{
		Success:     true,
		HasConflict: false,
	}, nil
}
