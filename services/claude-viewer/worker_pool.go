package claudeviewer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/agent"
)

// JobTask represents a task to be executed by the worker pool.
type JobTask struct {
	// JobID is the unique identifier for the job
	JobID string

	// ExecutionID is the unique identifier for this execution
	ExecutionID string

	// PlanPath is the path to the plan file to execute
	PlanPath string

	// AgentProvider is the name of the agent provider to use
	AgentProvider string

	// AgentConfig is the JSON configuration for the agent
	AgentConfig string

	// TriggeredBy indicates how the job was triggered (manual, scheduled, api)
	TriggeredBy string

	// WorkingDir is the directory where the agent should run
	WorkingDir string

	// Timeout specifies the maximum execution time
	Timeout time.Duration

	// Env contains environment variables for the execution
	Env map[string]string
}

// JobResult contains the result of a job execution.
type JobResult struct {
	// JobID is the job identifier
	JobID string

	// ExecutionID is the execution identifier
	ExecutionID string

	// Success indicates if the job completed successfully
	Success bool

	// ExitCode is the process exit code
	ExitCode int

	// Output contains stdout from the execution
	Output string

	// ErrorMessage contains stderr or error details
	ErrorMessage string

	// StartedAt is when execution began
	StartedAt time.Time

	// CompletedAt is when execution finished
	CompletedAt time.Time

	// Cancelled indicates if the execution was cancelled
	Cancelled bool
}

// WorkerPool manages concurrent execution of background jobs.
// It uses a semaphore pattern to limit concurrency and queues tasks when full.
type WorkerPool struct {
	// agentRegistry provides access to agent providers
	agentRegistry *agent.Registry

	// maxConcurrent is the maximum number of concurrent executions
	maxConcurrent int

	// semaphore controls concurrent execution slots
	semaphore chan struct{}

	// taskQueue is the channel for incoming tasks
	taskQueue chan JobTask

	// resultChan is where execution results are sent
	resultChan chan JobResult

	// activeExecutions tracks running executions by execution ID
	activeExecutions sync.Map // map[string]context.CancelFunc

	// wg tracks active workers
	wg sync.WaitGroup

	// ctx is the pool context
	ctx context.Context

	// cancel cancels the pool context
	cancel context.CancelFunc

	// running indicates if the pool is running
	running bool
	mu      sync.RWMutex
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(agentRegistry *agent.Registry, maxConcurrent, queueSize int) *WorkerPool {
	if maxConcurrent <= 0 {
		maxConcurrent = 3 // Default
	}
	if queueSize <= 0 {
		queueSize = 100 // Default queue size
	}

	return &WorkerPool{
		agentRegistry: agentRegistry,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		taskQueue:     make(chan JobTask, queueSize),
		resultChan:    make(chan JobResult, 20), // Buffered result channel
	}
}

// Start starts the worker pool.
func (p *WorkerPool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("worker pool already running")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.running = true

	// Start worker goroutines
	for i := 0; i < p.maxConcurrent; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	return nil
}

// Stop gracefully stops the worker pool.
// It waits for all active jobs to complete or for the context to be cancelled.
func (p *WorkerPool) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = false
	p.mu.Unlock()

	// Cancel pool context to stop accepting new tasks
	p.cancel()

	// Close task queue (no new tasks will be accepted)
	close(p.taskQueue)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers finished gracefully
		close(p.resultChan)
		return nil
	case <-ctx.Done():
		// Timeout - cancel all active executions
		p.activeExecutions.Range(func(key, value interface{}) bool {
			if cancelFunc, ok := value.(context.CancelFunc); ok {
				cancelFunc()
			}
			return true
		})
		// Wait a bit more for cancellations to complete
		select {
		case <-done:
			close(p.resultChan)
			return nil
		case <-time.After(2 * time.Second):
			close(p.resultChan)
			return fmt.Errorf("worker pool shutdown timed out")
		}
	}
}

