// Package agent provides interfaces and implementations for executing plan agents.
package agent

import (
	"context"
	"time"
)

// ExecutionResult represents the result of an agent execution.
type ExecutionResult struct {
	ExitCode     int
	Output       string
	ErrorMessage string
	StartedAt    time.Time
	CompletedAt  time.Time
	Cancelled    bool
}

// Executor defines the interface for executing agent tasks.
type Executor interface {
	// Execute runs the agent with the given configuration synchronously.
	Execute(ctx context.Context, config ExecutionConfig) (*ExecutionResult, error)

	// ExecuteStreaming runs the agent with real-time output streaming.
	ExecuteStreaming(ctx context.Context, config ExecutionConfig, callback func(string)) (*ExecutionResult, error)

	// Cancel cancels the currently running execution.
	Cancel(ctx context.Context) error

	// IsRunning returns true if an execution is currently running.
	IsRunning() bool
}

// ExecutionConfig contains configuration for executing an agent.
type ExecutionConfig struct {
	// PlanPath is the path to the plan file to execute.
	PlanPath string

	// AgentConfig is provider-specific configuration (JSON).
	AgentConfig string

	// Timeout is the maximum execution duration.
	Timeout time.Duration
}

// Provider defines the interface for agent providers.
type Provider interface {
	// Name returns the provider name (e.g., "claude-code").
	Name() string

	// CreateExecutor creates a new executor instance.
	CreateExecutor() Executor

	// ValidateConfig validates provider-specific configuration.
	ValidateConfig(config string) error
}
