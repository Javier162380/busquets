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
	Query    string   // Will be used for both title and content LIKE search
	TagNames []string // Tag names to filter by
	MatchAll bool     // true = AND logic (all tags must match), false = OR logic (any tag can match)
}

// SearchPaginationParams contains parameters for paginated search.
type SearchPaginationParams struct {
	Query    string
	TagNames []string
	MatchAll bool
	Limit    int64
	Offset   int64
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

// InsertTagParams contains parameters for inserting a new tag.
type InsertTagParams struct {
	Name        string
	Description *string
	Color       *string
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

// UpdateTagParams contains parameters for updating an existing tag.
type UpdateTagParams struct {
	ID          int64
	Name        string
	Description *string
	Color       *string
}

// TagFilterParams contains parameters for filtering plans by tags.
type TagFilterParams struct {
	TagNames []string
	MatchAll bool // AND vs OR logic
}

// InsertSessionParams contains parameters for inserting a new session.
type InsertSessionParams struct {
	SessionUUID    string
	ProjectPath    string
	ProjectName    string
	JSONLFilePath  string
	PlanID         *int64
	Status         string
	MessageCount   int64
	FirstMessageAt *time.Time
	LastMessageAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CWD            *string
	GitBranch      *string
	Slug           *string
}

// UpdateSessionParams contains parameters for updating an existing session.
type UpdateSessionParams struct {
	ID             int64 // WHERE clause
	Status         string
	MessageCount   int64
	FirstMessageAt *time.Time
	LastMessageAt  *time.Time
	UpdatedAt      time.Time
	CWD            *string
	GitBranch      *string
}

// InsertSessionMessageParams contains parameters for inserting a session message.
type InsertSessionMessageParams struct {
	SessionID      int64
	MessageUUID    string
	ParentUUID     *string
	MessageType    string
	MessageSubtype *string
	Content        *string
	Role           *string
	Timestamp      time.Time
	CWD            *string
	GitBranch      *string
	IsMeta         bool
	IsSidechain    bool
	CreatedAt      time.Time
}

// InsertSessionFileChangeParams contains parameters for inserting a file change.
type InsertSessionFileChangeParams struct {
	SessionID  int64
	MessageID  *int64
	FilePath   string
	ChangeType string
	DetectedAt time.Time
}

// InsertSessionTodoParams contains parameters for inserting a session todo.
type InsertSessionTodoParams struct {
	SessionID  int64
	MessageID  *int64
	Content    string
	Status     string
	ActiveForm *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpdateSessionTodoParams contains parameters for updating a session todo.
type UpdateSessionTodoParams struct {
	ID         int64 // WHERE clause
	Status     string
	ActiveForm *string
	UpdatedAt  time.Time
}
