package busquets

import (
	"context"
	"fmt"

	"github.com/Javier162380/busquets/services/busquets/dto"
)

// AddComment attaches a new comment to a plan.
func (s *Service) AddComment(ctx context.Context, planFileName, syncSource, content string) (Comment, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planFileName, syncSource)
	if err != nil {
		return Comment{}, fmt.Errorf("failed to get plan: %w", err)
	}

	now := s.nowProvider.Now()
	c, err := s.db.InsertComment(ctx, dto.InsertCommentParams{
		PlanID:    plan.ID,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	})
	return toComment(c), err
}

// GetPlanComments returns all comments for a plan, ordered by created_at ASC.
func (s *Service) GetPlanComments(ctx context.Context, planFileName, syncSource string) ([]Comment, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planFileName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	cs, err := s.db.GetPlanComments(ctx, plan.ID)
	return toComments(cs), err
}

// DeleteComment removes a comment by ID.
func (s *Service) DeleteComment(ctx context.Context, commentID int64) error {
	return s.db.DeleteComment(ctx, commentID)
}
