// Package postgres represents the postgres repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements dto.Repository for PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
	q    *Queries
}

// NewRepository creates a new PostgreSQL repository.
func NewRepository(db DBTX) *Repository {
	// Type assert to *pgxpool.Pool for transaction support
	pool, ok := db.(*pgxpool.Pool)
	if !ok {
		// If not *pgxpool.Pool, create repository without transaction support
		return &Repository{q: New(db)}
	}
	return &Repository{
		pool: pool,
		q:    New(db),
	}
}

// Verify interface compliance at compile time.
var _ dto.Repository = (*Repository)(nil)

// Type conversion helpers: pgtype.* -> Go types

func timestamptzToTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

func timeToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func textToPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func ptrToText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func float8ToPtr(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

func ptrToFloat8(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *f, Valid: true}
}

func pgBoolToPtr(b pgtype.Bool) *bool {
	if !b.Valid {
		return nil
	}
	return &b.Bool
}

func ptrToPgBool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func timestamptzToPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	return &ts.Time
}

func ptrToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// Model conversions: SQLC types -> domain types

func planToDomain(p Plan) dto.Plan {
	return dto.Plan{
		ID:         int64(p.ID),
		FileName:   p.FileName,
		FilePath:   p.FilePath,
		Title:      p.Title,
		Content:    p.Content,
		CreatedAt:  timestamptzToTime(p.CreatedAt),
		ModifiedAt: timestamptzToTime(p.ModifiedAt),
		IndexedAt:  timestamptzToTime(p.IndexedAt),
		FileSize:   p.FileSize,
		WordCount:  p.WordCount,
	}
}

func planSummaryFromListRow(r ListAllPlansWithTagsRow) dto.PlanSummary {
	planSummaryDto := dto.PlanSummary{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}

	tags := make([]dto.Tag, len(r.TagIds))
	for i, tagID := range r.TagIds {
		tags[i] = dto.Tag{
			ID: int64(tagID),
		}
		if i < len(r.TagNames)-1 {
			tags[i].Name = r.TagNames[i]
		}
	}
	planSummaryDto.Tags = tags
	return planSummaryDto
}

