// Package agent provides interfaces and implementations for executing background jobs
// with various agent providers (Claude Code CLI, custom agents, etc.)
package agent

import (
	"context"
	"time"
)

// ExecutionConfig contains configuration for executing an agent job.
type ExecutionConfig struct {
	// PlanPath is the absolute path to the plan file to execute
	PlanPath string

	// WorkingDir is the directory where the agent should run
	WorkingDir string

	// Timeout specifies the maximum execution time (0 means no timeout)
	Timeout time.Duration

	// AgentConfig is provider-specific configuration (JSON-encoded)
	AgentConfig string

	// Environment variables to pass to the agent process
	Env map[string]string
}

// ExecutionResult contains the results of an agent execution.
type ExecutionResult struct {
	// ExitCode is the process exit code
	ExitCode int

	// Stdout contains the standard output from the agent
	Stdout string

	// Stderr contains the standard error from the agent
	Stderr string

	// Error is any error that occurred during execution (non-zero exit code, timeout, etc.)
	Error error

	// StartedAt is when execution began
	StartedAt time.Time

	// CompletedAt is when execution finished
	CompletedAt time.Time

	// Cancelled indicates if the execution was cancelled
	Cancelled bool
}

// OutputCallback is called when streaming output from an agent execution.
// It receives chunks of stdout/stderr as they become available.
type OutputCallback func(output string, isStderr bool)

// Executor represents an agent that can execute plan files.
type Executor interface {
	// Execute runs the agent with the given configuration and waits for completion.
	// Returns the execution result or an error if execution fails to start.
	Execute(ctx context.Context, config ExecutionConfig) (*ExecutionResult, error)

	// ExecuteStreaming runs the agent and streams output via the callback.
	// The callback is invoked for each chunk of output (stdout or stderr).
	// Returns the execution result after completion.
	ExecuteStreaming(ctx context.Context, config ExecutionConfig, callback OutputCallback) (*ExecutionResult, error)

	// Cancel attempts to cancel a running execution.
	// Returns an error if cancellation fails.
	Cancel(ctx context.Context) error

	// IsRunning returns true if the executor currently has a running process.
	IsRunning() bool
}

// Provider represents a factory for creating agent executors.
// Different providers support different agent types (Claude Code CLI, custom agents, etc.)
type Provider interface {
	// Name returns the unique name of this provider (e.g., "claude-code")
	Name() string

	// CreateExecutor creates a new executor instance for this provider.
	CreateExecutor() Executor

	// ValidateConfig validates provider-specific configuration (JSON).
	// Returns an error if the configuration is invalid.
	ValidateConfig(config string) error

	// Description returns a human-readable description of this provider.
	Description() string
}
