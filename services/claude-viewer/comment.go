package claudeviewer

import (
	"context"
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// AddComment attaches a new comment to a plan.
func (s *Service) AddComment(ctx context.Context, planFileName, syncSource, content string) (dto.Comment, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planFileName, syncSource)
	if err != nil {
		return dto.Comment{}, fmt.Errorf("failed to get plan: %w", err)
	}

	now := s.nowProvider.Now()
	return s.db.InsertComment(ctx, dto.InsertCommentParams{
		PlanID:    plan.ID,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// GetPlanComments returns all comments for a plan, ordered by created_at ASC.
func (s *Service) GetPlanComments(ctx context.Context, planFileName, syncSource string) ([]dto.Comment, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planFileName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	return s.db.GetPlanComments(ctx, plan.ID)
}

// DeleteComment removes a comment by ID.
func (s *Service) DeleteComment(ctx context.Context, commentID int64) error {
	return s.db.DeleteComment(ctx, commentID)
}
