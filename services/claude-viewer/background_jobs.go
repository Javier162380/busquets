package claudeviewer

import (
	"context"
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/google/uuid"
)

// CreateBackgroundJob creates a new background job for executing a plan.
func (s *Service) CreateBackgroundJob(ctx context.Context, planFileName, name, description, provider, agentConfig string) (dto.BackgroundJob, error) {
	// Validate plan exists
	plan, err := s.db.GetPlanByFileName(ctx, planFileName)
	if err != nil {
		return dto.BackgroundJob{}, fmt.Errorf("plan not found: %w", err)
	}

	// Validate provider exists
	if !s.agentRegistry.Has(provider) {
		return dto.BackgroundJob{}, fmt.Errorf("agent provider %q not found", provider)
	}

	// Validate agent config if provided
	if agentConfig != "" {
		agentProvider, err := s.agentRegistry.Get(provider)
		if err != nil {
			return dto.BackgroundJob{}, fmt.Errorf("failed to get provider: %w", err)
		}
		if err := agentProvider.ValidateConfig(agentConfig); err != nil {
			return dto.BackgroundJob{}, fmt.Errorf("invalid agent config: %w", err)
		}
	}

	// Create job
	now := time.Now()
	jobID := uuid.New().String()

	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	params := dto.CreateJobParams{
		ID:            jobID,
		PlanID:        plan.ID,
		Name:          name,
		Description:   descPtr,
		AgentProvider: provider,
		AgentConfig:   agentConfig,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return s.db.InsertJob(ctx, params)
}

// GetBackgroundJob retrieves a background job by ID.
func (s *Service) GetBackgroundJob(ctx context.Context, jobID string) (dto.BackgroundJob, error) {
	return s.db.GetJobByID(ctx, jobID)
}

// ListBackgroundJobs lists background jobs with optional filters.
func (s *Service) ListBackgroundJobs(ctx context.Context, planID *int64, status *dto.JobStatus, limit, offset int) ([]dto.JobWithPlan, error) {
	params := dto.ListJobsParams{
		PlanID: planID,
		Status: status,
		Limit:  int64(limit),
		Offset: int64(offset),
	}
	return s.db.ListJobsWithPlans(ctx, params)
}

// UpdateBackgroundJob updates a background job's configuration.
func (s *Service) UpdateBackgroundJob(ctx context.Context, jobID string, name, description, agentConfig *string) error {
	// Validate agent config if provided
	if agentConfig != nil && *agentConfig != "" {
		job, err := s.db.GetJobByID(ctx, jobID)
		if err != nil {
			return fmt.Errorf("failed to get job: %w", err)
		}

		agentProvider, err := s.agentRegistry.Get(job.AgentProvider)
		if err != nil {
			return fmt.Errorf("failed to get provider: %w", err)
		}
		if err := agentProvider.ValidateConfig(*agentConfig); err != nil {
			return fmt.Errorf("invalid agent config: %w", err)
		}
	}

	params := dto.UpdateJobParams{
		ID:          jobID,
		Name:        name,
		Description: description,
		AgentConfig: agentConfig,
		UpdatedAt:   time.Now(),
	}

	return s.db.UpdateJob(ctx, params)
}

// DeleteBackgroundJob deletes a background job and all its executions.
func (s *Service) DeleteBackgroundJob(ctx context.Context, jobID string) error {
	// Check if job exists
	_, err := s.db.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	// Delete executions first (cascade should handle this, but being explicit)
	if err := s.db.DeleteExecutionsByJobID(ctx, jobID); err != nil {
		return fmt.Errorf("failed to delete executions: %w", err)
	}

	// Delete scheduled job if exists
	_ = s.db.DeleteScheduledJob(ctx, jobID) // Ignore error if not found

	// Delete job
	return s.db.DeleteJob(ctx, jobID)
}

// TriggerJob submits a job for immediate execution.
func (s *Service) TriggerJob(ctx context.Context, jobID string) error {
	if !s.jobManager.IsRunning() {
		return fmt.Errorf("job manager not running")
	}
	return s.jobManager.SubmitJob(ctx, jobID, "manual")
}

// ScheduleJob schedules a job for future execution (one-time).
func (s *Service) ScheduleJob(ctx context.Context, jobID string, scheduledAt time.Time) error {
	if !s.jobManager.IsRunning() {
		return fmt.Errorf("job manager not running")
	}
	return s.jobManager.ScheduleJob(ctx, jobID, scheduledAt)
}

// CancelScheduledJob cancels a scheduled job before execution.
func (s *Service) CancelScheduledJob(ctx context.Context, jobID string) error {
	if !s.jobManager.IsRunning() {
		return fmt.Errorf("job manager not running")
	}
	return s.jobManager.CancelScheduledJob(ctx, jobID)
}

// CancelJobExecution cancels a running job execution.
func (s *Service) CancelJobExecution(ctx context.Context, executionID string) error {
	if !s.jobManager.IsRunning() {
		return fmt.Errorf("job manager not running")
	}
	return s.jobManager.CancelExecution(ctx, executionID)
}

// GetJobExecution retrieves a job execution by ID.
func (s *Service) GetJobExecution(ctx context.Context, executionID string) (dto.JobExecution, error) {
	return s.db.GetExecutionByID(ctx, executionID)
}

// ListJobExecutions lists executions for a job.
func (s *Service) ListJobExecutions(ctx context.Context, jobID *string, limit, offset int) ([]dto.ExecutionWithJob, error) {
	params := dto.ListExecutionsParams{
		JobID:  jobID,
		Limit:  int64(limit),
		Offset: int64(offset),
	}
	return s.db.ListExecutionsWithContext(ctx, params)
}

// GetScheduledJob retrieves the scheduled execution for a job.
func (s *Service) GetScheduledJob(ctx context.Context, jobID string) (dto.ScheduledJob, error) {
	return s.db.GetScheduledJobByJobID(ctx, jobID)
}

// GetActiveJobCount returns the number of currently running job executions.
func (s *Service) GetActiveJobCount() int {
	if !s.jobManager.IsRunning() {
		return 0
	}
	return s.jobManager.GetActiveExecutionCount()
}
