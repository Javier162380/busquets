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

// JobStatus represents the status of a background job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusPaused    JobStatus = "paused"
)

// ExecutionStatus represents the status of a job execution.
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// BackgroundJob represents a background job definition.
type BackgroundJob struct {
	ID            string
	PlanID        int64
	Name          string
	Description   *string
	AgentProvider string
	AgentConfig   string // JSON configuration
	Status        JobStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastRunAt     *time.Time
}

// JobExecution represents a single execution of a background job.
type JobExecution struct {
	ID              string
	JobID           string
	ExecutionNumber int64
	Status          ExecutionStatus
	StartedAt       *time.Time
	CompletedAt     *time.Time
	ExitCode        *int
	OutputLog       *string
	ErrorMessage    *string
	TriggeredBy     string // manual, scheduled, api
}

// ScheduledJob represents a one-time scheduled execution.
type ScheduledJob struct {
	ID          string
	JobID       string
	ScheduledAt time.Time
	Cancelled   bool
	CreatedAt   time.Time
}

// JobWithPlan represents a job with its associated plan info.
type JobWithPlan struct {
	Job      BackgroundJob
	PlanName string
	PlanPath string
}

// ExecutionWithJob represents an execution with its job context.
type ExecutionWithJob struct {
	Execution JobExecution
	JobName   string
	PlanName  string
}

// CreateJobParams represents parameters for creating a new background job.
type CreateJobParams struct {
	ID            string
	PlanID        int64
	Name          string
	Description   *string
	AgentProvider string
	AgentConfig   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpdateJobParams represents parameters for updating a background job.
type UpdateJobParams struct {
	ID          string
	Name        *string
	Description *string
	AgentConfig *string
	UpdatedAt   time.Time
}

// ListJobsParams represents parameters for listing jobs.
type ListJobsParams struct {
	PlanID *int64
	Status *JobStatus
	Limit  int64
	Offset int64
}

// CreateExecutionParams represents parameters for creating a job execution.
type CreateExecutionParams struct {
	ID              string
	JobID           string
	ExecutionNumber int64
	Status          ExecutionStatus
	TriggeredBy     string
	CreatedAt       time.Time
}

// UpdateExecutionParams represents parameters for updating a job execution.
type UpdateExecutionParams struct {
	ID           string
	Status       ExecutionStatus
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ExitCode     *int
	OutputLog    *string
	ErrorMessage *string
}

// ListExecutionsParams represents parameters for listing executions.
type ListExecutionsParams struct {
	JobID  *string
	Limit  int64
	Offset int64
}

// CreateScheduledJobParams represents parameters for creating a scheduled job.
type CreateScheduledJobParams struct {
	ID          string
	JobID       string
	ScheduledAt time.Time
	CreatedAt   time.Time
}
