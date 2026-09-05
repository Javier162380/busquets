package busquets

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Javier162380/busquets/internal/retrier"
	"github.com/Javier162380/busquets/services/busquets/dto"
)

var validCharsRegex = regexp.MustCompile(`[^a-z0-9_-]+`)

const maxTagLength = 64

// NormalizeTags validates and normalizes tag names: lowercase, strip invalid
// characters, deduplicate, drop oversized tags, and sort for deterministic output.
func NormalizeTags(tags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))

	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		normalized = validCharsRegex.ReplaceAllString(normalized, "")
		if normalized == "" || len(normalized) > maxTagLength {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	sort.Strings(result)
	return result
}

// CreateTag creates a new tag with the given name, description, and color.
func (s *Service) CreateTag(ctx context.Context, name string, description, color *string) (Tag, error) {
	normalized := NormalizeTags([]string{name})
	if len(normalized) == 0 {
		return Tag{}, fmt.Errorf("invalid tag name: %s", name)
	}

	now := s.nowProvider.Now()
	t, err := s.db.InsertTag(ctx, dto.InsertTagParams{
		Name:        normalized[0],
		Description: description,
		Color:       color,
		CreatedAt:   now,
		ModifiedAt:  now,
	})
	return toTag(t), err
}

// GetAllTags returns all tags sorted by name.
func (s *Service) GetAllTags(ctx context.Context) ([]Tag, error) {
	ts, err := s.db.ListAllTags(ctx)
	return toTags(ts), err
}

// GetTagPlanCounts returns a map of tag name → number of plans tagged with it.
func (s *Service) GetTagPlanCounts(ctx context.Context) (map[string]int, error) {
	return s.db.GetTagPlanCounts(ctx)
}

// GetUntaggedPlanCount returns the number of plans with no tags.
func (s *Service) GetUntaggedPlanCount(ctx context.Context) (int64, error) {
	return s.db.GetUntaggedPlanCount(ctx)
}

// ListPlansWithTags returns all plans with their tags reliably populated via a single JOIN query.
// Plans with no tags have an empty Tags slice.
func (s *Service) ListPlansWithTags(ctx context.Context) ([]PlanSummary, error) {
	plans, err := s.db.ListPlansWithTags(ctx)
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}

// BuildTagPlanMap returns a map of tag name → plans carrying that tag.
// Plans with no tags are stored under the empty string key "".
func (s *Service) BuildTagPlanMap(ctx context.Context) (map[string][]PlanSummary, error) {
	sortKey, sortDir := s.getSortSettings(ctx)
	plans, err := s.ListPlansWithTags(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string][]PlanSummary)
	for _, p := range plans {
		if len(p.Tags) == 0 {
			m[""] = append(m[""], p)
		}
		for _, t := range p.Tags {
			if t.Name != "" {
				m[t.Name] = append(m[t.Name], p)
			}
		}
	}
	for tag := range m {
		sortPlanSummaries(m[tag], sortKey, sortDir)
	}
	return m, nil
}

// ListUntaggedPlansWithReadingTime returns plans with no tags, including reading time.
func (s *Service) ListUntaggedPlansWithReadingTime(ctx context.Context) ([]PlanSummary, error) {
	sortKey, sortDir := s.getSortSettings(ctx)
	wpm := s.GetReadingSpeedForDisplay(ctx)
	plans, err := s.db.ListUntaggedPlans(ctx, sortKeyToColumn(sortKey), sortDir, wpm)
	if err != nil {
		return nil, err
	}
	return s.toSummaries(ctx, plans), nil
}

// DeleteTag deletes a tag by ID and removes all plan-tag associations.
func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	if _, err := s.db.GetTagByID(ctx, id); err != nil {
		return fmt.Errorf("failed to get tag: %w", err)
	}
	return s.db.DeleteTag(ctx, id)
}

// resolveTagIDs converts tag names to IDs, creating any that don't exist.
// Retries handle the concurrent sync race where two goroutines try to create the same tag simultaneously.
func (s *Service) resolveTagIDs(ctx context.Context, tagNames []string, now time.Time) ([]int64, error) {
	r := retrier.NewRetrier(3, 50*time.Millisecond)
	tagIDs := make([]int64, 0, len(tagNames))

	for _, tagName := range tagNames {
		var tag dto.Tag
		err := r.Do(ctx, func(ctx context.Context) error {
			t, err := s.db.GetTagByName(ctx, tagName)
			if dto.IsNotFound(err) {
				t, err = s.db.InsertTag(ctx, dto.InsertTagParams{
					Name:       tagName,
					CreatedAt:  now,
					ModifiedAt: now,
				})
			}
			if err != nil {
				return err
			}
			tag = t
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get or create tag %s: %w", tagName, err)
		}
		tagIDs = append(tagIDs, tag.ID)
	}
	return tagIDs, nil
}

// SetPlanTags sets the tags for a plan, replacing any existing tags.
// Tag names will be normalized (lowercase, trimmed).
// If a tag doesn't exist, it will be created automatically.
func (s *Service) SetPlanTags(ctx context.Context, fileName, syncSource string, tagNames []string) error {
	tagNames = NormalizeTags(tagNames)

	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	now := s.nowProvider.Now()
	tagIDs, err := s.resolveTagIDs(ctx, tagNames, now)
	if err != nil {
		return err
	}

	return s.db.SetPlanTags(ctx, plan.ID, tagIDs, now)
}

// SetPlanTagsAndGet replaces a plan's tags and returns the resulting list, atomically —
// tag names will be normalized (lowercase, trimmed); unknown tags are created automatically.
func (s *Service) SetPlanTagsAndGet(ctx context.Context, fileName, syncSource string, tagNames []string) ([]Tag, error) {
	tagNames = NormalizeTags(tagNames)

	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	now := s.nowProvider.Now()
	tagIDs, err := s.resolveTagIDs(ctx, tagNames, now)
	if err != nil {
		return nil, err
	}

	tags, err := s.db.SetPlanTagsAndGet(ctx, plan.ID, tagIDs, now)
	return toTags(tags), err
}

// RemovePlanTag removes one tag from one plan by name, atomically. Returns an error if
// the tag doesn't exist at all, rather than silently succeeding as a no-op.
func (s *Service) RemovePlanTag(ctx context.Context, fileName, syncSource, tagName string) ([]Tag, error) {
	tagName = NormalizeTags([]string{tagName})[0]

	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	tag, err := s.db.GetTagByName(ctx, tagName)
	if dto.IsNotFound(err) {
		return nil, fmt.Errorf("tag %q not found", tagName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to look up tag: %w", err)
	}

	tags, err := s.db.RemoveTagFromPlanAndGet(ctx, plan.ID, tag.ID)
	return toTags(tags), err
}

// GetPlanTags returns all tags associated with a plan.
func (s *Service) GetPlanTags(ctx context.Context, fileName, syncSource string) ([]Tag, error) {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	ts, err := s.db.GetPlanTags(ctx, plan.ID)
	return toTags(ts), err
}
