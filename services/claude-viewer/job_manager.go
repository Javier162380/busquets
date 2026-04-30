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

// JobManager manages background job execution, scheduling, and worker pool.
type JobManager struct {
	service         *Service
	agentRegistry   *agent.Registry
	workerPool      *WorkerPool
	maxConcurrent   int
	pollingInterval time.Duration

	mu              sync.RWMutex
	running         bool
	cancel          context.CancelFunc
	resultChan      chan JobResult
	schedulerTicker *time.Ticker
}

// NewJobManager creates a new job manager.
func NewJobManager(service *Service, agentRegistry *agent.Registry, maxConcurrent int) *JobManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	return &JobManager{
		service:         service,
		agentRegistry:   agentRegistry,
		workerPool:      NewWorkerPool(maxConcurrent),
		maxConcurrent:   maxConcurrent,
		pollingInterval: 30 * time.Second, // Check for scheduled jobs every 30s
		resultChan:      make(chan JobResult, 20),
	}
}

// Start starts the job manager.
func (m *JobManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("job manager already running")
	}

	jobCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true

	// Start scheduler polling loop
	m.schedulerTicker = time.NewTicker(m.pollingInterval)
	go m.schedulerLoop(jobCtx)

	return nil
}

// Stop stops the job manager gracefully.
func (m *JobManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}

	if m.cancel != nil {
		m.cancel()
	}

	if m.schedulerTicker != nil {
		m.schedulerTicker.Stop()
	}

	m.running = false
	m.mu.Unlock()

	// Wait for all jobs to complete (with timeout)
	done := make(chan struct{})
	go func() {
		m.workerPool.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All jobs completed
	case <-time.After(30 * time.Second):
		// Timeout - force shutdown
	}
}

// IsRunning returns true if the job manager is running.
func (m *JobManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ResultChannel returns the channel for receiving job results.
func (m *JobManager) ResultChannel() <-chan JobResult {
	return m.resultChan
}

// SubmitJob queues a job for immediate execution.
func (m *JobManager) SubmitJob(ctx context.Context, jobID string, triggeredBy string) error {
	m.mu.RLock()
	running := m.running
	m.mu.RUnlock()

	if !running {
		return fmt.Errorf("job manager not running")
	}

	// Get job details
	job, err := m.service.db.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Get plan details
	plan, err := m.service.db.GetPlanByFileName(ctx, "") // TODO: Need to get plan by ID
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Create execution record
	executionID := uuid.New().String()
	executionNumber, err := m.service.db.GetNextExecutionNumber(ctx, jobID)
	if err != nil {
		executionNumber = 1
	}

	execution, err := m.service.db.InsertExecution(ctx, dto.CreateExecutionParams{
		ID:              executionID,
		JobID:           jobID,
		ExecutionNumber: executionNumber,
		Status:          dto.ExecutionStatusPending,
		TriggeredBy:     triggeredBy,
		CreatedAt:       time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to create execution: %w", err)
	}

	// Submit to worker pool
	task := WorkerTask{
		JobID:       jobID,
		ExecutionID: executionID,
		Ctx:         ctx,
		Execute: func(taskCtx context.Context) error {
			return m.executeJob(taskCtx, job, plan, execution)
		},
	}

	m.workerPool.Submit(task)

	return nil
}

// ScheduleJob schedules a job for future execution (one-time).
func (m *JobManager) ScheduleJob(ctx context.Context, jobID string, scheduledAt time.Time) error {
	m.mu.RLock()
	running := m.running
	m.mu.RUnlock()

	if !running {
		return fmt.Errorf("job manager not running")
	}

	// Validate scheduled time is in the future
	if scheduledAt.Before(time.Now()) {
		return fmt.Errorf("scheduled time must be in the future")
	}

	// Create scheduled job record
	_, err := m.service.db.InsertScheduledJob(ctx, dto.CreateScheduledJobParams{
		ID:          uuid.New().String(),
		JobID:       jobID,
		ScheduledAt: scheduledAt,
		CreatedAt:   time.Now(),
	})

	return err
}

// CancelScheduledJob cancels a scheduled job before execution.
func (m *JobManager) CancelScheduledJob(ctx context.Context, jobID string) error {
	return m.service.db.CancelScheduledJob(ctx, jobID)
}

// CancelExecution cancels a running execution.
func (m *JobManager) CancelExecution(ctx context.Context, executionID string) error {
	if m.workerPool.CancelTask(executionID) {
		// Update execution status
		now := time.Now()
		return m.service.db.UpdateExecution(ctx, dto.UpdateExecutionParams{
			ID:          executionID,
			Status:      dto.ExecutionStatusCancelled,
			CompletedAt: &now,
		})
	}

	return fmt.Errorf("execution not found or not running")
}

// GetActiveExecutionCount returns the number of currently running executions.
func (m *JobManager) GetActiveExecutionCount() int {
	return m.workerPool.ActiveCount()
}

// schedulerLoop polls for due scheduled jobs.
func (m *JobManager) schedulerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.schedulerTicker.C:
			m.checkScheduledJobs(ctx)
		}
	}
}

