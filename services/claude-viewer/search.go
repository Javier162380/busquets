package claudeviewer

import (
	"context"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

func (s *Service) toSummaries(ctx context.Context, plans []dto.PlanSummary) []PlanSummary {
	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	summaries := make([]PlanSummary, len(plans))
	for i, plan := range plans {
		rt := int(plan.ReadingTime)
		if rt == 0 {
			rt = s.CalculateReadingTimeWithWPM(int(plan.WordCount), readingSpeedWPM)
		}
		summaries[i] = PlanSummary{
			ID:          plan.ID,
			FileName:    plan.FileName,
			SyncSource:  plan.SyncSource,
			SyncLabel:   s.labelForSource(plan.SyncSource),
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt,
			ModifiedAt:  plan.ModifiedAt,
			FileSize:    plan.FileSize,
			ReadingTime: rt,
			Tags:        plan.Tags,
		}
	}
	return summaries
}

func (s *Service) ListAllPlansWithReadingTime(ctx context.Context) ([]PlanSummary, error) {
	sortKey, sortDir := s.getSortSettings(ctx)
	wpm := s.GetReadingSpeedForDisplay(ctx)
	plans, err := s.db.ListAllPlans(ctx, sortKeyToColumn(sortKey), sortDir, wpm)
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}

func (s *Service) SearchPlansWithReadingTime(ctx context.Context, query string) ([]PlanSummary, error) {
	if query == "" {
		return s.ListAllPlansWithReadingTime(ctx)
	}

	plans, err := s.db.SearchPlans(ctx, dto.SearchParams{Query: query, SearchOver: s.resolveSearchScope(ctx)})
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
		Query:      query,
		Limit:      limit,
		Offset:     offset,
		SearchOver: s.resolveSearchScope(ctx),
	})
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}

// SearchPlansWithTags searches plans with text query and tag filters.
func (s *Service) SearchPlansWithTags(ctx context.Context, query string, tags []string, matchAll bool) ([]PlanSummary, error) {
	if query == "" && len(tags) == 0 {
		return s.ListAllPlansWithReadingTime(ctx)
	}

	plans, err := s.db.SearchPlansWithTags(ctx, dto.SearchParams{
		Query:      query,
		TagNames:   tags,
		MatchAll:   matchAll,
		SearchOver: s.resolveSearchScope(ctx),
	})
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}
