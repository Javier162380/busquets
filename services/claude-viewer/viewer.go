package claudeviewer

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// GetPlanByFileName retrieves a plan by its file name and sync source.
func (s *Service) GetPlanByFileName(ctx context.Context, fileName, syncSource string) (*dto.Plan, error) {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return nil, err
	}
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

func (s *Service) GetPlanDetailByFileName(ctx context.Context, fileName, syncSource string) (*PlanDetail, error) {
	plan, err := s.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return nil, err
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
			SyncSource:  plan.SyncSource,
			SyncLabel:   s.labelForSource(plan.SyncSource),
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt,
			ModifiedAt:  plan.ModifiedAt,
			FileSize:    plan.FileSize,
			ReadingTime: readingTime,
			Tags:        toTags(plan.Tags),
		},
		FilePath:     plan.FilePath,
		Content:      plan.Content,
		RenderedHTML: renderedHTML,
	}, nil
}

// CopyToClipboard writes text to the system clipboard. The clipboard mode comes
// from the clipboard_mode setting (overridable via the PLAN_VIEWER_CLIPBOARD env
// var). Used to copy plan and version content that the caller already holds.
func (s *Service) CopyToClipboard(ctx context.Context, text string) error {
	if err := s.clipboard.Write(text, s.getClipboardMode(ctx)); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return nil
}
