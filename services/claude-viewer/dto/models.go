// Package dto defines the data transfer objects for the Claude Plan Viewer.
// These types are used to transfer data between the service layer and repository.
package dto

import "time"

// Plan represents a plan document.
type Plan struct {
	ID         int64
	FileName   string
	FilePath   string
	Title      string
	Content    string
	CreatedAt  time.Time
	ModifiedAt time.Time
	IndexedAt  time.Time
	FileSize   int64
	WordCount  int64
	Tags       []Tag
}

// PlanSummary represents a plan summary for listing.
// Used by ListAllPlans, SearchPlans, and their paginated variants.
type PlanSummary struct {
	ID         int64
	FileName   string
	Title      string
	CreatedAt  time.Time
	ModifiedAt time.Time
	FileSize   int64
	WordCount  int64
	Tags       []Tag
}

// PlanVersion represents a versioned snapshot of a plan.
type PlanVersion struct {
	ID            int64
	PlanID        int64
	VersionNumber int64
	FilePath      string
	Content       string
	WordCount     int64
	CreatedAt     time.Time
}

// Connector represents an external connector (e.g., Telegram).
type Connector struct {
	Name        string
	DisplayName string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ConnectorSetting represents a configuration setting for a connector.
type ConnectorSetting struct {
	ID            int64
	ConnectorName string
	SettingKey    string
	SettingValue  string
	IsSecret      bool
}

// Setting represents a generic application setting.
type Setting struct {
	VariableName  string
	VariableType  string
	StringValue   *string
	NumberValue   *float64
	BooleanValue  *bool
	DatetimeValue *time.Time
}

// Tag represents a tag that can be associated with plans.
type Tag struct {
	ID          int64
	Name        string
	Description *string
	Color       *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PlanWithTags represents a plan with its associated tags.
type PlanWithTags struct {
	Plan Plan
	Tags []Tag
}

// PlanSummaryWithTags represents a plan summary with its associated tags.
type PlanSummaryWithTags struct {
	Summary PlanSummary
	Tags    []Tag
}

// Session represents a Claude Code session.
type Session struct {
	ID             int64
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

// SessionSummary represents a session summary for listing.
type SessionSummary struct {
	ID            int64
	SessionUUID   string
	ProjectName   string
	Status        string
	MessageCount  int64
	LastMessageAt *time.Time
	Slug          *string
	PlanTitle     *string // If associated with plan
}

// SessionMessage represents an individual message in a session.
type SessionMessage struct {
	ID             int64
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

// SessionFileChange represents a file modification tracked during a session.
type SessionFileChange struct {
	ID         int64
	SessionID  int64
	MessageID  *int64
	FilePath   string
	ChangeType string
	DetectedAt time.Time
}

// SessionTodo represents a todo item from a session.
type SessionTodo struct {
	ID         int64
	SessionID  int64
	MessageID  *int64
	Content    string
	Status     string
	ActiveForm *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SessionDetail represents a session with full details including messages.
type SessionDetail struct {
	Session
	Messages       []SessionMessage
	FileChanges    []SessionFileChange
	Todos          []SessionTodo
	AssociatedPlan *PlanSummary
}
