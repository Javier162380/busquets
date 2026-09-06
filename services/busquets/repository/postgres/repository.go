// Package postgres represents the postgres repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Javier162380/busquets/services/busquets/dto"

	"github.com/huandu/go-sqlbuilder"
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

func (r *Repository) withTx(ctx context.Context, fn func(*Queries) error) error {
	if r.pool == nil {
		return errors.New("transaction support not available")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(r.q.WithTx(tx)); err != nil {
		_ = tx.Rollback(context.Background())
		return err
	}
	return tx.Commit(ctx)
}

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
		SyncSource: p.SyncSource,
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
		SyncSource: r.SyncSource,
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

func planSummaryFromPaginationRow(r ListAllPlansWithPaginationRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		SyncSource: r.SyncSource,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
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

func (r *Repository) GetPlanByFileName(ctx context.Context, fileName, syncSource string) (dto.Plan, error) {
	p, err := r.q.GetPlanByFileNameAndSource(ctx, GetPlanByFileNameAndSourceParams{
		FileName:   fileName,
		SyncSource: syncSource,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.Plan{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Plan{}, err
	}
	return planToDomain(p), nil
}

func (r *Repository) GetPlanByID(ctx context.Context, id int64) (dto.Plan, error) {
	p, err := r.q.GetPlanByID(ctx, int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.Plan{}, dto.ErrNotFound
	}
	if err != nil {
		return dto.Plan{}, err
	}
	return planToDomain(p), nil
}

func (r *Repository) InsertPlan(
	ctx context.Context,
	params dto.InsertPlanParams,
	writeFile func(id int64) (string, error),
	version func(planID int64) (dto.InsertPlanVersionParams, func() error),
) (int64, error) {
	var id int64
	err := r.withTx(ctx, func(q *Queries) error {
		insertedID, err := q.InsertPlan(ctx, InsertPlanParams{
			FileName:   params.FileName,
			SyncSource: params.SyncSource,
			FilePath:   params.FilePath,
			Title:      params.Title,
			Content:    params.Content,
			CreatedAt:  timeToTimestamptz(params.CreatedAt),
			ModifiedAt: timeToTimestamptz(params.ModifiedAt),
			IndexedAt:  timeToTimestamptz(params.IndexedAt),
			FileSize:   params.FileSize,
			WordCount:  params.WordCount,
		})
		if err != nil {
			return fmt.Errorf("failed to insert plan: %w", err)
		}

		filePath, err := writeFile(int64(insertedID))
		if err != nil {
			return err
		}

		if err := q.UpdatePlanFilePath(ctx, UpdatePlanFilePathParams{FilePath: filePath, ID: insertedID}); err != nil {
			return fmt.Errorf("failed to set plan file path: %w", err)
		}

		if version != nil {
			versionParams, writeVersionFile := version(int64(insertedID))
			if err := q.InsertPlanVersion(ctx, InsertPlanVersionParams{
				PlanID:        versionParams.PlanID,
				VersionNumber: versionParams.VersionNumber,
				FilePath:      versionParams.FilePath,
				Content:       versionParams.Content,
				WordCount:     versionParams.WordCount,
				CreatedAt:     timeToTimestamptz(versionParams.CreatedAt),
			}); err != nil {
				return fmt.Errorf("failed to save initial version to database: %w", err)
			}
			if err := writeVersionFile(); err != nil {
				return fmt.Errorf("failed to write initial version file: %w", err)
			}
		}

		id = int64(insertedID)
		return nil
	})
	return id, err
}

func (r *Repository) UpdatePlanFilePath(ctx context.Context, id int64, filePath string, writeFile func() error) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := writeFile(); err != nil {
			return err
		}
		return q.UpdatePlanFilePath(ctx, UpdatePlanFilePathParams{FilePath: filePath, ID: int32(id)})
	})
}

func (r *Repository) UpdatePlanVersionFilePath(ctx context.Context, id int64, filePath string, writeFile func() error) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := writeFile(); err != nil {
			return err
		}
		return q.UpdatePlanVersionFilePath(ctx, UpdatePlanVersionFilePathParams{FilePath: filePath, ID: int32(id)})
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
			ID:         int64(row.ID),
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

func (r *Repository) UpdatePlan(ctx context.Context, params dto.UpdatePlanParams) (dto.Plan, error) {
	p, err := r.q.UpdatePlan(ctx, UpdatePlanParams{
		FileName:   params.FileName,
		SyncSource: params.SyncSource,
		Title:      params.Title,
		Content:    params.Content,
		ModifiedAt: timeToTimestamptz(params.ModifiedAt),
		IndexedAt:  timeToTimestamptz(params.IndexedAt),
		FileSize:   params.FileSize,
		WordCount:  params.WordCount,
	})
	if err != nil {
		return dto.Plan{}, err
	}
	return planToDomain(p), nil
}

func (r *Repository) UpdatePlanContent(
	ctx context.Context,
	params dto.UpdatePlanContentParams,
	writeContent func() (time.Time, int64, error),
	writeVersionFile func(planID, versionNumber int64) (string, int64, error),
) (dto.Plan, error) {
	var plan dto.Plan
	err := r.withTx(ctx, func(q *Queries) error {
		modifiedAt, fileSize, err := writeContent()
		if err != nil {
			return err
		}

		p, err := q.UpdatePlan(ctx, UpdatePlanParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
			Title:      params.Plan.Title,
			Content:    params.Plan.Content,
			ModifiedAt: timeToTimestamptz(modifiedAt),
			IndexedAt:  timeToTimestamptz(params.Plan.IndexedAt),
			FileSize:   fileSize,
			WordCount:  params.Plan.WordCount,
		})
		if err != nil {
			return fmt.Errorf("failed to update plan: %w", err)
		}
		plan = planToDomain(p)

		tagRows, err := q.GetPlanTags(ctx, plan.ID)
		if err != nil {
			return fmt.Errorf("failed to get plan tags: %w", err)
		}
		plan.Tags = make([]dto.Tag, len(tagRows))
		for i, row := range tagRows {
			plan.Tags[i] = tagToDomain(row)
		}

		if writeVersionFile == nil {
			return nil
		}

		lastVersionNum, err := q.GetLatestVersionNumber(ctx, plan.ID)
		if err != nil {
			return fmt.Errorf("failed to get latest version number: %w", err)
		}
		nextVersionNum := coalesceToInt64(lastVersionNum) + 1

		versionFilePath, versionWordCount, err := writeVersionFile(plan.ID, nextVersionNum)
		if err != nil {
			return fmt.Errorf("failed to write version file: %w", err)
		}

		err = q.InsertPlanVersion(ctx, InsertPlanVersionParams{
			PlanID:        plan.ID,
			VersionNumber: nextVersionNum,
			FilePath:      versionFilePath,
			Content:       params.VersionContent,
			WordCount:     versionWordCount,
			CreatedAt:     timeToTimestamptz(params.VersionCreatedAt),
		})
		if err != nil {
			return fmt.Errorf("failed to save version to database: %w", err)
		}

		return nil
	})
	if err != nil {
		return dto.Plan{}, err
	}
	return plan, nil
}

