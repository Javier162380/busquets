package claudeviewer

import (
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// Type aliases for TUI integration - these expose DTO types at the service layer

// BackgroundJob is an alias for dto.BackgroundJob.
type BackgroundJob = dto.BackgroundJob

// JobWithPlan is an alias for dto.JobWithPlan.
type JobWithPlan = dto.JobWithPlan

// ExecutionWithJob is an alias for dto.ExecutionWithJob.
type ExecutionWithJob = dto.ExecutionWithJob

// ScheduledJob is an alias for dto.ScheduledJob.
type ScheduledJob = dto.ScheduledJob

// JobStatus is an alias for dto.JobStatus.
type JobStatus = dto.JobStatus

// JobResult represents the result of a job execution from the worker pool.
// This type is exported for use by the TUI and other consumers.
type JobResult struct {
	JobID        string
	ExecutionID  string
	Success      bool
	ExitCode     int
	Output       string
	ErrorMessage string
	StartedAt    time.Time
	CompletedAt  time.Time
	Cancelled    bool
}
