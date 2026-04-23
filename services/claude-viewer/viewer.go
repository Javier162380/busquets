package claudeviewer

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// GetPlanByFileName retrieves a plan by its file name.
func (s *Service) GetPlanByFileName(ctx context.Context, fileName string) (*dto.Plan, error) {
	plan, err := s.db.GetPlanByFileName(ctx, fileName)
	if err != nil {
		return nil, err
	}
	// Get tags for the plan
	tags, err := s.db.GetPlanTags(ctx, plan.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tags: %w", err)
	}

	plan.Tags = tags

	return &plan, nil
}

func (s *Service) RenderMarkdown(content string) (string, error) {
	var buf bytes.Buffer
	if err := s.markdownHTMLRendered.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("failed to render markdown: %w", err)
	}

	return buf.String(), nil
}

func (s *Service) GetPlanDetailByFileName(ctx context.Context, fileName string) (*PlanDetail, error) {
	plan, err := s.GetPlanByFileName(ctx, fileName)
	if err != nil {
		return nil, err
	}

	// Get tags for the plan
	tags, err := s.db.GetPlanTags(ctx, plan.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tags: %w", err)
	}

	renderedHTML, err := s.RenderMarkdown(plan.Content)
	if err != nil {
		return nil, err
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	readingTime := s.CalculateReadingTimeWithWPM(int(plan.WordCount), readingSpeedWPM)

	return &PlanDetail{
		PlanSummary: PlanSummary{
			ID:          plan.ID,
			FileName:    plan.FileName,
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt,
			ModifiedAt:  plan.ModifiedAt,
			FileSize:    plan.FileSize,
			ReadingTime: readingTime,
			Tags:        tags,
		},
		Content:      plan.Content,
		RenderedHTML: renderedHTML,
	}, nil
}
