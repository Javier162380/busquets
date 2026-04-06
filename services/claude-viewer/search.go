package claudeviewer

import (
	"context"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

func (s *Service) ListAllPlans(ctx context.Context) ([]dto.PlanSummary, error) {
	return s.db.ListAllPlans(ctx)
}

func (s *Service) ListAllPlansWithReadingTime(ctx context.Context) ([]PlanSummary, error) {
	plans, err := s.db.ListAllPlans(ctx)
	if err != nil {
		return nil, err
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	summaries := make([]PlanSummary, len(plans))
	for i, plan := range plans {
		summaries[i] = PlanSummary{
			ID:          plan.ID,
			FileName:    plan.FileName,
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt,
			ModifiedAt:  plan.ModifiedAt,
			FileSize:    plan.FileSize,
			ReadingTime: s.CalculateReadingTimeWithWPM(int(plan.WordCount), readingSpeedWPM),
		}
	}

	return summaries, nil
}

func (s *Service) SearchPlansWithReadingTime(ctx context.Context, query string) ([]PlanSummary, error) {
	if query == "" {
		return s.ListAllPlansWithReadingTime(ctx)
	}

	plans, err := s.db.SearchPlans(ctx, dto.SearchParams{Query: query})
	if err != nil {
		return nil, err
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	summaries := make([]PlanSummary, len(plans))
	for i, plan := range plans {
		summaries[i] = PlanSummary{
			ID:          plan.ID,
			FileName:    plan.FileName,
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt,
			ModifiedAt:  plan.ModifiedAt,
			FileSize:    plan.FileSize,
			ReadingTime: s.CalculateReadingTimeWithWPM(int(plan.WordCount), readingSpeedWPM),
		}
	}

	return summaries, nil
}

// ListAllPlansWithPaginationAndReadingTime returns paginated plans with reading time.
func (s *Service) ListAllPlansWithPaginationAndReadingTime(ctx context.Context, limit, offset int64) ([]PlanSummary, error) {
	plans, err := s.db.ListAllPlansWithPagination(ctx, dto.PaginationParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	summaries := make([]PlanSummary, len(plans))
	for i, plan := range plans {
		summaries[i] = PlanSummary{
			ID:          plan.ID,
			FileName:    plan.FileName,
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt,
			ModifiedAt:  plan.ModifiedAt,
			FileSize:    plan.FileSize,
			ReadingTime: s.CalculateReadingTimeWithWPM(int(plan.WordCount), readingSpeedWPM),
		}
	}

	return summaries, nil
}

// SearchPlansWithPaginationAndReadingTime returns paginated search results with reading time.
func (s *Service) SearchPlansWithPaginationAndReadingTime(ctx context.Context, query string, limit, offset int64) ([]PlanSummary, error) {
	if query == "" {
		return s.ListAllPlansWithPaginationAndReadingTime(ctx, limit, offset)
	}

	plans, err := s.db.SearchPlansWithPagination(ctx, dto.SearchPaginationParams{
		Query:  query,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	summaries := make([]PlanSummary, len(plans))
	for i, plan := range plans {
		summaries[i] = PlanSummary{
			ID:          plan.ID,
			FileName:    plan.FileName,
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt,
			ModifiedAt:  plan.ModifiedAt,
			FileSize:    plan.FileSize,
			ReadingTime: s.CalculateReadingTimeWithWPM(int(plan.WordCount), readingSpeedWPM),
		}
	}

	return summaries, nil
}
