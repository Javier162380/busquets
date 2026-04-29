package dto

import "time"

// JobStatus represents the lifecycle state of a background job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// ExecutionStatus represents the state of a job execution.
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
	AgentConfig   string // JSON configuration specific to the provider
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

// ScheduledJob represents a one-time scheduled execution of a job.
type ScheduledJob struct {
	ID          string
	JobID       string
	ScheduledAt time.Time
	Cancelled   bool
	CreatedAt   time.Time
}

// JobWithPlan combines a background job with its associated plan information.
type JobWithPlan struct {
	Job      BackgroundJob
	PlanName string
	PlanPath string
}

// ExecutionWithJob combines an execution with its job and plan context.
type ExecutionWithJob struct {
	Execution JobExecution
	JobName   string
	PlanName  string
}

// CreateJobParams contains parameters for creating a background job.
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

// UpdateJobParams contains parameters for updating a background job.
type UpdateJobParams struct {
	ID          string
	Name        *string
	Description *string
	AgentConfig *string
	Status      *JobStatus
	UpdatedAt   time.Time
}

// CreateExecutionParams contains parameters for creating a job execution.
type CreateExecutionParams struct {
	ID              string
	JobID           string
	ExecutionNumber int64
	Status          ExecutionStatus
	TriggeredBy     string
	StartedAt       *time.Time
}

// UpdateExecutionParams contains parameters for updating a job execution.
type UpdateExecutionParams struct {
	ID           string
	Status       ExecutionStatus
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ExitCode     *int
	OutputLog    *string
	ErrorMessage *string
}

// CreateScheduledJobParams contains parameters for creating a scheduled job.
type CreateScheduledJobParams struct {
	ID          string
	JobID       string
	ScheduledAt time.Time
	CreatedAt   time.Time
}

// ListJobsParams contains parameters for listing background jobs.
type ListJobsParams struct {
	PlanID *int64
	Status *JobStatus
	Limit  int64
	Offset int64
}

// ListExecutionsParams contains parameters for listing job executions.
type ListExecutionsParams struct {
	JobID  *string
	Limit  int64
	Offset int64
}
