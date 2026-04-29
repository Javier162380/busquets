// Package providers contains implementations of various agent providers.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/agent"
)

// ClaudeCodeConfig represents the configuration for the Claude Code provider.
type ClaudeCodeConfig struct {
	// ClaudeCodePath is the path to the claude-code CLI executable
	// If empty, will search in PATH
	ClaudeCodePath string `json:"claude_code_path,omitempty"`

	// Model is the Claude model to use (e.g., "sonnet-4.5")
	Model string `json:"model,omitempty"`

	// MaxTokens is the maximum number of tokens for the conversation
	MaxTokens int `json:"max_tokens,omitempty"`

	// AdditionalArgs are extra arguments to pass to claude-code
	AdditionalArgs []string `json:"additional_args,omitempty"`
}

// ClaudeCodeProvider implements the Provider interface for Claude Code CLI.
type ClaudeCodeProvider struct{}

// Name returns the provider name.
func (p *ClaudeCodeProvider) Name() string {
	return "claude-code"
}

// Description returns the provider description.
func (p *ClaudeCodeProvider) Description() string {
	return "Claude Code CLI - Execute plans using the claude-code command-line tool"
}

// CreateExecutor creates a new Claude Code executor.
func (p *ClaudeCodeProvider) CreateExecutor() agent.Executor {
	return &ClaudeCodeExecutor{}
}

// ValidateConfig validates the Claude Code configuration.
func (p *ClaudeCodeProvider) ValidateConfig(configJSON string) error {
	if configJSON == "" {
		return nil // Empty config is valid - will use defaults
	}

	var config ClaudeCodeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return fmt.Errorf("invalid JSON configuration: %w", err)
	}

	// If a custom path is specified, verify it exists and is executable
	if config.ClaudeCodePath != "" {
		if _, err := os.Stat(config.ClaudeCodePath); err != nil {
			return fmt.Errorf("claude-code executable not found at %s: %w", config.ClaudeCodePath, err)
		}
	}

	return nil
}

// ClaudeCodeExecutor implements the Executor interface for Claude Code CLI.
type ClaudeCodeExecutor struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	cancel  context.CancelFunc
}

// Execute runs claude-code plan execute and waits for completion.
func (e *ClaudeCodeExecutor) Execute(ctx context.Context, config agent.ExecutionConfig) (*agent.ExecutionResult, error) {
	return e.execute(ctx, config, nil)
}

// ExecuteStreaming runs claude-code and streams output via callback.
func (e *ClaudeCodeExecutor) ExecuteStreaming(ctx context.Context, config agent.ExecutionConfig, callback agent.OutputCallback) (*agent.ExecutionResult, error) {
	return e.execute(ctx, config, callback)
}

// execute is the internal implementation that handles both Execute and ExecuteStreaming.
func (e *ClaudeCodeExecutor) execute(ctx context.Context, config agent.ExecutionConfig, callback agent.OutputCallback) (*agent.ExecutionResult, error) {
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
		e.cmd = nil
		e.cancel = nil
		e.mu.Unlock()
	}()

	result := &agent.ExecutionResult{
		StartedAt: time.Now(),
	}

	// Parse agent config
	var agentConfig ClaudeCodeConfig
	if config.AgentConfig != "" {
		if err := json.Unmarshal([]byte(config.AgentConfig), &agentConfig); err != nil {
			result.Error = fmt.Errorf("invalid agent config: %w", err)
			return result, result.Error
		}
	}

	// Determine claude-code executable path
	claudeCodePath := agentConfig.ClaudeCodePath
	if claudeCodePath == "" {
		claudeCodePath = "claude-code" // Will search in PATH
	}

	// Build command arguments
	args := []string{"plan", "execute", config.PlanPath}

	// Add model flag if specified
	if agentConfig.Model != "" {
		args = append(args, "--model", agentConfig.Model)
	}

	// Add max-tokens flag if specified
	if agentConfig.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", agentConfig.MaxTokens))
	}

	// Add any additional arguments
	args = append(args, agentConfig.AdditionalArgs...)

	// Create command with timeout context if specified
	cmdCtx := ctx
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	// Store cancel function for Cancel() method
	cmdCtx, e.cancel = context.WithCancel(cmdCtx)

	cmd := exec.CommandContext(cmdCtx, claudeCodePath, args...)
	cmd.Dir = config.WorkingDir

	// Set environment variables
	if len(config.Env) > 0 {
		cmd.Env = os.Environ()
		for key, value := range config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	e.mu.Lock()
	e.cmd = cmd
	e.mu.Unlock()

	var stdout, stderr bytes.Buffer

	if callback != nil {
		// Streaming mode: pipe output to callback and also capture it
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			result.Error = fmt.Errorf("failed to create stdout pipe: %w", err)
			return result, result.Error
		}

		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			result.Error = fmt.Errorf("failed to create stderr pipe: %w", err)
			return result, result.Error
		}

		if err := cmd.Start(); err != nil {
			result.Error = fmt.Errorf("failed to start command: %w", err)
			return result, result.Error
		}

		// Stream stdout and stderr concurrently
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			buf := make([]byte, 1024)
			for {
				n, err := stdoutPipe.Read(buf)
				if n > 0 {
					chunk := string(buf[:n])
					stdout.WriteString(chunk)
					callback(chunk, false)
				}
				if err != nil {
					break
				}
			}
		}()

		go func() {
			defer wg.Done()
			buf := make([]byte, 1024)
			for {
				n, err := stderrPipe.Read(buf)
				if n > 0 {
					chunk := string(buf[:n])
					stderr.WriteString(chunk)
					callback(chunk, true)
				}
				if err != nil {
					break
				}
			}
		}()

		wg.Wait()
		err = cmd.Wait()

		result.CompletedAt = time.Now()
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else if cmdCtx.Err() == context.Canceled {
				result.Cancelled = true
				result.Error = fmt.Errorf("execution cancelled")
			} else if cmdCtx.Err() == context.DeadlineExceeded {
				result.Error = fmt.Errorf("execution timeout after %s", config.Timeout)
			} else {
				result.Error = fmt.Errorf("command failed: %w", err)
			}
		}
	} else {
		// Non-streaming mode: capture output in buffers
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		result.CompletedAt = time.Now()
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else if cmdCtx.Err() == context.Canceled {
				result.Cancelled = true
				result.Error = fmt.Errorf("execution cancelled")
			} else if cmdCtx.Err() == context.DeadlineExceeded {
				result.Error = fmt.Errorf("execution timeout after %s", config.Timeout)
			} else {
				result.Error = fmt.Errorf("command failed: %w", err)
			}
		}
	}

	return result, nil
}

// Cancel attempts to cancel a running execution.
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
		// Try graceful termination first
		if err := e.cmd.Process.Signal(os.Interrupt); err != nil {
			// If interrupt fails, force kill
			return e.cmd.Process.Kill()
		}
	}

	return nil
}

// IsRunning returns true if an execution is currently running.
func (e *ClaudeCodeExecutor) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// streamPipe reads from a pipe and writes to both a buffer and callback.
func streamPipe(pipe io.Reader, buf *bytes.Buffer, callback agent.OutputCallback, isStderr bool) {
	scanner := make([]byte, 1024)
	for {
		n, err := pipe.Read(scanner)
		if n > 0 {
			chunk := string(scanner[:n])
			buf.WriteString(chunk)
			if callback != nil {
				callback(chunk, isStderr)
			}
		}
		if err != nil {
			break
		}
	}
}
