package claudeviewer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/agent"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/google/uuid"
)

// JobManager manages background job execution.
// It follows the WatchManager pattern for consistency.
type JobManager struct {
	service         *Service
	agentRegistry   *agent.Registry
	workerPool      *WorkerPool
	mu              sync.RWMutex
	running         bool
	cancel          context.CancelFunc
	resultChan      chan JobResult
	maxConcurrent   int
	schedulerTicker *time.Ticker
	pollingInterval time.Duration
}

// NewJobManager creates a new job manager.
func NewJobManager(service *Service, agentRegistry *agent.Registry, maxConcurrent int) *JobManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 3 // Default
	}

	return &JobManager{
		service:         service,
		agentRegistry:   agentRegistry,
		maxConcurrent:   maxConcurrent,
		resultChan:      make(chan JobResult, 20), // Buffered to prevent blocking
		pollingInterval: 30 * time.Second,         // Default polling interval
	}
}

// Start starts the job manager.
func (jm *JobManager) Start(ctx context.Context) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if jm.running {
		return nil // Already running
	}

	// Create worker pool
	jm.workerPool = NewWorkerPool(jm.agentRegistry, jm.maxConcurrent, 100)

	// Start worker pool
	poolCtx, cancel := context.WithCancel(ctx)
	jm.cancel = cancel

	if err := jm.workerPool.Start(poolCtx); err != nil {
		cancel()
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	jm.running = true

	// Start result forwarding goroutine
	go jm.forwardResults(poolCtx)

	// Start scheduler polling loop
	go jm.schedulerLoop(poolCtx)

	return nil
}

// Stop gracefully stops the job manager.
func (jm *JobManager) Stop(ctx context.Context) error {
	jm.mu.Lock()
	if !jm.running {
		jm.mu.Unlock()
		return nil
	}
	jm.running = false
	jm.mu.Unlock()

	// Cancel context to stop scheduler loop
	if jm.cancel != nil {
		jm.cancel()
	}

	// Stop scheduler ticker
	if jm.schedulerTicker != nil {
		jm.schedulerTicker.Stop()
	}

	// Stop worker pool with timeout
	if jm.workerPool != nil {
		return jm.workerPool.Stop(ctx)
	}

	return nil
}

// IsRunning returns true if the job manager is running.
func (jm *JobManager) IsRunning() bool {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.running
}

// ResultChannel returns the channel for receiving job results.
func (jm *JobManager) ResultChannel() <-chan JobResult {
	return jm.resultChan
}

// SubmitJob submits a job for immediate execution.
func (jm *JobManager) SubmitJob(ctx context.Context, jobID, triggeredBy string) error {
	jm.mu.RLock()
	if !jm.running {
		jm.mu.RUnlock()
		return fmt.Errorf("job manager not running")
	}
	jm.mu.RUnlock()

	// Get job from database
	job, err := jm.service.db.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Get plan to obtain file path
	plan, err := jm.service.db.GetPlanByID(ctx, job.PlanID)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Create execution record
	executionID := uuid.New().String()
	nextNum, err := jm.service.db.GetNextExecutionNumber(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get next execution number: %w", err)
	}

	now := time.Now()
	execution := dto.CreateExecutionParams{
		ID:              executionID,
		JobID:           jobID,
		ExecutionNumber: nextNum,
		Status:          dto.ExecutionStatusPending,
		TriggeredBy:     triggeredBy,
		StartedAt:       &now,
	}

	_, err = jm.service.db.InsertExecution(ctx, execution)
	if err != nil {
		return fmt.Errorf("failed to create execution record: %w", err)
	}

	// Update job status to running
	if err := jm.service.db.UpdateJobStatus(ctx, jobID, dto.JobStatusRunning, &now); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Submit task to worker pool
	task := JobTask{
		JobID:         jobID,
		ExecutionID:   executionID,
		PlanPath:      plan.FilePath,
		AgentProvider: job.AgentProvider,
		AgentConfig:   job.AgentConfig,
		TriggeredBy:   triggeredBy,
		WorkingDir:    jm.service.sourcePlansDir,
		Timeout:       30 * time.Minute, // Default timeout
	}

	return jm.workerPool.Submit(task)
}

// ScheduleJob schedules a job for future execution (one-time).
func (jm *JobManager) ScheduleJob(ctx context.Context, jobID string, scheduledAt time.Time) error {
	jm.mu.RLock()
	if !jm.running {
		jm.mu.RUnlock()
		return fmt.Errorf("job manager not running")
	}
	jm.mu.RUnlock()

	// Validate scheduled time is in the future
	if scheduledAt.Before(time.Now()) {
		return fmt.Errorf("scheduled time must be in the future")
	}

	// Check if job exists
	_, err := jm.service.db.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Check if job already has a schedule
	_, err = jm.service.db.GetScheduledJobByJobID(ctx, jobID)
	if err == nil {
		return fmt.Errorf("job already has a scheduled execution")
	}
	if err != dto.ErrNotFound {
		return fmt.Errorf("failed to check existing schedule: %w", err)
	}

	// Create scheduled job
	scheduleID := uuid.New().String()
	params := dto.CreateScheduledJobParams{
		ID:          scheduleID,
		JobID:       jobID,
		ScheduledAt: scheduledAt,
		CreatedAt:   time.Now(),
	}

	_, err = jm.service.db.InsertScheduledJob(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create scheduled job: %w", err)
	}

	return nil
}