func planSummaryFromPaginationRow(r ListAllPlansWithPaginationRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func planSummaryFromSearchPaginationRow(r SearchPlansWithPaginationRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func planSummaryFromSearchWithTagsRow(r SearchPlansWithTagsRow) dto.PlanSummary {
	planSummaryDto := dto.PlanSummary{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}

	tags := make([]dto.Tag, len(r.TagIds))
	for i, tagID := range r.TagIds {
		tags[i] = dto.Tag{
			ID: int64(tagID),
		}
		if i < len(r.TagNames) {
			tags[i].Name = r.TagNames[i]
		}
	}
	planSummaryDto.Tags = tags
	return planSummaryDto
}

func planVersionToDomain(pv PlanVersion) dto.PlanVersion {
	return dto.PlanVersion{
		ID:            int64(pv.ID),
		PlanID:        pv.PlanID,
		VersionNumber: pv.VersionNumber,
		FilePath:      pv.FilePath,
		Content:       pv.Content,
		WordCount:     pv.WordCount,
		CreatedAt:     timestamptzToTime(pv.CreatedAt),
	}
}

func connectorToDomain(c Connector) dto.Connector {
	return dto.Connector{
		Name:        c.Name,
		DisplayName: c.DisplayName,
		Enabled:     c.Enabled,
		CreatedAt:   timestamptzToTime(c.CreatedAt),
		UpdatedAt:   timestamptzToTime(c.UpdatedAt),
	}
}

func connectorSettingToDomain(cs ConnectorSetting) dto.ConnectorSetting {
	return dto.ConnectorSetting{
		ID:            int64(cs.ID),
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
		StringValue:   textToPtr(s.StringValue),
		NumberValue:   float8ToPtr(s.NumberValue),
		BooleanValue:  pgBoolToPtr(s.BooleanValue),
		DatetimeValue: timestamptzToPtr(s.DatetimeValue),
	}
}

// Plan operations

func (r *Repository) CountPlans(ctx context.Context) (int64, error) {
	return r.q.CountPlans(ctx)
}

func (r *Repository) GetPlanByFileName(ctx context.Context, fileName string) (dto.Plan, error) {
	p, err := r.q.GetPlanByFileName(ctx, fileName)
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.Plan{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Plan{}, err
	}
	return planToDomain(p), nil
}

func (r *Repository) InsertPlan(ctx context.Context, params dto.InsertPlanParams) error {
	return r.q.InsertPlan(ctx, InsertPlanParams{
		FileName:   params.FileName,
		FilePath:   params.FilePath,
		Title:      params.Title,
		Content:    params.Content,
		CreatedAt:  timeToTimestamptz(params.CreatedAt),
		ModifiedAt: timeToTimestamptz(params.ModifiedAt),
		IndexedAt:  timeToTimestamptz(params.IndexedAt),
		FileSize:   params.FileSize,
		WordCount:  params.WordCount,
	})
}

func (r *Repository) UpdatePlan(ctx context.Context, params dto.UpdatePlanParams) error {
	return r.q.UpdatePlan(ctx, UpdatePlanParams{
		FileName:   params.FileName,
		Title:      params.Title,
		Content:    params.Content,
		ModifiedAt: timeToTimestamptz(params.ModifiedAt),
		IndexedAt:  timeToTimestamptz(params.IndexedAt),
		FileSize:   params.FileSize,
		WordCount:  params.WordCount,
	})
}

func (r *Repository) DeletePlan(ctx context.Context, fileName string) error {
	if r.pool == nil {
		return errors.New("transaction support not available")
	}

	// Begin transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	// Create queries with transaction
	qtx := r.q.WithTx(tx)

	// First, get the plan to obtain its ID
	plan, err := qtx.GetPlanByFileName(ctx, fileName)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Delete all plan_tags associations for this plan
	if err := qtx.RemoveAllTagsFromPlan(ctx, int64(plan.ID)); err != nil {
		return fmt.Errorf("failed to delete plan_tags associations: %w", err)
	}

	// Then delete the plan itself
	if err := qtx.DeletePlan(ctx, fileName); err != nil {
		return fmt.Errorf("failed to delete plan: %w", err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		rollBarErr := tx.Rollback(ctx)
		if rollBarErr != nil {
			return fmt.Errorf("failed to rollback: %w orginial error %w", rollBarErr, err)
		}
	}
	return err
}

func (r *Repository) ListAllPlans(ctx context.Context) ([]dto.PlanSummary, error) {
	rows, err := r.q.ListAllPlansWithTags(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanSummary, len(rows))
	for i, row := range rows {
		result[i] = planSummaryFromListRow(row)
	}
	return result, nil
}

func (r *Repository) ListAllPlansWithPagination(ctx context.Context, params dto.PaginationParams) ([]dto.PlanSummary, error) {
	rows, err := r.q.ListAllPlansWithPagination(ctx, ListAllPlansWithPaginationParams{
		Limit:  int32(params.Limit),
		Offset: int32(params.Offset),
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
	searchPattern := "%" + params.Query + "%"
	rows, err := r.q.SearchPlansWithTags(ctx, SearchPlansWithTagsParams{
		Title:   searchPattern,
		Content: searchPattern,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanSummary, len(rows))
	for i, row := range rows {
		result[i] = planSummaryFromSearchWithTagsRow(row)
	}
	return result, nil
}

func (r *Repository) SearchPlansWithPagination(ctx context.Context, params dto.SearchPaginationParams) ([]dto.PlanSummary, error) {
	searchPattern := "%" + params.Query + "%"
	rows, err := r.q.SearchPlansWithPagination(ctx, SearchPlansWithPaginationParams{
		Title:   searchPattern,
		Content: searchPattern,
		Limit:   int32(params.Limit),
		Offset:  int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanSummary, len(rows))
	for i, row := range rows {
		result[i] = planSummaryFromSearchPaginationRow(row)
	}
	return result, nil
}

func (r *Repository) SearchPlansWithTags(ctx context.Context, params dto.SearchParams) ([]dto.PlanSummary, error) {
	// If no tags, just do regular search
	if len(params.TagNames) == 0 {
		return r.SearchPlans(ctx, params)
	}

	// Strategy: Get all plans (filtered by text search if query provided), then filter by tags
	// This uses SQLC queries and avoids dynamic IN clauses

	// Step 1: Get all plans (filtered by text search if query provided)
	var allPlans []dto.PlanSummary
	var err error

	if params.Query == "" {
		// No text search, get all plans
		allPlans, err = r.ListAllPlans(ctx)
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

func (r *Repository) InsertPlanVersion(ctx context.Context, params dto.InsertPlanVersionParams) error {
	return r.q.InsertPlanVersion(ctx, InsertPlanVersionParams{
		PlanID:        params.PlanID,
		VersionNumber: params.VersionNumber,
		FilePath:      params.FilePath,
		Content:       params.Content,
		WordCount:     params.WordCount,
		CreatedAt:     timeToTimestamptz(params.CreatedAt),
	})
}

func (r *Repository) GetPlanVersionHistory(ctx context.Context, params dto.VersionHistoryParams) ([]dto.PlanVersion, error) {
	rows, err := r.q.GetPlanVersionHistory(ctx, GetPlanVersionHistoryParams{
		PlanID: params.PlanID,
		Limit:  int32(params.Limit),
		Offset: int32(params.Offset),
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
	if errors.Is(err, pgx.ErrNoRows) {
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
	// Handle the COALESCE result
	if result == nil {
		return 0, nil
	}
	return result.(int64), nil
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
	if errors.Is(err, pgx.ErrNoRows) {
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
		StringValue:   ptrToText(params.StringValue),
		NumberValue:   ptrToFloat8(params.NumberValue),
		BooleanValue:  ptrToPgBool(params.BooleanValue),
		DatetimeValue: ptrToTimestamptz(params.DatetimeValue),
	})
}

func (r *Repository) DeleteSetting(ctx context.Context, name string) error {
	return r.q.DeleteSetting(ctx, name)
}

// Connector operations

func (r *Repository) GetConnectorByName(ctx context.Context, name string) (dto.Connector, error) {
	c, err := r.q.GetConnectorByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
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

func (r *Repository) GetEnabledConnector(ctx context.Context) (dto.Connector, error) {
	c, err := r.q.GetEnabledConnector(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.Connector{}, dto.ErrNoConnectorEnabled
	}
	if err != nil {
		return dto.Connector{}, err
	}
	return connectorToDomain(c), nil
}

func (r *Repository) UpsertConnector(ctx context.Context, params dto.UpsertConnectorParams) error {
	return r.q.UpsertConnector(ctx, UpsertConnectorParams{
		Name:        params.Name,
		DisplayName: params.DisplayName,
		Enabled:     params.Enabled,
	})
}

func (r *Repository) SetConnectorEnabled(ctx context.Context, name string) error {
	return r.q.SetConnectorEnabled(ctx, name)
}

func (r *Repository) DisableAllConnectors(ctx context.Context) error {
	return r.q.DisableAllConnectors(ctx)
}

func (r *Repository) DeleteConnector(ctx context.Context, name string) error {
	return r.q.DeleteConnector(ctx, name)
}

// Connector setting operations

func (r *Repository) GetConnectorSetting(ctx context.Context, connectorName, key string) (dto.ConnectorSetting, error) {
	cs, err := r.q.GetConnectorSetting(ctx, GetConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.ConnectorSetting{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.ConnectorSetting{}, err
	}
	return connectorSettingToDomain(cs), nil
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

func (r *Repository) UpsertConnectorSetting(ctx context.Context, params dto.UpsertConnectorSettingParams) error {
	return r.q.UpsertConnectorSetting(ctx, UpsertConnectorSettingParams{
		ConnectorName: params.ConnectorName,
		SettingKey:    params.SettingKey,
		SettingValue:  params.SettingValue,
		IsSecret:      params.IsSecret,
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
		ID:          int64(t.ID),
		Name:        t.Name,
		Description: textToPtr(t.Description),
		Color:       textToPtr(t.Color),
		CreatedAt:   timestamptzToTime(t.CreatedAt),
		UpdatedAt:   timestamptzToTime(t.UpdatedAt),
	}
}

// Tag operations

func (r *Repository) InsertTag(ctx context.Context, params dto.InsertTagParams) (dto.Tag, error) {
	tag, err := r.q.InsertTag(ctx, InsertTagParams{
		Name:        params.Name,
		Description: ptrToText(params.Description),
		Color:       ptrToText(params.Color),
		CreatedAt:   pgtype.Timestamptz{Time: params.CreatedAt, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: params.ModifiedAt, Valid: true},
	})
	if err != nil {
		return dto.Tag{}, err
	}
	return tagToDomain(tag), nil
}

func (r *Repository) GetTagByName(ctx context.Context, name string) (dto.Tag, error) {
	tag, err := r.q.GetTagByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.Tag{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Tag{}, err
	}
	return tagToDomain(tag), nil
}

func (r *Repository) GetTagByID(ctx context.Context, id int64) (dto.Tag, error) {
	tag, err := r.q.GetTagByID(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.Tag{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Tag{}, err
	}
	return tagToDomain(tag), nil
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
		Description: ptrToText(params.Description),
		Color:       ptrToText(params.Color),
		ID:          int32(params.ID),
	})
}

func (r *Repository) DeleteTag(ctx context.Context, id int64) error {
	if r.pool == nil {
		return errors.New("transaction support not available")
	}

	// Begin transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	// Create queries with transaction
	qtx := r.q.WithTx(tx)

	// First, delete all plan_tags associations for this tag
	if err := qtx.RemoveAllPlansFromTag(ctx, id); err != nil {
		return fmt.Errorf("failed to delete plan_tags associations: %w", err)
	}

	// Then delete the tag itself
	if err := qtx.DeleteTag(ctx, int32(id)); err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		rollBarErr := tx.Rollback(ctx)
		if rollBarErr != nil {
			return fmt.Errorf("failed to rollback: %w orginial error %w", rollBarErr, err)
		}
	}
	return err
}

// Plan-Tag associations

func (r *Repository) AddTagToPlan(ctx context.Context, planID, tagID int64, assignedAt time.Time) error {
	return r.q.AddTagToPlan(ctx, AddTagToPlanParams{
		PlanID:     planID,
		TagID:      tagID,
		AssignedAt: pgtype.Timestamptz{Time: assignedAt, Valid: true},
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
	if r.pool == nil {
		return errors.New("transaction support not available")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	qtx := r.q.WithTx(tx)

	if err := qtx.RemoveAllTagsFromPlan(ctx, planID); err != nil {
		return err
	}

	for _, tagID := range tagIDs {
		if err := qtx.AddTagToPlan(ctx, AddTagToPlanParams{
			PlanID:     planID,
			TagID:      tagID,
			AssignedAt: pgtype.Timestamptz{Time: assignedAt, Valid: true},
		}); err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		rollBarErr := tx.Rollback(ctx)
		if rollBarErr != nil {
			return fmt.Errorf("failed to rollback: %w orginial error %w", rollBarErr, err)
		}
	}

	return err
}