// Submit submits a task to the worker pool.
// Returns an error if the pool is not running or the queue is full.
func (p *WorkerPool) Submit(task JobTask) error {
	p.mu.RLock()
	if !p.running {
		p.mu.RUnlock()
		return fmt.Errorf("worker pool not running")
	}
	p.mu.RUnlock()

	select {
	case p.taskQueue <- task:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("worker pool shutting down")
	default:
		return fmt.Errorf("worker pool queue is full")
	}
}

// CancelExecution cancels a specific execution by ID.
func (p *WorkerPool) CancelExecution(executionID string) error {
	if value, ok := p.activeExecutions.Load(executionID); ok {
		if cancelFunc, ok := value.(context.CancelFunc); ok {
			cancelFunc()
			return nil
		}
	}
	return fmt.Errorf("execution %s not found or already completed", executionID)
}

// ResultChan returns the channel where execution results are sent.
func (p *WorkerPool) ResultChan() <-chan JobResult {
	return p.resultChan
}

// IsRunning returns true if the pool is running.
func (p *WorkerPool) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// ActiveCount returns the number of currently executing jobs.
func (p *WorkerPool) ActiveCount() int {
	count := 0
	p.activeExecutions.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// worker is the main worker goroutine that processes tasks.
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				// Queue closed, exit worker
				return
			}

			// Acquire semaphore slot (blocks if all slots are full)
			select {
			case p.semaphore <- struct{}{}:
				// Got a slot, execute task
				p.executeTask(task)
				// Release semaphore slot
				<-p.semaphore
			case <-p.ctx.Done():
				// Pool is shutting down
				return
			}

		case <-p.ctx.Done():
			// Pool is shutting down
			return
		}
	}
}

// executeTask executes a single task.
func (p *WorkerPool) executeTask(task JobTask) {
	result := JobResult{
		JobID:       task.JobID,
		ExecutionID: task.ExecutionID,
		StartedAt:   time.Now(),
	}

	// Create cancellable context for this execution
	execCtx, cancel := context.WithCancel(p.ctx)
	defer cancel()

	// Register cancel function
	p.activeExecutions.Store(task.ExecutionID, cancel)
	defer p.activeExecutions.Delete(task.ExecutionID)

	// Get agent provider
	provider, err := p.agentRegistry.Get(task.AgentProvider)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("agent provider not found: %v", err)
		result.CompletedAt = time.Now()
		p.sendResult(result)
		return
	}

	// Create executor
	executor := provider.CreateExecutor()

	// Build execution config
	execConfig := agent.ExecutionConfig{
		PlanPath:    task.PlanPath,
		WorkingDir:  task.WorkingDir,
		Timeout:     task.Timeout,
		AgentConfig: task.AgentConfig,
		Env:         task.Env,
	}

	// Execute
	execResult, err := executor.Execute(execCtx, execConfig)
	result.CompletedAt = time.Now()

	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		if execResult != nil {
			result.ExitCode = execResult.ExitCode
			result.Output = execResult.Stdout
			if execResult.Stderr != "" {
				result.ErrorMessage = execResult.Stderr
			}
			result.Cancelled = execResult.Cancelled
		}
	} else if execResult != nil {
		result.Success = execResult.ExitCode == 0
		result.ExitCode = execResult.ExitCode
		result.Output = execResult.Stdout
		result.ErrorMessage = execResult.Stderr
		result.Cancelled = execResult.Cancelled
		result.StartedAt = execResult.StartedAt
		result.CompletedAt = execResult.CompletedAt
	}

	p.sendResult(result)
}

// sendResult sends a result to the result channel without blocking.
func (p *WorkerPool) sendResult(result JobResult) {
	select {
	case p.resultChan <- result:
		// Result sent successfully
	case <-time.After(5 * time.Second):
		// Timeout sending result - result channel might be full
		// Log this but don't block
	}
}
