// Package providers contains agent provider implementations.
package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/agent"
)

// ClaudeCodeConfig is the configuration for Claude Code CLI.
type ClaudeCodeConfig struct {
	ClaudeCodePath string `json:"claude_code_path"`
	Model          string `json:"model,omitempty"`
	MaxTokens      int    `json:"max_tokens,omitempty"`
}

// ClaudeCodeProvider implements the Provider interface for Claude Code CLI.
type ClaudeCodeProvider struct{}

// Name returns the provider name.
func (p *ClaudeCodeProvider) Name() string {
	return "claude-code"
}

// CreateExecutor creates a new Claude Code executor.
func (p *ClaudeCodeProvider) CreateExecutor() agent.Executor {
	return &ClaudeCodeExecutor{}
}

// ValidateConfig validates Claude Code configuration.
func (p *ClaudeCodeProvider) ValidateConfig(config string) error {
	if config == "" {
		return nil // Empty config is allowed (uses defaults)
	}

	var cfg ClaudeCodeConfig
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return fmt.Errorf("invalid JSON config: %w", err)
	}

	return nil
}

// ClaudeCodeExecutor executes plans using Claude Code CLI.
type ClaudeCodeExecutor struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	running bool
}

// Execute runs Claude Code synchronously.
func (e *ClaudeCodeExecutor) Execute(ctx context.Context, config agent.ExecutionConfig) (*agent.ExecutionResult, error) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil, fmt.Errorf("executor already running")
	}
	e.running = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.running = false
		e.cancel = nil
		e.cmd = nil
		e.mu.Unlock()
	}()

	// Parse config
	cfg, err := parseConfig(config.AgentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	e.mu.Lock()
	e.cancel = cancel
	e.mu.Unlock()

	// Build command
	claudeCodePath := cfg.ClaudeCodePath
	if claudeCodePath == "" {
		claudeCodePath = "claude-code" // Use PATH
	}

	args := []string{"plan", "execute", config.PlanPath}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", cfg.MaxTokens))
	}

	cmd := exec.CommandContext(execCtx, claudeCodePath, args...)
	e.mu.Lock()
	e.cmd = cmd
	e.mu.Unlock()

	// Execute and capture output
	startedAt := time.Now()
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	result := &agent.ExecutionResult{
		Output:      string(output),
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Cancelled:   execCtx.Err() == context.Canceled,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.ErrorMessage = err.Error()
		} else {
			result.ExitCode = -1
			result.ErrorMessage = err.Error()
		}
	}

	return result, nil
}

// ExecuteStreaming runs Claude Code with real-time output streaming.
func (e *ClaudeCodeExecutor) ExecuteStreaming(ctx context.Context, config agent.ExecutionConfig, callback func(string)) (*agent.ExecutionResult, error) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil, fmt.Errorf("executor already running")
	}
	e.running = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.running = false
		e.cancel = nil
		e.cmd = nil
		e.mu.Unlock()
	}()

	// Parse config
	cfg, err := parseConfig(config.AgentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	e.mu.Lock()
	e.cancel = cancel
	e.mu.Unlock()

	// Build command
	claudeCodePath := cfg.ClaudeCodePath
	if claudeCodePath == "" {
		claudeCodePath = "claude-code"
	}

	args := []string{"plan", "execute", config.PlanPath}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", cfg.MaxTokens))
	}

	cmd := exec.CommandContext(execCtx, claudeCodePath, args...)
	e.mu.Lock()
	e.cmd = cmd
	e.mu.Unlock()

	// Set up output pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Stream output
	var outputBuilder string
	var wg sync.WaitGroup

	streamOutput := func(r io.Reader, prefix string) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuilder += line + "\n"
			if callback != nil {
				callback(prefix + line)
			}
		}
	}

	wg.Add(2)
	go streamOutput(stdout, "[stdout] ")
	go streamOutput(stderr, "[stderr] ")

	// Wait for output streaming to complete
	wg.Wait()

	// Wait for command to finish
	err = cmd.Wait()
	completedAt := time.Now()

	result := &agent.ExecutionResult{
		Output:      outputBuilder,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Cancelled:   execCtx.Err() == context.Canceled,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.ErrorMessage = err.Error()
		} else {
			result.ExitCode = -1
			result.ErrorMessage = err.Error()
		}
	}

	return result, nil
}

// Cancel cancels the running execution.
func (e *ClaudeCodeExecutor) Cancel(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("no execution running")
	}

	if e.cancel != nil {
		e.cancel()
	}

	if e.cmd != nil && e.cmd.Process != nil {
		if err := e.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	return nil
}

// IsRunning returns true if an execution is running.
func (e *ClaudeCodeExecutor) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// parseConfig parses the agent config JSON.
func parseConfig(configJSON string) (*ClaudeCodeConfig, error) {
	if configJSON == "" {
		return &ClaudeCodeConfig{}, nil
	}

	var cfg ClaudeCodeConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
