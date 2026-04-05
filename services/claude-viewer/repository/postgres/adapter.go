// Package postgres represents the postgres layout.
package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"

	"github.com/jackc/pgx/v5/pgtype"
)

// Adapter wraps postgres.Queries to implement repository.Querier.
// This allows the service to use Postgres without knowing about pgtype conversions.
type Adapter struct {
	q *Queries
}

// NewAdapter creates a new Postgres adapter.
func NewAdapter(q *Queries) *Adapter {
	return &Adapter{q: q}
}

// Compile-time check that Adapter implements repository.Querier.
var _ repository.Querier = (*Adapter)(nil)

// --- Type conversion helpers ---

func timeToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timestamptzToTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

func nullStringToText(ns sql.NullString) pgtype.Text {
	return pgtype.Text{String: ns.String, Valid: ns.Valid}
}

func textToNullString(t pgtype.Text) sql.NullString {
	return sql.NullString{String: t.String, Valid: t.Valid}
}

func nullFloat64ToFloat8(nf sql.NullFloat64) pgtype.Float8 {
	return pgtype.Float8{Float64: nf.Float64, Valid: nf.Valid}
}

func float8ToNullFloat64(f pgtype.Float8) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f.Float64, Valid: f.Valid}
}

func nullBoolToPgBool(nb sql.NullBool) pgtype.Bool {
	return pgtype.Bool{Bool: nb.Bool, Valid: nb.Valid}
}

func pgBoolToNullBool(b pgtype.Bool) sql.NullBool {
	return sql.NullBool{Bool: b.Bool, Valid: b.Valid}
}

func nullTimeToTimestamptz(nt sql.NullTime) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: nt.Time, Valid: nt.Valid}
}

func timestamptzToNullTime(ts pgtype.Timestamptz) sql.NullTime {
	return sql.NullTime{Time: ts.Time, Valid: ts.Valid}
}

// --- Model conversion helpers ---

