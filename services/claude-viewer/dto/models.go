// Package dto defines the data transfer objects for the Claude Plan Viewer.
// These types are used to transfer data between the service layer and repository.
package dto

import "time"

// Plan represents a plan document.
type Plan struct {
	ID         int64
	FileName   string
	SyncSource string
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
	ID          int64
	FileName    string
	SyncSource  string
	Title       string
	CreatedAt   time.Time
	ModifiedAt  time.Time
	FileSize    int64
	WordCount   int64
	ReadingTime int64
	Tags        []Tag
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
