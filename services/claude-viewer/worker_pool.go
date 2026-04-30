package claudeviewer

import (
	"context"
	"sync"
)

// WorkerTask represents a task to be executed by the worker pool.
type WorkerTask struct {
	JobID       string
	ExecutionID string
	Ctx         context.Context
	Execute     func(context.Context) error
}

// WorkerPool manages a pool of workers for executing background jobs.
type WorkerPool struct {
	maxWorkers int
	semaphore  chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	activeTasks map[string]context.CancelFunc // executionID -> cancel
}

// NewWorkerPool creates a new worker pool with the given max workers.
func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	return &WorkerPool{
		maxWorkers:  maxWorkers,
		semaphore:   make(chan struct{}, maxWorkers),
		activeTasks: make(map[string]context.CancelFunc),
	}
}

// Submit submits a task to the worker pool.
// If all workers are busy, this blocks until a worker becomes available.
func (p *WorkerPool) Submit(task WorkerTask) {
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		// Acquire semaphore (blocks if pool is full)
		p.semaphore <- struct{}{}
		defer func() { <-p.semaphore }()

		// Create cancellable context for this task
		taskCtx, cancel := context.WithCancel(task.Ctx)
		defer cancel()

		// Register task for cancellation
		p.mu.Lock()
		p.activeTasks[task.ExecutionID] = cancel
		p.mu.Unlock()

		defer func() {
			p.mu.Lock()
			delete(p.activeTasks, task.ExecutionID)
			p.mu.Unlock()
		}()

		// Execute task
		_ = task.Execute(taskCtx)
	}()
}

// CancelTask cancels a running task by execution ID.
func (p *WorkerPool) CancelTask(executionID string) bool {
	p.mu.RLock()
	cancel, exists := p.activeTasks[executionID]
	p.mu.RUnlock()

	if exists && cancel != nil {
		cancel()
		return true
	}

	return false
}

// Wait waits for all tasks to complete.
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// ActiveCount returns the number of currently active tasks.
func (p *WorkerPool) ActiveCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.activeTasks)
}

// MaxWorkers returns the maximum number of concurrent workers.
func (p *WorkerPool) MaxWorkers() int {
	return p.maxWorkers
}