func planToRepo(p Plan) repository.Plan {
	return repository.Plan{
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

func planVersionToRepo(pv PlanVersion) repository.PlanVersion {
	return repository.PlanVersion{
		ID:            int64(pv.ID),
		PlanID:        pv.PlanID,
		VersionNumber: pv.VersionNumber,
		FilePath:      pv.FilePath,
		Content:       pv.Content,
		WordCount:     pv.WordCount,
		CreatedAt:     timestamptzToTime(pv.CreatedAt),
	}
}

func settingToRepo(s Setting) repository.Setting {
	return repository.Setting{
		VariableName:  s.VariableName,
		VariableType:  s.VariableType,
		StringValue:   textToNullString(s.StringValue),
		NumberValue:   float8ToNullFloat64(s.NumberValue),
		BooleanValue:  pgBoolToNullBool(s.BooleanValue),
		DatetimeValue: timestamptzToNullTime(s.DatetimeValue),
	}
}

func connectorToRepo(c Connector) repository.Connector {
	return repository.Connector{
		Name:        c.Name,
		DisplayName: c.DisplayName,
		Enabled:     c.Enabled,
		CreatedAt:   timestamptzToTime(c.CreatedAt),
		UpdatedAt:   timestamptzToTime(c.UpdatedAt),
	}
}

func connectorSettingToRepo(cs ConnectorSetting) repository.ConnectorSetting {
	return repository.ConnectorSetting{
		ID:            int64(cs.ID),
		ConnectorName: cs.ConnectorName,
		SettingKey:    cs.SettingKey,
		SettingValue:  cs.SettingValue,
		IsSecret:      cs.IsSecret,
	}
}

func listAllPlansRowToRepo(r ListAllPlansRow) repository.ListAllPlansRow {
	return repository.ListAllPlansRow{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func listAllPlansWithPaginationRowToRepo(r ListAllPlansWithPaginationRow) repository.ListAllPlansWithPaginationRow {
	return repository.ListAllPlansWithPaginationRow{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func searchPlansRowToRepo(r SearchPlansRow) repository.SearchPlansRow {
	return repository.SearchPlansRow{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

func searchPlansWithPaginationRowToRepo(r SearchPlansWithPaginationRow) repository.SearchPlansWithPaginationRow {
	return repository.SearchPlansWithPaginationRow{
		ID:         int64(r.ID),
		FileName:   r.FileName,
		Title:      r.Title,
		CreatedAt:  timestamptzToTime(r.CreatedAt),
		ModifiedAt: timestamptzToTime(r.ModifiedAt),
		FileSize:   r.FileSize,
		WordCount:  r.WordCount,
	}
}

// --- Plan operations ---

func (a *Adapter) CountPlans(ctx context.Context) (int64, error) {
	return a.q.CountPlans(ctx)
}

func (a *Adapter) DeletePlan(ctx context.Context, fileName string) error {
	return a.q.DeletePlan(ctx, fileName)
}

func (a *Adapter) GetPlanByFileName(ctx context.Context, fileName string) (repository.Plan, error) {
	p, err := a.q.GetPlanByFileName(ctx, fileName)
	if err != nil {
		return repository.Plan{}, err
	}
	return planToRepo(p), nil
}

func (a *Adapter) InsertPlan(ctx context.Context, arg repository.InsertPlanParams) error {
	return a.q.InsertPlan(ctx, InsertPlanParams{
		FileName:   arg.FileName,
		FilePath:   arg.FilePath,
		Title:      arg.Title,
		Content:    arg.Content,
		CreatedAt:  timeToTimestamptz(arg.CreatedAt),
		ModifiedAt: timeToTimestamptz(arg.ModifiedAt),
		IndexedAt:  timeToTimestamptz(arg.IndexedAt),
		FileSize:   arg.FileSize,
		WordCount:  arg.WordCount,
	})
}

func (a *Adapter) UpdatePlan(ctx context.Context, arg repository.UpdatePlanParams) error {
	return a.q.UpdatePlan(ctx, UpdatePlanParams{
		Title:      arg.Title,
		Content:    arg.Content,
		ModifiedAt: timeToTimestamptz(arg.ModifiedAt),
		IndexedAt:  timeToTimestamptz(arg.IndexedAt),
		FileSize:   arg.FileSize,
		WordCount:  arg.WordCount,
		FileName:   arg.FileName,
	})
}

func (a *Adapter) ListAllPlans(ctx context.Context) ([]repository.ListAllPlansRow, error) {
	rows, err := a.q.ListAllPlans(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]repository.ListAllPlansRow, len(rows))
	for i, r := range rows {
		result[i] = listAllPlansRowToRepo(r)
	}
	return result, nil
}

func (a *Adapter) ListAllPlansWithPagination(ctx context.Context, arg repository.ListAllPlansWithPaginationParams) ([]repository.ListAllPlansWithPaginationRow, error) {
	rows, err := a.q.ListAllPlansWithPagination(ctx, ListAllPlansWithPaginationParams{
		Limit:  int32(arg.Limit),
		Offset: int32(arg.Offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]repository.ListAllPlansWithPaginationRow, len(rows))
	for i, r := range rows {
		result[i] = listAllPlansWithPaginationRowToRepo(r)
	}
	return result, nil
}

func (a *Adapter) SearchPlans(ctx context.Context, arg repository.SearchPlansParams) ([]repository.SearchPlansRow, error) {
	rows, err := a.q.SearchPlans(ctx, SearchPlansParams{
		Title:   arg.Title,
		Content: arg.Content,
	})
	if err != nil {
		return nil, err
	}
	result := make([]repository.SearchPlansRow, len(rows))
	for i, r := range rows {
		result[i] = searchPlansRowToRepo(r)
	}
	return result, nil
}

func (a *Adapter) SearchPlansWithPagination(ctx context.Context, arg repository.SearchPlansWithPaginationParams) ([]repository.SearchPlansWithPaginationRow, error) {
	rows, err := a.q.SearchPlansWithPagination(ctx, SearchPlansWithPaginationParams{
		Title:   arg.Title,
		Content: arg.Content,
		Limit:   int32(arg.Limit),
		Offset:  int32(arg.Offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]repository.SearchPlansWithPaginationRow, len(rows))
	for i, r := range rows {
		result[i] = searchPlansWithPaginationRowToRepo(r)
	}
	return result, nil
}

// --- Plan version operations ---

func (a *Adapter) InsertPlanVersion(ctx context.Context, arg repository.InsertPlanVersionParams) error {
	return a.q.InsertPlanVersion(ctx, InsertPlanVersionParams{
		PlanID:        arg.PlanID,
		VersionNumber: arg.VersionNumber,
		FilePath:      arg.FilePath,
		Content:       arg.Content,
		WordCount:     arg.WordCount,
		CreatedAt:     timeToTimestamptz(arg.CreatedAt),
	})
}

func (a *Adapter) GetPlanVersionHistory(ctx context.Context, arg repository.GetPlanVersionHistoryParams) ([]repository.PlanVersion, error) {
	rows, err := a.q.GetPlanVersionHistory(ctx, GetPlanVersionHistoryParams{
		PlanID: arg.PlanID,
		Limit:  int32(arg.Limit),
		Offset: int32(arg.Offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]repository.PlanVersion, len(rows))
	for i, r := range rows {
		result[i] = planVersionToRepo(r)
	}
	return result, nil
}

func (a *Adapter) GetPlanVersionByNumber(ctx context.Context, arg repository.GetPlanVersionByNumberParams) (repository.PlanVersion, error) {
	pv, err := a.q.GetPlanVersionByNumber(ctx, GetPlanVersionByNumberParams{
		PlanID:        arg.PlanID,
		VersionNumber: arg.VersionNumber,
	})
	if err != nil {
		return repository.PlanVersion{}, err
	}
	return planVersionToRepo(pv), nil
}

func (a *Adapter) GetLatestVersionNumber(ctx context.Context, planID int64) (interface{}, error) {
	return a.q.GetLatestVersionNumber(ctx, planID)
}

func (a *Adapter) GetVersionCount(ctx context.Context, planID int64) (int64, error) {
	return a.q.GetVersionCount(ctx, planID)
}

func (a *Adapter) DeleteVersionsOlderThan(ctx context.Context, arg repository.DeleteVersionsOlderThanParams) error {
	return a.q.DeleteVersionsOlderThan(ctx, DeleteVersionsOlderThanParams{
		PlanID:        arg.PlanID,
		VersionNumber: arg.VersionNumber,
	})
}

func (a *Adapter) SearchVersionsByContent(ctx context.Context, arg repository.SearchVersionsByContentParams) ([]repository.PlanVersion, error) {
	rows, err := a.q.SearchVersionsByContent(ctx, SearchVersionsByContentParams{
		PlanID:  arg.PlanID,
		Content: arg.Content,
	})
	if err != nil {
		return nil, err
	}
	result := make([]repository.PlanVersion, len(rows))
	for i, r := range rows {
		result[i] = planVersionToRepo(r)
	}
	return result, nil
}

// --- Setting operations ---

func (a *Adapter) GetSettingByName(ctx context.Context, variableName string) (repository.Setting, error) {
	s, err := a.q.GetSettingByName(ctx, variableName)
	if err != nil {
		return repository.Setting{}, err
	}
	return settingToRepo(s), nil
}

func (a *Adapter) UpsertSetting(ctx context.Context, arg repository.UpsertSettingParams) error {
	return a.q.UpsertSetting(ctx, UpsertSettingParams{
		VariableName:  arg.VariableName,
		VariableType:  arg.VariableType,
		StringValue:   nullStringToText(arg.StringValue),
		NumberValue:   nullFloat64ToFloat8(arg.NumberValue),
		BooleanValue:  nullBoolToPgBool(arg.BooleanValue),
		DatetimeValue: nullTimeToTimestamptz(arg.DatetimeValue),
	})
}

func (a *Adapter) DeleteSetting(ctx context.Context, variableName string) error {
	return a.q.DeleteSetting(ctx, variableName)
}

// --- Connector operations ---

func (a *Adapter) GetConnectorByName(ctx context.Context, name string) (repository.Connector, error) {
	c, err := a.q.GetConnectorByName(ctx, name)
	if err != nil {
		return repository.Connector{}, err
	}
	return connectorToRepo(c), nil
}

func (a *Adapter) ListConnectors(ctx context.Context) ([]repository.Connector, error) {
	rows, err := a.q.ListConnectors(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]repository.Connector, len(rows))
	for i, r := range rows {
		result[i] = connectorToRepo(r)
	}
	return result, nil
}

func (a *Adapter) GetEnabledConnector(ctx context.Context) (repository.Connector, error) {
	c, err := a.q.GetEnabledConnector(ctx)
	if err != nil {
		return repository.Connector{}, err
	}
	return connectorToRepo(c), nil
}

func (a *Adapter) UpsertConnector(ctx context.Context, arg repository.UpsertConnectorParams) error {
	return a.q.UpsertConnector(ctx, UpsertConnectorParams{
		Name:        arg.Name,
		DisplayName: arg.DisplayName,
		Enabled:     arg.Enabled,
	})
}

func (a *Adapter) SetConnectorEnabled(ctx context.Context, name string) error {
	return a.q.SetConnectorEnabled(ctx, name)
}

func (a *Adapter) DisableAllConnectors(ctx context.Context) error {
	return a.q.DisableAllConnectors(ctx)
}

func (a *Adapter) DeleteConnector(ctx context.Context, name string) error {
	return a.q.DeleteConnector(ctx, name)
}

// --- Connector setting operations ---

func (a *Adapter) GetConnectorSetting(ctx context.Context, arg repository.GetConnectorSettingParams) (repository.ConnectorSetting, error) {
	cs, err := a.q.GetConnectorSetting(ctx, GetConnectorSettingParams{
		ConnectorName: arg.ConnectorName,
		SettingKey:    arg.SettingKey,
	})
	if err != nil {
		return repository.ConnectorSetting{}, err
	}
	return connectorSettingToRepo(cs), nil
}

func (a *Adapter) ListConnectorSettings(ctx context.Context, connectorName string) ([]repository.ConnectorSetting, error) {
	rows, err := a.q.ListConnectorSettings(ctx, connectorName)
	if err != nil {
		return nil, err
	}
	result := make([]repository.ConnectorSetting, len(rows))
	for i, r := range rows {
		result[i] = connectorSettingToRepo(r)
	}
	return result, nil
}

func (a *Adapter) UpsertConnectorSetting(ctx context.Context, arg repository.UpsertConnectorSettingParams) error {
	return a.q.UpsertConnectorSetting(ctx, UpsertConnectorSettingParams{
		ConnectorName: arg.ConnectorName,
		SettingKey:    arg.SettingKey,
		SettingValue:  arg.SettingValue,
		IsSecret:      arg.IsSecret,
	})
}

func (a *Adapter) DeleteConnectorSetting(ctx context.Context, arg repository.DeleteConnectorSettingParams) error {
	return a.q.DeleteConnectorSetting(ctx, DeleteConnectorSettingParams{
		ConnectorName: arg.ConnectorName,
		SettingKey:    arg.SettingKey,
	})
}

func (a *Adapter) DeleteAllConnectorSettings(ctx context.Context, connectorName string) error {
	return a.q.DeleteAllConnectorSettings(ctx, connectorName)
}