// checkScheduledJobs checks for and triggers due scheduled jobs.
func (m *JobManager) checkScheduledJobs(ctx context.Context) {
	now := time.Now()

	// Get due scheduled jobs
	scheduled, err := m.service.db.ListDueScheduledJobs(ctx, now)
	if err != nil {
		// Log error but don't crash
		return
	}

	for _, sched := range scheduled {
		if sched.Cancelled {
			continue
		}

		// Trigger job
		if err := m.SubmitJob(ctx, sched.JobID, "scheduled"); err != nil {
			// Log error but continue with other jobs
			continue
		}

		// Delete scheduled job after triggering
		_ = m.service.db.DeleteScheduledJob(ctx, sched.JobID)
	}
}

// executeJob executes a single job.
func (m *JobManager) executeJob(ctx context.Context, job dto.BackgroundJob, plan dto.Plan, execution dto.JobExecution) error {
	// Update job and execution status to running
	now := time.Now()
	if err := m.service.db.UpdateJobStatus(ctx, job.ID, dto.JobStatusRunning, &now); err != nil {
		return err
	}

	if err := m.service.db.UpdateExecution(ctx, dto.UpdateExecutionParams{
		ID:        execution.ID,
		Status:    dto.ExecutionStatusRunning,
		StartedAt: &now,
	}); err != nil {
		return err
	}

	// Get agent provider
	provider, err := m.agentRegistry.Get(job.AgentProvider)
	if err != nil {
		m.sendResult(JobResult{
			JobID:        job.ID,
			ExecutionID:  execution.ID,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to get agent provider: %v", err),
			StartedAt:    now,
			CompletedAt:  time.Now(),
		})
		return err
	}

	// Create executor
	executor := provider.CreateExecutor()

	// Execute plan
	result, err := executor.Execute(ctx, agent.ExecutionConfig{
		PlanPath:    plan.FilePath,
		AgentConfig: job.AgentConfig,
		Timeout:     30 * time.Minute, // TODO: Make configurable
	})

	completedAt := time.Now()
	if result == nil {
		result = &agent.ExecutionResult{
			ExitCode:     -1,
			ErrorMessage: "execution failed with no result",
			StartedAt:    now,
			CompletedAt:  completedAt,
		}
	}

	// Determine final status
	var finalStatus dto.ExecutionStatus
	var jobStatus dto.JobStatus
	if result.Cancelled {
		finalStatus = dto.ExecutionStatusCancelled
		jobStatus = dto.JobStatusCancelled
	} else if err != nil || result.ExitCode != 0 {
		finalStatus = dto.ExecutionStatusFailed
		jobStatus = dto.JobStatusFailed
	} else {
		finalStatus = dto.ExecutionStatusCompleted
		jobStatus = dto.JobStatusCompleted
	}

	// Update execution record
	exitCode := result.ExitCode
	outputLog := result.Output
	errorMsg := result.ErrorMessage

	if err := m.service.db.UpdateExecution(ctx, dto.UpdateExecutionParams{
		ID:           execution.ID,
		Status:       finalStatus,
		CompletedAt:  &completedAt,
		ExitCode:     &exitCode,
		OutputLog:    &outputLog,
		ErrorMessage: &errorMsg,
	}); err != nil {
		return err
	}

	// Update job status
	if err := m.service.db.UpdateJobStatus(ctx, job.ID, jobStatus, &completedAt); err != nil {
		return err
	}

	// Send result to channel
	m.sendResult(JobResult{
		JobID:        job.ID,
		ExecutionID:  execution.ID,
		Success:      finalStatus == dto.ExecutionStatusCompleted,
		ExitCode:     result.ExitCode,
		Output:       result.Output,
		ErrorMessage: result.ErrorMessage,
		StartedAt:    result.StartedAt,
		CompletedAt:  result.CompletedAt,
		Cancelled:    result.Cancelled,
	})

	return nil
}

// sendResult sends a job result to the result channel (non-blocking).
func (m *JobManager) sendResult(result JobResult) {
	select {
	case m.resultChan <- result:
		// Sent successfully
	default:
		// Channel full, drop result (log error in production)
	}
}
