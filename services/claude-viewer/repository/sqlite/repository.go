// Package sqlite represents the sqlite repository.
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// Repository implements dto.Repository for SQLite.
type Repository struct {
	q *Queries
}

// NewRepository creates a new SQLite repository.
func NewRepository(db DBTX) *Repository {
	return &Repository{q: New(db)}
}

// Verify interface compliance at compile time.
var _ dto.Repository = (*Repository)(nil)

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

// Model conversions: SQLC types -> domain types

func planToDomain(p Plan) dto.Plan {
	return dto.Plan{
		ID:         p.ID,
		FileName:   p.FileName,
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

func planSummaryFromListRow(r ListAllPlansRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         r.ID,
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  r.CreatedAt,
		ModifiedAt: r.ModifiedAt,
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func planSummaryFromPaginationRow(r ListAllPlansWithPaginationRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         r.ID,
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  r.CreatedAt,
		ModifiedAt: r.ModifiedAt,
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func planSummaryFromSearchRow(r SearchPlansRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         r.ID,
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  r.CreatedAt,
		ModifiedAt: r.ModifiedAt,
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func planSummaryFromSearchPaginationRow(r SearchPlansWithPaginationRow) dto.PlanSummary {
	return dto.PlanSummary{
		ID:         r.ID,
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  r.CreatedAt,
		ModifiedAt: r.ModifiedAt,
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
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

func (r *Repository) GetPlanByFileName(ctx context.Context, fileName string) (dto.Plan, error) {
	p, err := r.q.GetPlanByFileName(ctx, fileName)
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
		CreatedAt:  params.CreatedAt,
		ModifiedAt: params.ModifiedAt,
		IndexedAt:  params.IndexedAt,
		FileSize:   params.FileSize,
		WordCount:  params.WordCount,
	})
}

func (r *Repository) UpdatePlan(ctx context.Context, params dto.UpdatePlanParams) error {
	return r.q.UpdatePlan(ctx, UpdatePlanParams{
		FileName:   params.FileName,
		Title:      params.Title,
		Content:    params.Content,
		ModifiedAt: params.ModifiedAt,
		IndexedAt:  params.IndexedAt,
		FileSize:   params.FileSize,
		WordCount:  params.WordCount,
	})
}

func (r *Repository) DeletePlan(ctx context.Context, fileName string) error {
	return r.q.DeletePlan(ctx, fileName)
}

func (r *Repository) ListAllPlans(ctx context.Context) ([]dto.PlanSummary, error) {
	rows, err := r.q.ListAllPlans(ctx)
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
	searchPattern := "%" + params.Query + "%"
	rows, err := r.q.SearchPlans(ctx, SearchPlansParams{
		Title:   searchPattern,
		Content: searchPattern,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanSummary, len(rows))
	for i, row := range rows {
		result[i] = planSummaryFromSearchRow(row)
	}
	return result, nil
}

func (r *Repository) SearchPlansWithPagination(ctx context.Context, params dto.SearchPaginationParams) ([]dto.PlanSummary, error) {
	searchPattern := "%" + params.Query + "%"
	rows, err := r.q.SearchPlansWithPagination(ctx, SearchPlansWithPaginationParams{
		Title:   searchPattern,
		Content: searchPattern,
		Limit:   params.Limit,
		Offset:  params.Offset,
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

// Plan version operations

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