// CancelScheduledJob cancels a scheduled job before execution.
func (jm *JobManager) CancelScheduledJob(ctx context.Context, jobID string) error {
	jm.mu.RLock()
	if !jm.running {
		jm.mu.RUnlock()
		return fmt.Errorf("job manager not running")
	}
	jm.mu.RUnlock()

	// Check if job has a schedule
	_, err := jm.service.db.GetScheduledJobByJobID(ctx, jobID)
	if err != nil {
		if err == dto.ErrNotFound {
			return fmt.Errorf("job has no scheduled execution")
		}
		return fmt.Errorf("failed to get scheduled job: %w", err)
	}

	// Cancel the scheduled job
	return jm.service.db.CancelScheduledJob(ctx, jobID)
}

// CancelExecution cancels a running execution.
func (jm *JobManager) CancelExecution(ctx context.Context, executionID string) error {
	jm.mu.RLock()
	if !jm.running || jm.workerPool == nil {
		jm.mu.RUnlock()
		return fmt.Errorf("job manager not running")
	}
	jm.mu.RUnlock()

	return jm.workerPool.CancelExecution(executionID)
}

// GetActiveExecutionCount returns the number of currently running executions.
func (jm *JobManager) GetActiveExecutionCount() int {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	if jm.workerPool == nil {
		return 0
	}
	return jm.workerPool.ActiveCount()
}

// schedulerLoop polls the database for due scheduled jobs.
func (jm *JobManager) schedulerLoop(ctx context.Context) {
	jm.mu.Lock()
	jm.schedulerTicker = time.NewTicker(jm.pollingInterval)
	jm.mu.Unlock()

	defer func() {
		jm.mu.Lock()
		if jm.schedulerTicker != nil {
			jm.schedulerTicker.Stop()
		}
		jm.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-jm.schedulerTicker.C:
			jm.processDueScheduledJobs(ctx)
		}
	}
}

// processDueScheduledJobs checks for and executes due scheduled jobs.
func (jm *JobManager) processDueScheduledJobs(ctx context.Context) {
	now := time.Now()

	// Get due scheduled jobs
	dueJobs, err := jm.service.db.ListDueScheduledJobs(ctx, now)
	if err != nil {
		// Log error but continue
		return
	}

	for _, scheduledJob := range dueJobs {
		// Skip cancelled jobs
		if scheduledJob.Cancelled {
			// Delete cancelled scheduled job
			_ = jm.service.db.DeleteScheduledJob(ctx, scheduledJob.JobID)
			continue
		}

		// Submit job for execution
		if err := jm.SubmitJob(ctx, scheduledJob.JobID, "scheduled"); err != nil {
			// Log error but continue with other jobs
			continue
		}

		// Delete scheduled job after submission
		_ = jm.service.db.DeleteScheduledJob(ctx, scheduledJob.JobID)
	}
}

// forwardResults forwards results from worker pool to job manager result channel.
func (jm *JobManager) forwardResults(ctx context.Context) {
	poolResultChan := jm.workerPool.ResultChan()

	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-poolResultChan:
			if !ok {
				// Pool result channel closed
				return
			}

			// Update execution record in database
			jm.updateExecutionResult(ctx, result)

			// Forward result to job manager channel
			select {
			case jm.resultChan <- result:
			case <-time.After(5 * time.Second):
				// Timeout - don't block
			}
		}
	}
}

// updateExecutionResult updates the execution record with the result.
func (jm *JobManager) updateExecutionResult(ctx context.Context, result JobResult) {
	// Update execution status
	status := dto.ExecutionStatusFailed
	if result.Success {
		status = dto.ExecutionStatusCompleted
	} else if result.Cancelled {
		status = dto.ExecutionStatusCancelled
	}

	params := dto.UpdateExecutionParams{
		ID:           result.ExecutionID,
		Status:       status,
		StartedAt:    &result.StartedAt,
		CompletedAt:  &result.CompletedAt,
		ExitCode:     &result.ExitCode,
		OutputLog:    &result.Output,
		ErrorMessage: &result.ErrorMessage,
	}

	if err := jm.service.db.UpdateExecution(ctx, params); err != nil {
		// Log error but don't fail
		return
	}

	// Update job status
	jobStatus := dto.JobStatusCompleted
	if !result.Success {
		jobStatus = dto.JobStatusFailed
	}

	now := time.Now()
	_ = jm.service.db.UpdateJobStatus(ctx, result.JobID, jobStatus, &now)
}
