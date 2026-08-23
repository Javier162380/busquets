package busquets

import (
	"context"
	"fmt"

	"github.com/Javier162380/busquets/services/busquets/dto"
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

func (s *Service) GetPlanDetailByFileName(ctx context.Context, fileName, syncSource string) (*PlanDetail, error) {
	plan, err := s.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return nil, err
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	readingTime := s.CalculateReadingTimeWithWPM(int(plan.WordCount), readingSpeedWPM)

	return s.planDetailFromRow(*plan, plan.Tags, plan.Content, readingTime), nil
}

// planDetailFromRow maps a plan row, its tags, and a precomputed reading
// time into a PlanDetail. The single place this mapping lives, so callers
// that already hold these three pieces (from a load, or from a save that
// just wrote them) don't each restate the field list.
func (s *Service) planDetailFromRow(plan dto.Plan, tags []dto.Tag, content string, readingTime int) *PlanDetail {
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
			Tags:        toTags(tags),
		},
		FilePath: plan.FilePath,
		Content:  content,
	}
}

// CopyToClipboard writes text to the system clipboard. The clipboard mode comes
// from the clipboard_mode setting (overridable via the BUSQUETS_CLIPBOARD env
// var). Used to copy plan and version content that the caller already holds.
func (s *Service) CopyToClipboard(ctx context.Context, text string) error {
	if err := s.clipboard.Write(text, s.getClipboardMode(ctx)); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return nil
}
