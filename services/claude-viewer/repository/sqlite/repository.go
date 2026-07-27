// Package sqlite represents the sqlite repository.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/huandu/go-sqlbuilder"
)

// Repository implements dto.Repository for SQLite.
type Repository struct {
	db *sql.DB
	q  *Queries
}

// NewRepository creates a new SQLite repository.
func NewRepository(db DBTX) *Repository {
	// Type assert to *sql.DB for transaction support
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		// If not *sql.DB, create repository without transaction support
		return &Repository{q: New(db)}
	}
	return &Repository{
		db: sqlDB,
		q:  New(db),
	}
}

// Verify interface compliance at compile time.
var _ dto.Repository = (*Repository)(nil)

func (r *Repository) withTx(ctx context.Context, fn func(*Queries) error) error {
	if r.db == nil {
		return errors.New("transaction support not available")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(r.q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Type conversion helpers: sql.Null* -> Go pointers

func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func ptrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullFloat64ToPtr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

func ptrToNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullBoolToPtr(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	return &nb.Bool
}

func ptrToNullBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func nullTimeToPtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

func ptrToNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// parseCommaSeparatedTags converts GROUP_CONCAT results into []dto.Tag.
func parseCommaSeparatedTags(tagIdsStr, tagNamesStr string) []dto.Tag {
	// Handle empty case (plan with no tags)
	if tagIdsStr == "" || tagNamesStr == "" {
		return []dto.Tag{}
	}

	// Split comma-separated strings
	idStrings := strings.Split(tagIdsStr, ",")
	names := strings.Split(tagNamesStr, ",")

	// Ensure arrays have same length
	minLen := len(idStrings)
	if len(names) < minLen {
		minLen = len(names)
	}

	tags := make([]dto.Tag, 0, minLen)
	for i := 0; i < minLen; i++ {
		// Parse ID from string to int64
		id, err := strconv.ParseInt(strings.TrimSpace(idStrings[i]), 10, 64)
		if err != nil {
			continue // Skip invalid IDs
		}

		tags = append(tags, dto.Tag{
			ID:   id,
			Name: strings.TrimSpace(names[i]),
		})
	}

	return tags
}

// Model conversions: SQLC types -> domain types

func planToDomain(p Plan) dto.Plan {
	return dto.Plan{
		ID:         p.ID,
		FileName:   p.FileName,
		SyncSource: p.SyncSource,
		FilePath:   p.FilePath,
		Title:      p.Title,
		Content:    p.Content,
		CreatedAt:  p.CreatedAt,
		ModifiedAt: p.ModifiedAt,
		IndexedAt:  p.IndexedAt,
		FileSize:   p.FileSize,
		WordCount:  p.WordCount,
	}
}

func planSummaryFromPaginationRow(r ListAllPlansWithPaginationRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         r.ID,
		FileName:   r.FileName,
		SyncSource: r.SyncSource,
		Title:      r.Title,
		CreatedAt:  r.CreatedAt,
		ModifiedAt: r.ModifiedAt,
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func planSummaryFromListWithTagsRow(r ListAllPlansWithTagsRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         r.ID,
		FileName:   r.FileName,
		SyncSource: r.SyncSource,
		Title:      r.Title,
		CreatedAt:  r.CreatedAt,
		ModifiedAt: r.ModifiedAt,
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
		Tags:       parseCommaSeparatedTags(r.TagIds, r.TagNames),
	}
}

func planVersionToDomain(pv PlanVersion) dto.PlanVersion {
	return dto.PlanVersion{
		ID:            pv.ID,
		PlanID:        pv.PlanID,
		VersionNumber: pv.VersionNumber,
		FilePath:      pv.FilePath,
		Content:       pv.Content,
		WordCount:     pv.WordCount,
		CreatedAt:     pv.CreatedAt,
	}
}

func connectorToDomain(c Connector) dto.Connector {
	return dto.Connector{
		Name:        c.Name,
		DisplayName: c.DisplayName,
		Enabled:     c.Enabled,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func connectorSettingToDomain(cs ConnectorSetting) dto.ConnectorSetting {
	return dto.ConnectorSetting{
		ID:            cs.ID,
		ConnectorName: cs.ConnectorName,
		SettingKey:    cs.SettingKey,
		SettingValue:  cs.SettingValue,
		IsSecret:      cs.IsSecret,
	}
}

func settingToDomain(s Setting) dto.Setting {
	return dto.Setting{
		VariableName:  s.VariableName,
		VariableType:  s.VariableType,
		StringValue:   nullStringToPtr(s.StringValue),
		NumberValue:   nullFloat64ToPtr(s.NumberValue),
		BooleanValue:  nullBoolToPtr(s.BooleanValue),
		DatetimeValue: nullTimeToPtr(s.DatetimeValue),
	}
}

// Plan operations

func (r *Repository) CountPlans(ctx context.Context) (int64, error) {
	return r.q.CountPlans(ctx)
}

func (r *Repository) GetPlanByFileName(ctx context.Context, fileName, syncSource string) (dto.Plan, error) {
	p, err := r.q.GetPlanByFileNameAndSource(ctx, GetPlanByFileNameAndSourceParams{
		FileName:   fileName,
		SyncSource: syncSource,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Plan{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Plan{}, err
	}
	return planToDomain(p), nil
}

func (r *Repository) InsertPlan(ctx context.Context, params dto.InsertPlanParams, writeFile func(id int64) (string, error)) (int64, error) {
	var id int64
	err := r.withTx(ctx, func(q *Queries) error {
		insertedID, err := q.InsertPlan(ctx, InsertPlanParams{
			FileName:   params.FileName,
			SyncSource: params.SyncSource,
			FilePath:   params.FilePath,
			Title:      params.Title,
			Content:    params.Content,
			CreatedAt:  params.CreatedAt,
			ModifiedAt: params.ModifiedAt,
			IndexedAt:  params.IndexedAt,
			FileSize:   params.FileSize,
			WordCount:  params.WordCount,
		})
		if err != nil {
			return fmt.Errorf("failed to insert plan: %w", err)
		}

		filePath, err := writeFile(insertedID)
		if err != nil {
			return err
		}

		if err := q.UpdatePlanFilePath(ctx, UpdatePlanFilePathParams{FilePath: filePath, ID: insertedID}); err != nil {
			return fmt.Errorf("failed to set plan file path: %w", err)
		}

		id = insertedID
		return nil
	})
	return id, err
}

func (r *Repository) UpdatePlanFilePath(ctx context.Context, id int64, filePath string, writeFile func() error) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := writeFile(); err != nil {
			return err
		}
		return q.UpdatePlanFilePath(ctx, UpdatePlanFilePathParams{FilePath: filePath, ID: id})
	})
}

func (r *Repository) UpdatePlanVersionFilePath(ctx context.Context, id int64, filePath string, writeFile func() error) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := writeFile(); err != nil {
			return err
		}
		return q.UpdatePlanVersionFilePath(ctx, UpdatePlanVersionFilePathParams{FilePath: filePath, ID: id})
	})
}

func (r *Repository) ListAllPlansFull(ctx context.Context) ([]dto.Plan, error) {
	rows, err := r.q.ListAllPlansFull(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Plan, len(rows))
	for i, row := range rows {
		result[i] = dto.Plan{
			ID:         row.ID,
			FileName:   row.FileName,
			SyncSource: row.SyncSource,
			FilePath:   row.FilePath,
			Content:    row.Content,
		}
	}
	return result, nil
}

func (r *Repository) ListPlanVersionsAll(ctx context.Context, planID int64) ([]dto.PlanVersion, error) {
	rows, err := r.q.ListPlanVersionsAll(ctx, planID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanVersion, len(rows))
	for i, row := range rows {
		result[i] = planVersionToDomain(row)
	}
	return result, nil
}

func (r *Repository) UpdatePlan(ctx context.Context, params dto.UpdatePlanParams) error {
	return r.q.UpdatePlan(ctx, UpdatePlanParams{
		FileName:   params.FileName,
		SyncSource: params.SyncSource,
		Title:      params.Title,
		Content:    params.Content,
		ModifiedAt: params.ModifiedAt,
		IndexedAt:  params.IndexedAt,
		FileSize:   params.FileSize,
		WordCount:  params.WordCount,
	})
}

func (r *Repository) InsertPlanWithTags(ctx context.Context, params dto.InsertPlanWithTagsParams) error {
	return r.withTx(ctx, func(q *Queries) error {
		if _, err := q.InsertPlan(ctx, InsertPlanParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
			FilePath:   params.Plan.FilePath,
			Title:      params.Plan.Title,
			Content:    params.Plan.Content,
			CreatedAt:  params.Plan.CreatedAt,
			ModifiedAt: params.Plan.ModifiedAt,
			IndexedAt:  params.Plan.IndexedAt,
			FileSize:   params.Plan.FileSize,
			WordCount:  params.Plan.WordCount,
		}); err != nil {
			return fmt.Errorf("failed to insert plan: %w", err)
		}

		plan, err := q.GetPlanByFileNameAndSource(ctx, GetPlanByFileNameAndSourceParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
		})
		if err != nil {
			return fmt.Errorf("failed to get inserted plan: %w", err)
		}

		for _, tagID := range params.TagIDs {
			if err := q.AddTagToPlan(ctx, AddTagToPlanParams{
				PlanID:     plan.ID,
				TagID:      tagID,
				AssignedAt: params.AssignedAt,
			}); err != nil {
				return fmt.Errorf("failed to add tag to plan: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) UpdatePlanWithTags(ctx context.Context, params dto.UpdatePlanWithTagsParams) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := q.UpdatePlan(ctx, UpdatePlanParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
			Title:      params.Plan.Title,
			Content:    params.Plan.Content,
			ModifiedAt: params.Plan.ModifiedAt,
			IndexedAt:  params.Plan.IndexedAt,
			FileSize:   params.Plan.FileSize,
			WordCount:  params.Plan.WordCount,
		}); err != nil {
			return fmt.Errorf("failed to update plan: %w", err)
		}

		plan, err := q.GetPlanByFileNameAndSource(ctx, GetPlanByFileNameAndSourceParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
		})
		if err != nil {
			return fmt.Errorf("failed to get plan: %w", err)
		}

		if err := q.RemoveAllTagsFromPlan(ctx, plan.ID); err != nil {
			return fmt.Errorf("failed to clear plan tags: %w", err)
		}

		for _, tagID := range params.TagIDs {
			if err := q.AddTagToPlan(ctx, AddTagToPlanParams{
				PlanID:     plan.ID,
				TagID:      tagID,
				AssignedAt: params.AssignedAt,
			}); err != nil {
				return fmt.Errorf("failed to add tag to plan: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) DeletePlan(ctx context.Context, fileName, syncSource string) error {
	return r.withTx(ctx, func(q *Queries) error {
		plan, err := q.GetPlanByFileNameAndSource(ctx, GetPlanByFileNameAndSourceParams{
			FileName:   fileName,
			SyncSource: syncSource,
		})
		if err != nil {
			return fmt.Errorf("failed to get plan: %w", err)
		}

		if err := q.RemoveAllTagsFromPlan(ctx, plan.ID); err != nil {
			return fmt.Errorf("failed to delete plan_tags associations: %w", err)
		}

		if err := q.DeletePlanVersions(ctx, plan.ID); err != nil {
			return fmt.Errorf("failed to delete plan_versions: %w", err)
		}

		if err := q.DeletePlanComments(ctx, plan.ID); err != nil {
			return fmt.Errorf("failed to delete plan_comments: %w", err)
		}

		if err := q.DeletePlan(ctx, DeletePlanParams{FileName: fileName, SyncSource: syncSource}); err != nil {
			return fmt.Errorf("failed to delete plan: %w", err)
		}
		return nil
	})
}

func (r *Repository) RenamePlanFile(ctx context.Context, params dto.RenamePlanFileParams, renameFiles func() error) error {
	return r.withTx(ctx, func(q *Queries) error {
		// Versions live under a plan-id-keyed directory (see versionsDirFor) —
		// nothing about them depends on the plan's file name, so a rename never
		// needs to touch plan_versions at all.
		if err := q.RenamePlanRow(ctx, RenamePlanRowParams{
			NewFileName: params.NewFileName,
			NewFilePath: params.NewFilePath,
			OldFileName: params.OldFileName,
			SyncSource:  params.SyncSource,
		}); err != nil {
			return fmt.Errorf("failed to rename plan: %w", err)
		}

		// Rename the files while the transaction is still open. Returning an error here
		// rolls the DB writes above back with it, so the rename is atomic across DB
		// and filesystem — no separate compensating write.
		if renameFiles != nil {
			if err := renameFiles(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ListAllPlans(ctx context.Context, sortCol, sortDir string, wpm int) ([]dto.PlanSummary, error) {
	sb := sqlbuilder.SQLite.NewSelectBuilder()
	rtExpr := fmt.Sprintf("MAX(1, (p.word_count + %d - 1) / %d) AS reading_time", wpm, wpm)
	sb.Select("p.id", "p.file_name", "p.sync_source", "p.title", "p.created_at", "p.modified_at", "p.file_size", "p.word_count", rtExpr, "COALESCE(cc.comment_count, 0) AS comment_count").
		From("plans p").
		JoinWithOption(sqlbuilder.LeftJoin,
			"(SELECT plan_id, COUNT(*) AS comment_count FROM plan_comments GROUP BY plan_id) cc",
			"cc.plan_id = p.id",
		)
	if sortCol == "reading_time" {
		if sortDir == "asc" {
			sb.OrderByAsc("reading_time")
		} else {
			sb.OrderByDesc("reading_time")
		}
	} else {
		applyOrder(sb, "p."+sortCol, sortDir)
	}
	q, args := sb.Build()
	return r.queryPlanSummaries(ctx, q, args...)
}

func (r *Repository) ListAllPlansWithPagination(ctx context.Context, params dto.PaginationParams) ([]dto.PlanSummary, error) {
	rows, err := r.q.ListAllPlansWithPagination(ctx, ListAllPlansWithPaginationParams{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanSummary, len(rows))
	for i, row := range rows {
		result[i] = planSummaryFromPaginationRow(row)
	}
	return result, nil
}

func (r *Repository) SearchPlans(ctx context.Context, params dto.SearchParams) ([]dto.PlanSummary, error) {
	return r.searchPlansDynamic(ctx, "%"+params.Query+"%", params.SearchOver)
}

func (r *Repository) SearchPlansWithPagination(ctx context.Context, params dto.SearchPaginationParams) ([]dto.PlanSummary, error) {
	return r.searchPlansPaginationDynamic(ctx, "%"+params.Query+"%", params)
}

func (r *Repository) searchPlansDynamic(ctx context.Context, pattern string, searchOver dto.SearchField) ([]dto.PlanSummary, error) {
	sb := sqlbuilder.SQLite.NewSelectBuilder()
	sb.Select(
		"p.id", "p.file_name", "p.sync_source", "p.title",
		"p.created_at", "p.modified_at", "p.file_size", "p.word_count",
		"COALESCE(t.tag_ids, '') AS tag_ids",
		"COALESCE(t.tag_names, '') AS tag_names",
		"COALESCE(cc.comment_count, 0) AS comment_count",
	).From("plans p").
		JoinWithOption(sqlbuilder.LeftJoin,
			`(SELECT pt.plan_id,
			         GROUP_CONCAT(DISTINCT t.id)   AS tag_ids,
			         GROUP_CONCAT(DISTINCT t.name) AS tag_names
			  FROM plan_tags pt
			  JOIN tags t ON pt.tag_id = t.id
			  GROUP BY pt.plan_id) t`,
			"t.plan_id = p.id",
		).
		JoinWithOption(sqlbuilder.LeftJoin,
			"(SELECT plan_id, COUNT(*) AS comment_count FROM plan_comments GROUP BY plan_id) cc",
			"cc.plan_id = p.id",
		).
		OrderByDesc("p.modified_at")

	switch searchOver {
	case dto.SearchOverPlanName:
		sb.Where(sb.Like("p.title", pattern))
	case dto.SearchOverContent:
		sb.Where(sb.Like("p.content", pattern))
	default:
		sb.Where(sb.Or(sb.Like("p.title", pattern), sb.Like("p.content", pattern)))
	}

	q, args := sb.Build()
	return r.queryPlanSummariesWithTags(ctx, q, args...)
}

func (r *Repository) searchPlansPaginationDynamic(ctx context.Context, pattern string, params dto.SearchPaginationParams) ([]dto.PlanSummary, error) {
	sb := sqlbuilder.SQLite.NewSelectBuilder()
	sb.Select(
		"p.id", "p.file_name", "p.sync_source", "p.title",
		"p.created_at", "p.modified_at", "p.file_size", "p.word_count",
		"COALESCE(t.tag_ids, '') AS tag_ids",
		"COALESCE(t.tag_names, '') AS tag_names",
		"COALESCE(cc.comment_count, 0) AS comment_count",
	).From("plans p").
		JoinWithOption(sqlbuilder.LeftJoin,
			`(SELECT pt.plan_id,
			         GROUP_CONCAT(DISTINCT t.id)   AS tag_ids,
			         GROUP_CONCAT(DISTINCT t.name) AS tag_names
			  FROM plan_tags pt
			  JOIN tags t ON pt.tag_id = t.id
			  GROUP BY pt.plan_id) t`,
			"t.plan_id = p.id",
		).
		JoinWithOption(sqlbuilder.LeftJoin,
			"(SELECT plan_id, COUNT(*) AS comment_count FROM plan_comments GROUP BY plan_id) cc",
			"cc.plan_id = p.id",
		).
		OrderByDesc("p.modified_at").
		Limit(int(params.Limit)).Offset(int(params.Offset))

	switch params.SearchOver {
	case dto.SearchOverPlanName:
		sb.Where(sb.Like("p.title", pattern))
	case dto.SearchOverContent:
		sb.Where(sb.Like("p.content", pattern))
	default:
		sb.Where(sb.Or(sb.Like("p.title", pattern), sb.Like("p.content", pattern)))
	}

	q, args := sb.Build()
	return r.queryPlanSummariesWithTags(ctx, q, args...)
}

func (r *Repository) SearchPlansWithTags(ctx context.Context, params dto.SearchParams) ([]dto.PlanSummary, error) {
	// If no tags, just do regular search
	if len(params.TagNames) == 0 {
		return r.SearchPlans(ctx, params)
	}

	// Strategy: Get all tag IDs first, then use SQLC queries
	// This avoids dynamic IN clauses while still using SQLC

	// Step 1: Get all plans (filtered by text search if query provided)
	var allPlans []dto.PlanSummary
	var err error

	if params.Query == "" {
		// No text search, get all plans
		allPlans, err = r.ListAllPlans(ctx, "modified_at", "desc", 200)
	} else {
		// Text search
		allPlans, err = r.SearchPlans(ctx, params)
	}
	if err != nil {
		return nil, err
	}

	// Step 2: Filter plans by tags
	var result []dto.PlanSummary
	for _, plan := range allPlans {
		// Get tags for this plan
		planTags, err := r.GetPlanTags(ctx, plan.ID)
		if err != nil {
			return nil, err
		}

		// Check if plan matches tag filter
		if r.matchesTagFilter(planTags, params.TagNames, params.MatchAll) {
			plan.Tags = planTags
			result = append(result, plan)
		}
	}

	return result, nil
}

// matchesTagFilter checks if a plan's tags match the filter criteria.
func (r *Repository) matchesTagFilter(planTags []dto.Tag, filterTags []string, matchAll bool) bool {
	if len(filterTags) == 0 {
		return true
	}

	// Build a map of plan tag names
	planTagMap := make(map[string]bool)
	for _, tag := range planTags {
		planTagMap[strings.ToLower(tag.Name)] = true
	}

	matchCount := 0
	for _, filterTag := range filterTags {
		if planTagMap[strings.ToLower(filterTag)] {
			matchCount++
			if !matchAll {
				// OR logic: at least one match is enough
				return true
			}
		}
	}

	if matchAll {
		// AND logic: all tags must match
		return matchCount == len(filterTags)
	}

	return false
}

// Plan version operations

func (r *Repository) RestorePlanVersion(ctx context.Context, params dto.RestorePlanVersionParams) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := q.UpdatePlan(ctx, UpdatePlanParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
			Title:      params.Plan.Title,
			Content:    params.Plan.Content,
			ModifiedAt: params.Plan.ModifiedAt,
			IndexedAt:  params.Plan.IndexedAt,
			FileSize:   params.Plan.FileSize,
			WordCount:  params.Plan.WordCount,
		}); err != nil {
			return fmt.Errorf("failed to update plan: %w", err)
		}

		if err := q.InsertPlanVersion(ctx, InsertPlanVersionParams{
			PlanID:        params.Version.PlanID,
			VersionNumber: params.Version.VersionNumber,
			FilePath:      params.Version.FilePath,
			Content:       params.Version.Content,
			WordCount:     params.Version.WordCount,
			CreatedAt:     params.Version.CreatedAt,
		}); err != nil {
			return fmt.Errorf("failed to insert plan version: %w", err)
		}
		return nil
	})
}

func (r *Repository) InsertPlanVersion(ctx context.Context, params dto.InsertPlanVersionParams) error {
	return r.q.InsertPlanVersion(ctx, InsertPlanVersionParams{
		PlanID:        params.PlanID,
		VersionNumber: params.VersionNumber,
		FilePath:      params.FilePath,
		Content:       params.Content,
		WordCount:     params.WordCount,
		CreatedAt:     params.CreatedAt,
	})
}

func (r *Repository) GetPlanVersionHistory(ctx context.Context, params dto.VersionHistoryParams) ([]dto.PlanVersion, error) {
	rows, err := r.q.GetPlanVersionHistory(ctx, GetPlanVersionHistoryParams{
		PlanID: params.PlanID,
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanVersion, len(rows))
	for i, row := range rows {
		result[i] = planVersionToDomain(row)
	}
	return result, nil
}

func (r *Repository) GetPlanVersionByNumber(ctx context.Context, planID, versionNumber int64) (dto.PlanVersion, error) {
	pv, err := r.q.GetPlanVersionByNumber(ctx, GetPlanVersionByNumberParams{
		PlanID:        planID,
		VersionNumber: versionNumber,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return dto.PlanVersion{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.PlanVersion{}, err
	}
	return planVersionToDomain(pv), nil
}

func (r *Repository) GetLatestVersionNumber(ctx context.Context, planID int64) (int64, error) {
	result, err := r.q.GetLatestVersionNumber(ctx, planID)
	if err != nil {
		return 0, err
	}
	// SQLC returns interface{} for COALESCE, convert to int64
	switch v := result.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, nil
	}
}

func (r *Repository) GetVersionCount(ctx context.Context, planID int64) (int64, error) {
	return r.q.GetVersionCount(ctx, planID)
}

func (r *Repository) DeleteVersionsOlderThan(ctx context.Context, params dto.DeleteVersionsParams) error {
	return r.q.DeleteVersionsOlderThan(ctx, DeleteVersionsOlderThanParams{
		PlanID:        params.PlanID,
		VersionNumber: params.VersionNumber,
	})
}

func (r *Repository) SearchVersionsByContent(ctx context.Context, params dto.SearchVersionsParams) ([]dto.PlanVersion, error) {
	searchPattern := "%" + params.Query + "%"
	rows, err := r.q.SearchVersionsByContent(ctx, SearchVersionsByContentParams{
		PlanID:  params.PlanID,
		Content: searchPattern,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanVersion, len(rows))
	for i, row := range rows {
		result[i] = planVersionToDomain(row)
	}
	return result, nil
}

// Setting operations

func (r *Repository) GetSettingByName(ctx context.Context, name string) (dto.Setting, error) {
	s, err := r.q.GetSettingByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Setting{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Setting{}, err
	}
	return settingToDomain(s), nil
}

func (r *Repository) UpsertSetting(ctx context.Context, params dto.UpsertSettingParams) error {
	return r.q.UpsertSetting(ctx, UpsertSettingParams{
		VariableName:  params.VariableName,
		VariableType:  params.VariableType,
		StringValue:   ptrToNullString(params.StringValue),
		NumberValue:   ptrToNullFloat64(params.NumberValue),
		BooleanValue:  ptrToNullBool(params.BooleanValue),
		DatetimeValue: ptrToNullTime(params.DatetimeValue),
	})
}

func (r *Repository) DeleteSetting(ctx context.Context, name string) error {
	return r.q.DeleteSetting(ctx, name)
}

// Connector operations

func (r *Repository) GetConnectorByName(ctx context.Context, name string) (dto.Connector, error) {
	c, err := r.q.GetConnectorByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Connector{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Connector{}, err
	}
	return connectorToDomain(c), nil
}

func (r *Repository) ListConnectors(ctx context.Context) ([]dto.Connector, error) {
	rows, err := r.q.ListConnectors(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Connector, len(rows))
	for i, row := range rows {
		result[i] = connectorToDomain(row)
	}
	return result, nil
}

func (r *Repository) GetConnectorForRole(ctx context.Context, role dto.ConnectorRole) (string, bool, error) {
	c, err := r.q.GetConnectorByRole(ctx, string(role))
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return c.Name, true, nil
}

func (r *Repository) SetConnectorForRole(ctx context.Context, name string, role dto.ConnectorRole) error {
	if err := r.q.ClearConnectorRole(ctx, string(role)); err != nil {
		return err
	}
	return r.q.SetConnectorRole(ctx, SetConnectorRoleParams{
		Role: string(role),
		Name: name,
	})
}

func (r *Repository) ClearConnectorForRole(ctx context.Context, role dto.ConnectorRole) error {
	return r.q.ClearConnectorRole(ctx, string(role))
}

func (r *Repository) UpsertConnector(ctx context.Context, name, displayName string, enabled bool) error {
	return r.q.UpsertConnector(ctx, UpsertConnectorParams{
		Name:        name,
		DisplayName: displayName,
		Enabled:     enabled,
	})
}

func (r *Repository) DeleteConnector(ctx context.Context, name string) error {
	return r.q.DeleteConnector(ctx, name)
}

// Connector setting operations

func (r *Repository) GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error) {
	cs, err := r.q.GetConnectorSetting(ctx, GetConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return cs.SettingValue, true, nil
}

func (r *Repository) ListConnectorSettings(ctx context.Context, connectorName string) ([]dto.ConnectorSetting, error) {
	rows, err := r.q.ListConnectorSettings(ctx, connectorName)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ConnectorSetting, len(rows))
	for i, row := range rows {
		result[i] = connectorSettingToDomain(row)
	}
	return result, nil
}

func (r *Repository) UpsertConnectorSetting(ctx context.Context, connectorName, key, value string, isSecret bool) error {
	return r.q.UpsertConnectorSetting(ctx, UpsertConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
		SettingValue:  value,
		IsSecret:      isSecret,
	})
}

func (r *Repository) DeleteConnectorSetting(ctx context.Context, connectorName, key string) error {
	return r.q.DeleteConnectorSetting(ctx, DeleteConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
	})
}

func (r *Repository) DeleteAllConnectorSettings(ctx context.Context, connectorName string) error {
	return r.q.DeleteAllConnectorSettings(ctx, connectorName)
}

// Tag domain conversion helpers

func tagToDomain(t Tag) dto.Tag {
	return dto.Tag{
		ID:          t.ID,
		Name:        t.Name,
		Description: nullStringToPtr(t.Description),
		Color:       nullStringToPtr(t.Color),
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// Tag operations

func (r *Repository) InsertTag(ctx context.Context, params dto.InsertTagParams) (dto.Tag, error) {
	tag, err := r.q.InsertTag(ctx, InsertTagParams{
		Name:        params.Name,
		Description: ptrToNullString(params.Description),
		Color:       ptrToNullString(params.Color),
		CreatedAt:   params.CreatedAt,
		UpdatedAt:   params.ModifiedAt,
	})
	if err != nil {
		return dto.Tag{}, err
	}
	return tagToDomain(tag), nil
}

func (r *Repository) GetTagByName(ctx context.Context, name string) (dto.Tag, error) {
	tag, err := r.q.GetTagByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Tag{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Tag{}, err
	}
	return tagToDomain(tag), nil
}

func (r *Repository) GetTagByID(ctx context.Context, id int64) (dto.Tag, error) {
	tag, err := r.q.GetTagByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.Tag{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Tag{}, err
	}
	return tagToDomain(tag), nil
}

func (r *Repository) GetTagPlanCounts(ctx context.Context) (map[string]int, error) {
	rows, err := r.q.GetTagPlanCounts(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.Name] = int(row.PlanCount)
	}
	return counts, nil
}

func (r *Repository) GetUntaggedPlanCount(ctx context.Context) (int64, error) {
	return r.q.GetUntaggedPlanCount(ctx)
}

func (r *Repository) ListUntaggedPlans(ctx context.Context, sortCol, sortDir string, wpm int) ([]dto.PlanSummary, error) {
	sb := sqlbuilder.SQLite.NewSelectBuilder()
	rtExpr := fmt.Sprintf("MAX(1, (p.word_count + %d - 1) / %d) AS reading_time", wpm, wpm)
	sb.Select("p.id", "p.file_name", "p.sync_source", "p.title", "p.created_at", "p.modified_at", "p.file_size", "p.word_count", rtExpr, "COALESCE(cc.comment_count, 0) AS comment_count").
		From("plans p").
		JoinWithOption(sqlbuilder.LeftJoin,
			"(SELECT plan_id, COUNT(*) AS comment_count FROM plan_comments GROUP BY plan_id) cc",
			"cc.plan_id = p.id",
		).
		Where("p.id NOT IN (SELECT DISTINCT plan_id FROM plan_tags)")
	if sortCol == "reading_time" {
		if sortDir == "asc" {
			sb.OrderByAsc("reading_time")
		} else {
			sb.OrderByDesc("reading_time")
		}
	} else {
		applyOrder(sb, "p."+sortCol, sortDir)
	}
	q, args := sb.Build()
	return r.queryPlanSummaries(ctx, q, args...)
}

func (r *Repository) ListPlansWithTags(ctx context.Context) ([]dto.PlanSummary, error) {
	rows, err := r.q.ListAllPlansWithTags(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanSummary, len(rows))
	for i, row := range rows {
		result[i] = planSummaryFromListWithTagsRow(row)
	}
	return result, nil
}

func (r *Repository) ListAllTags(ctx context.Context) ([]dto.Tag, error) {
	rows, err := r.q.ListAllTags(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Tag, len(rows))
	for i, row := range rows {
		result[i] = tagToDomain(row)
	}
	return result, nil
}

func (r *Repository) UpdateTag(ctx context.Context, params dto.UpdateTagParams) error {
	return r.q.UpdateTag(ctx, UpdateTagParams{
		Name:        params.Name,
		Description: ptrToNullString(params.Description),
		Color:       ptrToNullString(params.Color),
		ID:          params.ID,
	})
}

func (r *Repository) DeleteTag(ctx context.Context, id int64) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := q.RemoveAllPlansFromTag(ctx, id); err != nil {
			return fmt.Errorf("failed to delete plan_tags associations: %w", err)
		}
		if err := q.DeleteTag(ctx, id); err != nil {
			return fmt.Errorf("failed to delete tag: %w", err)
		}
		return nil
	})
}

// Plan-Tag associations

func (r *Repository) AddTagToPlan(ctx context.Context, planID, tagID int64, assignedAt time.Time) error {
	return r.q.AddTagToPlan(ctx, AddTagToPlanParams{
		PlanID:     planID,
		TagID:      tagID,
		AssignedAt: assignedAt,
	})
}

func (r *Repository) RemoveTagFromPlan(ctx context.Context, planID, tagID int64) error {
	return r.q.RemoveTagFromPlan(ctx, RemoveTagFromPlanParams{
		PlanID: planID,
		TagID:  tagID,
	})
}

func (r *Repository) RemoveAllTagsFromPlan(ctx context.Context, planID int64) error {
	return r.q.RemoveAllTagsFromPlan(ctx, planID)
}

func (r *Repository) GetPlanTags(ctx context.Context, planID int64) ([]dto.Tag, error) {
	rows, err := r.q.GetPlanTags(ctx, planID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Tag, len(rows))
	for i, row := range rows {
		result[i] = tagToDomain(row)
	}
	return result, nil
}

func (r *Repository) SetPlanTags(ctx context.Context, planID int64, tagIDs []int64, assignedAt time.Time) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := q.RemoveAllTagsFromPlan(ctx, planID); err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			if err := q.AddTagToPlan(ctx, AddTagToPlanParams{
				PlanID:     planID,
				TagID:      tagID,
				AssignedAt: assignedAt,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func applyOrder(sb *sqlbuilder.SelectBuilder, col, dir string) {
	if dir == "asc" {
		sb.OrderByAsc(col)
	} else {
		sb.OrderByDesc(col)
	}
}

// Comment operations

func commentToDomain(c PlanComment) dto.Comment {
	return dto.Comment{
		ID:        c.ID,
		PlanID:    c.PlanID,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func (r *Repository) InsertComment(ctx context.Context, params dto.InsertCommentParams) (dto.Comment, error) {
	c, err := r.q.InsertComment(ctx, InsertCommentParams{
		PlanID:    params.PlanID,
		Content:   params.Content,
		CreatedAt: params.CreatedAt,
		UpdatedAt: params.UpdatedAt,
	})
	if err != nil {
		return dto.Comment{}, err
	}
	return commentToDomain(c), nil
}

func (r *Repository) GetPlanComments(ctx context.Context, planID int64) ([]dto.Comment, error) {
	rows, err := r.q.GetPlanComments(ctx, planID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Comment, len(rows))
	for i, row := range rows {
		result[i] = commentToDomain(row)
	}
	return result, nil
}

func (r *Repository) DeleteComment(ctx context.Context, id int64) error {
	return r.q.DeleteComment(ctx, id)
}

func (r *Repository) GetPlanCommentCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := r.q.GetPlanCommentCounts(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.PlanID] = int(row.CommentCount)
	}
	return counts, nil
}

func (r *Repository) queryPlanSummariesWithTags(ctx context.Context, q string, args ...interface{}) ([]dto.PlanSummary, error) {
	rows, err := r.q.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []dto.PlanSummary
	for rows.Next() {
		var (
			id           int64
			fileName     string
			syncSource   string
			title        string
			createdAt    time.Time
			modifiedAt   time.Time
			fileSize     int64
			wordCount    int64
			tagIDs       string
			tagNames     string
			commentCount int64
		)
		if err := rows.Scan(&id, &fileName, &syncSource, &title, &createdAt, &modifiedAt, &fileSize, &wordCount, &tagIDs, &tagNames, &commentCount); err != nil {
			return nil, err
		}
		result = append(result, dto.PlanSummary{
			ID:           id,
			FileName:     fileName,
			SyncSource:   syncSource,
			Title:        title,
			CreatedAt:    createdAt,
			ModifiedAt:   modifiedAt,
			FileSize:     fileSize,
			WordCount:    wordCount,
			Tags:         parseCommaSeparatedTags(tagIDs, tagNames),
			CommentCount: commentCount,
		})
	}
	return result, rows.Err()
}

func (r *Repository) queryPlanSummaries(ctx context.Context, q string, args ...interface{}) ([]dto.PlanSummary, error) {
	rows, err := r.q.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []dto.PlanSummary
	for rows.Next() {
		var (
			id           int64
			fileName     string
			syncSource   string
			title        string
			createdAt    time.Time
			modifiedAt   time.Time
			fileSize     int64
			wordCount    int64
			readingTime  int64
			commentCount int64
		)
		if err := rows.Scan(&id, &fileName, &syncSource, &title, &createdAt, &modifiedAt, &fileSize, &wordCount, &readingTime, &commentCount); err != nil {
			return nil, err
		}
		result = append(result, dto.PlanSummary{
			ID:           id,
			FileName:     fileName,
			SyncSource:   syncSource,
			Title:        title,
			CreatedAt:    createdAt,
			ModifiedAt:   modifiedAt,
			FileSize:     fileSize,
			WordCount:    wordCount,
			ReadingTime:  readingTime,
			CommentCount: commentCount,
		})
	}
	return result, rows.Err()
}