func (r *Repository) InsertPlanWithTags(ctx context.Context, params dto.InsertPlanWithTagsParams) error {
	return r.withTx(ctx, func(q *Queries) error {
		if _, err := q.InsertPlan(ctx, InsertPlanParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
			FilePath:   params.Plan.FilePath,
			Title:      params.Plan.Title,
			Content:    params.Plan.Content,
			CreatedAt:  timeToTimestamptz(params.Plan.CreatedAt),
			ModifiedAt: timeToTimestamptz(params.Plan.ModifiedAt),
			IndexedAt:  timeToTimestamptz(params.Plan.IndexedAt),
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
				PlanID:     int64(plan.ID),
				TagID:      tagID,
				AssignedAt: timeToTimestamptz(params.AssignedAt),
			}); err != nil {
				return fmt.Errorf("failed to add tag to plan: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) UpdatePlanWithTags(ctx context.Context, params dto.UpdatePlanWithTagsParams) error {
	return r.withTx(ctx, func(q *Queries) error {
		if _, err := q.UpdatePlan(ctx, UpdatePlanParams{
			FileName:   params.Plan.FileName,
			SyncSource: params.Plan.SyncSource,
			Title:      params.Plan.Title,
			Content:    params.Plan.Content,
			ModifiedAt: timeToTimestamptz(params.Plan.ModifiedAt),
			IndexedAt:  timeToTimestamptz(params.Plan.IndexedAt),
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

		if err := q.RemoveAllTagsFromPlan(ctx, int64(plan.ID)); err != nil {
			return fmt.Errorf("failed to clear plan tags: %w", err)
		}

		for _, tagID := range params.TagIDs {
			if err := q.AddTagToPlan(ctx, AddTagToPlanParams{
				PlanID:     int64(plan.ID),
				TagID:      tagID,
				AssignedAt: timeToTimestamptz(params.AssignedAt),
			}); err != nil {
				return fmt.Errorf("failed to add tag to plan: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) DeletePlan(ctx context.Context, fileName, syncSource string, deleteFiles func() error) error {
	return r.withTx(ctx, func(q *Queries) error {
		plan, err := q.GetPlanByFileNameAndSource(ctx, GetPlanByFileNameAndSourceParams{
			FileName:   fileName,
			SyncSource: syncSource,
		})
		if err != nil {
			return fmt.Errorf("failed to get plan: %w", err)
		}

		if err := q.RemoveAllTagsFromPlan(ctx, int64(plan.ID)); err != nil {
			return fmt.Errorf("failed to delete plan_tags associations: %w", err)
		}

		if err := q.DeletePlanVersions(ctx, int64(plan.ID)); err != nil {
			return fmt.Errorf("failed to delete plan_versions: %w", err)
		}

		if err := q.DeletePlanComments(ctx, int64(plan.ID)); err != nil {
			return fmt.Errorf("failed to delete plan_comments: %w", err)
		}

		if err := q.DeletePlan(ctx, DeletePlanParams{FileName: fileName, SyncSource: syncSource}); err != nil {
			return fmt.Errorf("failed to delete plan: %w", err)
		}

		// Dispose of the files while the transaction is still open. Returning an error
		// here rolls the DB deletes above back with it, so the delete is atomic across
		// DB and filesystem — no separate compensating write.
		if deleteFiles != nil {
			if err := deleteFiles(); err != nil {
				return err
			}
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
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	rtExpr := fmt.Sprintf("GREATEST(1, CEIL(p.word_count::numeric / %d)::integer) AS reading_time", wpm)
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
	return r.searchPlansDynamic(ctx, "%"+params.Query+"%", params.SearchOver)
}

func (r *Repository) SearchPlansWithPagination(ctx context.Context, params dto.SearchPaginationParams) ([]dto.PlanSummary, error) {
	return r.searchPlansPaginationDynamic(ctx, "%"+params.Query+"%", params)
}

func (r *Repository) searchPlansDynamic(ctx context.Context, pattern string, searchOver dto.SearchField) ([]dto.PlanSummary, error) {
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select(
		"p.id", "p.file_name", "p.sync_source", "p.title",
		"p.created_at", "p.modified_at", "p.file_size", "p.word_count",
		"COALESCE(t.tag_ids, ARRAY[]::int4[]) AS tag_ids",
		"COALESCE(t.tag_names, ARRAY[]::text[]) AS tag_names",
		"COALESCE(cc.comment_count, 0) AS comment_count",
	).From("plans p").
		JoinWithOption(sqlbuilder.LeftJoin,
			`(SELECT pt.plan_id,
			         array_agg(DISTINCT t.id ORDER BY t.id)::int4[]   AS tag_ids,
			         array_agg(DISTINCT t.name ORDER BY t.name)::text[] AS tag_names
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
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select(
		"p.id", "p.file_name", "p.sync_source", "p.title",
		"p.created_at", "p.modified_at", "p.file_size", "p.word_count",
		"COALESCE(t.tag_ids, ARRAY[]::int4[]) AS tag_ids",
		"COALESCE(t.tag_names, ARRAY[]::text[]) AS tag_names",
		"COALESCE(cc.comment_count, 0) AS comment_count",
	).From("plans p").
		JoinWithOption(sqlbuilder.LeftJoin,
			`(SELECT pt.plan_id,
			         array_agg(DISTINCT t.id ORDER BY t.id)::int4[]   AS tag_ids,
			         array_agg(DISTINCT t.name ORDER BY t.name)::text[] AS tag_names
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

	// Strategy: Get all plans (filtered by text search if query provided), then filter by tags
	// This uses SQLC queries and avoids dynamic IN clauses

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

func (r *Repository) InsertPlanVersion(ctx context.Context, params dto.InsertPlanVersionParams, writeFile func() error) error {
	return r.withTx(ctx, func(queries *Queries) error {
		err := queries.InsertPlanVersion(ctx, InsertPlanVersionParams{
			PlanID:        params.PlanID,
			VersionNumber: params.VersionNumber,
			FilePath:      params.FilePath,
			Content:       params.Content,
			WordCount:     params.WordCount,
			CreatedAt:     timeToTimestamptz(params.CreatedAt),
		})
		if err != nil {
			return err
		}
		if writeFile == nil {
			return nil
		}

		return writeFile()
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
	return coalesceToInt64(result), nil
}

// coalesceToInt64 converts the interface{} SQLC generates for a COALESCE(...)
// result into an int64. Shared by GetLatestVersionNumber and
// UpdatePlanContent so the nil-check lives in one place.
func coalesceToInt64(v any) int64 {
	if v == nil {
		return 0
	}
	return v.(int64)
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

func (r *Repository) ListSettings(ctx context.Context, names []string) ([]dto.Setting, error) {
	rows, err := r.q.ListSettingsByName(ctx, names)
	if err != nil {
		return nil, err
	}
	settings := make([]dto.Setting, len(rows))
	for i, row := range rows {
		settings[i] = settingToDomain(row)
	}
	return settings, nil
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

func (r *Repository) GetConnectorForRole(ctx context.Context, role dto.ConnectorRole) (string, bool, error) {
	c, err := r.q.GetConnectorByRole(ctx, pgtype.Text{String: string(role), Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return c.Name, true, nil
}

func (r *Repository) SetConnectorForRole(ctx context.Context, name string, role dto.ConnectorRole) error {
	roleVal := pgtype.Text{String: string(role), Valid: true}
	if err := r.q.ClearConnectorRole(ctx, roleVal); err != nil {
		return err
	}
	return r.q.SetConnectorRole(ctx, SetConnectorRoleParams{
		Role: roleVal,
		Name: name,
	})
}

func (r *Repository) ClearConnectorForRole(ctx context.Context, role dto.ConnectorRole) error {
	return r.q.ClearConnectorRole(ctx, pgtype.Text{String: string(role), Valid: true})
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
	if errors.Is(err, pgx.ErrNoRows) {
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
	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	rtExpr := fmt.Sprintf("GREATEST(1, CEIL(p.word_count::numeric / %d)::integer) AS reading_time", wpm)
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
		result[i] = planSummaryFromListRow(row)
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
		Description: ptrToText(params.Description),
		Color:       ptrToText(params.Color),
		ID:          int32(params.ID),
	})
}

func (r *Repository) DeleteTag(ctx context.Context, id int64) error {
	return r.withTx(ctx, func(q *Queries) error {
		if err := q.RemoveAllPlansFromTag(ctx, id); err != nil {
			return fmt.Errorf("failed to delete plan_tags associations: %w", err)
		}
		if err := q.DeleteTag(ctx, int32(id)); err != nil {
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
	return r.withTx(ctx, func(q *Queries) error {
		if err := q.RemoveAllTagsFromPlan(ctx, planID); err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			if err := q.AddTagToPlan(ctx, AddTagToPlanParams{
				PlanID:     planID,
				TagID:      tagID,
				AssignedAt: pgtype.Timestamptz{Time: assignedAt, Valid: true},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetPlanTagsAndGet atomically replaces a plan's tags and returns the resulting list.
func (r *Repository) SetPlanTagsAndGet(ctx context.Context, planID int64, tagIDs []int64, assignedAt time.Time) ([]dto.Tag, error) {
	var result []dto.Tag
	err := r.withTx(ctx, func(q *Queries) error {
		if err := q.RemoveAllTagsFromPlan(ctx, planID); err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			if err := q.AddTagToPlan(ctx, AddTagToPlanParams{
				PlanID:     planID,
				TagID:      tagID,
				AssignedAt: pgtype.Timestamptz{Time: assignedAt, Valid: true},
			}); err != nil {
				return err
			}
		}
		rows, err := q.GetPlanTags(ctx, planID)
		if err != nil {
			return err
		}
		result = make([]dto.Tag, len(rows))
		for i, row := range rows {
			result[i] = tagToDomain(row)
		}
		return nil
	})
	return result, err
}

// RemoveTagFromPlanAndGet atomically removes one tag from one plan and returns the resulting list.
func (r *Repository) RemoveTagFromPlanAndGet(ctx context.Context, planID, tagID int64) ([]dto.Tag, error) {
	var result []dto.Tag
	err := r.withTx(ctx, func(q *Queries) error {
		if err := q.RemoveTagFromPlan(ctx, RemoveTagFromPlanParams{PlanID: planID, TagID: tagID}); err != nil {
			return err
		}
		rows, err := q.GetPlanTags(ctx, planID)
		if err != nil {
			return err
		}
		result = make([]dto.Tag, len(rows))
		for i, row := range rows {
			result[i] = tagToDomain(row)
		}
		return nil
	})
	return result, err
}

func applyOrder(sb *sqlbuilder.SelectBuilder, col, dir string) {
	if dir == "asc" {
		sb.OrderByAsc(col)
	} else {
		sb.OrderByDesc(col)
	}
}

func (r *Repository) queryPlanSummariesWithTags(ctx context.Context, q string, args ...interface{}) ([]dto.PlanSummary, error) {
	rows, err := r.q.db.Query(ctx, q, args...)
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
			tagIDs       []int32
			tagNames     []string
			commentCount int64
		)
		if err := rows.Scan(&id, &fileName, &syncSource, &title, &createdAt, &modifiedAt, &fileSize, &wordCount, &tagIDs, &tagNames, &commentCount); err != nil {
			return nil, err
		}
		tags := make([]dto.Tag, len(tagIDs))
		for i, tagID := range tagIDs {
			tags[i] = dto.Tag{ID: int64(tagID)}
			if i < len(tagNames) {
				tags[i].Name = tagNames[i]
			}
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
			Tags:         tags,
			CommentCount: commentCount,
		})
	}
	return result, rows.Err()
}

func (r *Repository) queryPlanSummaries(ctx context.Context, q string, args ...interface{}) ([]dto.PlanSummary, error) {
	rows, err := r.q.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []dto.PlanSummary
	for rows.Next() {
		var p dto.PlanSummary
		if err := rows.Scan(&p.ID, &p.FileName, &p.SyncSource, &p.Title, &p.CreatedAt, &p.ModifiedAt, &p.FileSize, &p.WordCount, &p.ReadingTime, &p.CommentCount); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// Comment operations

func commentToDomain(c PlanComment) dto.Comment {
	return dto.Comment{
		ID:        int64(c.ID),
		PlanID:    c.PlanID,
		Content:   c.Content,
		CreatedAt: timestamptzToTime(c.CreatedAt),
		UpdatedAt: timestamptzToTime(c.UpdatedAt),
	}
}

func (r *Repository) InsertComment(ctx context.Context, params dto.InsertCommentParams) (dto.Comment, error) {
	c, err := r.q.InsertComment(ctx, InsertCommentParams{
		PlanID:    params.PlanID,
		Content:   params.Content,
		CreatedAt: timeToTimestamptz(params.CreatedAt),
		UpdatedAt: timeToTimestamptz(params.UpdatedAt),
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
	return r.q.DeleteComment(ctx, int32(id))
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
