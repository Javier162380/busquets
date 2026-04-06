package dto

import "time"

// InsertPlanParams contains parameters for inserting a new plan.
type InsertPlanParams struct {
	FileName   string
	FilePath   string
	Title      string
	Content    string
	CreatedAt  time.Time
	ModifiedAt time.Time
	IndexedAt  time.Time
	FileSize   int64
	WordCount  int64
}

// UpdatePlanParams contains parameters for updating an existing plan.
type UpdatePlanParams struct {
	FileName   string // WHERE clause
	Title      string
	Content    string
	ModifiedAt time.Time
	IndexedAt  time.Time
	FileSize   int64
	WordCount  int64
}

// InsertPlanVersionParams contains parameters for inserting a plan version.
type InsertPlanVersionParams struct {
	PlanID        int64
	VersionNumber int64
	FilePath      string
	Content       string
	WordCount     int64
	CreatedAt     time.Time
}

// PaginationParams contains limit/offset for paginated queries.
type PaginationParams struct {
	Limit  int64
	Offset int64
}

// VersionHistoryParams contains parameters for fetching version history.
type VersionHistoryParams struct {
	PlanID int64
	Limit  int64
	Offset int64
}

// DeleteVersionsParams contains parameters for deleting old versions.
type DeleteVersionsParams struct {
	PlanID        int64
	VersionNumber int64 // Delete versions <= this number
}

// SearchParams contains parameters for searching plans.
type SearchParams struct {
	Query string // Will be used for both title and content LIKE search
}

// SearchPaginationParams contains parameters for paginated search.
type SearchPaginationParams struct {
	Query  string
	Limit  int64
	Offset int64
}

// SearchVersionsParams contains parameters for searching versions by content.
type SearchVersionsParams struct {
	PlanID int64
	Query  string
}

// UpsertSettingParams contains parameters for upserting a setting.
type UpsertSettingParams struct {
	VariableName  string
	VariableType  string
	StringValue   *string
	NumberValue   *float64
	BooleanValue  *bool
	DatetimeValue *time.Time
}

// UpsertConnectorParams contains parameters for upserting a connector.
type UpsertConnectorParams struct {
	Name        string
	DisplayName string
	Enabled     bool
}

// UpsertConnectorSettingParams contains parameters for upserting a connector setting.
type UpsertConnectorSettingParams struct {
	ConnectorName string
	SettingKey    string
	SettingValue  string
	IsSecret      bool
}
