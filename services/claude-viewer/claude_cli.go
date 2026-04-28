package claudeviewer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ClaudeCLI wraps the claude command-line interface.
type ClaudeCLI struct {
	binaryPath string
	timeout    time.Duration
}

// NewClaudeCLI creates a new Claude CLI wrapper.
// It attempts to find the claude binary in PATH.
func NewClaudeCLI() (*ClaudeCLI, error) {
	binaryPath, err := findClaudeBinary()
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found: %w", err)
	}

	return &ClaudeCLI{
		binaryPath: binaryPath,
		timeout:    30 * time.Second, // Default 30s timeout
	}, nil
}

// SetTimeout sets the timeout for Claude CLI commands.
func (c *ClaudeCLI) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// SendMessage sends a message to an existing Claude session.
func (c *ClaudeCLI) SendMessage(ctx context.Context, sessionUUID, message string) (string, error) {
	if sessionUUID == "" {
		return "", fmt.Errorf("session UUID cannot be empty")
	}
	if message == "" {
		return "", fmt.Errorf("message cannot be empty")
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build command: claude -r <uuid> -p "<message>"
	cmd := exec.CommandContext(cmdCtx, c.binaryPath, "-r", sessionUUID, "-p", message)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("claude command failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

// StartSession starts a new Claude session with an initial message.
// Returns the session UUID.
func (c *ClaudeCLI) StartSession(ctx context.Context, cwd, initialMessage string) (string, error) {
	// Use a default message if none provided
	if initialMessage == "" {
		initialMessage = "Hello"
	}

	// Resolve working directory to absolute path
	if cwd != "" {
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to resolve working directory: %w", err)
		}
		cwd = absCwd
	}

	// Generate a new session UUID
	sessionUUID := uuid.New().String()

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build command: claude --session-id <uuid> -p "<message>"
	cmd := exec.CommandContext(cmdCtx, c.binaryPath, "--session-id", sessionUUID, "-p", initialMessage)

	// Set working directory if provided
	if cwd != "" {
		cmd.Dir = cwd
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("claude command failed: %w (stderr: %s)", err, stderr.String())
	}

	return sessionUUID, nil
}

// StartSessionFromPlan starts a new Claude session initialized with plan content.
// The plan content is provided as the initial message.
func (c *ClaudeCLI) StartSessionFromPlan(ctx context.Context, cwd, planTitle, planContent string) (string, error) {
	// Build initial message with plan context
	initialMessage := fmt.Sprintf(`I'm working on: %s

Here's the plan:

%s

Please help me implement this.`, planTitle, planContent)

	return c.StartSession(ctx, cwd, initialMessage)
}

// GetVersion gets the Claude CLI version.
func (c *ClaudeCLI) GetVersion(ctx context.Context) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.binaryPath, "--version")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// IsAvailable checks if the Claude CLI is available and working.
func (c *ClaudeCLI) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binaryPath, "--version")
	err := cmd.Run()
	return err == nil
}

// GetSessionJSONLPath returns the expected path to a session's JSONL file.
func GetSessionJSONLPath(sessionUUID, projectPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Resolve to absolute path if relative
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project path: %w", err)
	}

	// Encode project path: replace / with - and prepend -
	encodedPath := "-" + strings.ReplaceAll(strings.TrimPrefix(absPath, "/"), "/", "-")

	// Build path: ~/.claude/projects/{encoded-path}/{session-uuid}.jsonl
	jsonlPath := filepath.Join(homeDir, ".claude", "projects", encodedPath, sessionUUID+".jsonl")

	return jsonlPath, nil
}

// DiscoverAllSessions scans ~/.claude/projects/ to find all session JSONL files.
func DiscoverAllSessions() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")

	// Check if projects directory exists
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return []string{}, nil // No sessions yet
	}

	var sessionFiles []string

	// Walk the projects directory
	err = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't read
			return nil
		}

		// Only interested in .jsonl files
		if !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			sessionFiles = append(sessionFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk projects directory: %w", err)
	}

	return sessionFiles, nil
}

// SendMessageWithJSON sends a message and returns the response in JSON format.
func (c *ClaudeCLI) SendMessageWithJSON(ctx context.Context, sessionUUID, message string) (map[string]interface{}, error) {
	if sessionUUID == "" {
		return nil, fmt.Errorf("session UUID cannot be empty")
	}
	if message == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build command: claude -r <uuid> -p --output-format json "<message>"
	cmd := exec.CommandContext(cmdCtx, c.binaryPath, "-r", sessionUUID, "-p", "--output-format", "json", message)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("claude command failed: %w (stderr: %s)", err, stderr.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result, nil
}

// findClaudeBinary attempts to locate the claude binary in PATH.
func findClaudeBinary() (string, error) {
	// Try to find claude in PATH
	path, err := exec.LookPath("claude")
	if err == nil {
		return path, nil
	}

	// Common installation locations
	commonPaths := []string{
		"/usr/local/bin/claude",
		"/usr/bin/claude",
		"/opt/homebrew/bin/claude",
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "claude"),
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("claude binary not found in PATH or common locations")
}

// ValidateSessionUUID validates that a string is a valid session UUID.
func ValidateSessionUUID(sessionUUID string) error {
	if sessionUUID == "" {
		return fmt.Errorf("session UUID cannot be empty")
	}

	// Validate UUID format
	if _, err := uuid.Parse(sessionUUID); err != nil {
		return fmt.Errorf("invalid session UUID format: %w", err)
	}

	return nil
}
