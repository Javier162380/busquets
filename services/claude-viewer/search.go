package claudeviewer

import (
	"context"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

func (s *Service) toSummaries(ctx context.Context, plans []dto.PlanSummary) []PlanSummary {
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
			Tags:        plan.Tags,
		}
	}
	return summaries
}

func (s *Service) ListAllPlans(ctx context.Context) ([]dto.PlanSummary, error) {
	return s.db.ListAllPlans(ctx)
}

func (s *Service) ListAllPlansWithReadingTime(ctx context.Context, sortKey, sortDir string) ([]PlanSummary, error) {
	plans, err := s.db.ListAllPlansSorted(ctx, sortKeyToColumn(sortKey), sortDir)
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}

func (s *Service) SearchPlansWithReadingTime(ctx context.Context, query string) ([]PlanSummary, error) {
	if query == "" {
		return s.ListAllPlansWithReadingTime(ctx, DefaultPlansSortKey, DefaultSortDir)
	}

	plans, err := s.db.SearchPlans(ctx, dto.SearchParams{Query: query})
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
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
	return s.toSummaries(ctx, plans), nil
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
	return s.toSummaries(ctx, plans), nil
}

// SearchPlansWithTags searches plans with text query and tag filters.
func (s *Service) SearchPlansWithTags(ctx context.Context, query string, tags []string, matchAll bool) ([]PlanSummary, error) {
	if query == "" && len(tags) == 0 {
		return s.ListAllPlansWithReadingTime(ctx, DefaultPlansSortKey, DefaultSortDir)
	}

	plans, err := s.db.SearchPlansWithTags(ctx, dto.SearchParams{
		Query:    query,
		TagNames: tags,
		MatchAll: matchAll,
	})
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}
