package claudeviewer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// JSONLMessage represents a single line from a Claude Code session JSONL file.
type JSONLMessage struct {
	Type        string          `json:"type"`
	Subtype     string          `json:"subtype,omitempty"`
	SessionID   string          `json:"sessionId,omitempty"`
	UUID        string          `json:"uuid,omitempty"`
	ParentUUID  string          `json:"parentUuid,omitempty"`
	MessageID   string          `json:"messageId,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
	CWD         string          `json:"cwd,omitempty"`
	GitBranch   string          `json:"gitBranch,omitempty"`
	Slug        string          `json:"slug,omitempty"`
	IsMeta      bool            `json:"isMeta,omitempty"`
	IsSidechain bool            `json:"isSidechain,omitempty"`
	Message     *MessageContent `json:"message,omitempty"`
	Todos       []TodoItem      `json:"todos,omitempty"`
}

// MessageContent represents the message content in a JSONL line.
type MessageContent struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// GetContentAsString extracts content as a string, handling both string and array formats.
func (m *MessageContent) GetContentAsString() string {
	if m.Content == nil {
		return ""
	}

	// Try to unmarshal as a string first
	var contentStr string
	if err := json.Unmarshal(m.Content, &contentStr); err == nil {
		return contentStr
	}

	// Try to unmarshal as an array of content blocks
	var contentBlocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(m.Content, &contentBlocks); err == nil {
		// Concatenate all text blocks
		var parts []string
		for _, block := range contentBlocks {
			if block.Type == "text" && block.Text != "" {
				parts = append(parts, block.Text)
			}
		}
		return strings.Join(parts, "\n")
	}

	// Fallback: return the raw JSON as string
	return string(m.Content)
}

// TodoItem represents a todo item from a session.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// SessionMetadata contains metadata extracted from a JSONL session file.
type SessionMetadata struct {
	SessionUUID    string
	ProjectPath    string
	ProjectName    string
	JSONLFilePath  string
	MessageCount   int
	FirstMessageAt *time.Time
	LastMessageAt  *time.Time
	CWD            string
	GitBranch      string
	Slug           string
}

// ParseJSONLFile reads and parses a Claude Code JSONL session file.
// Returns all messages from the file.
func ParseJSONLFile(filePath string) ([]JSONLMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file: %w", err)
	}
	defer file.Close()

	var messages []JSONLMessage
	scanner := bufio.NewScanner(file)

	// Increase buffer size for large messages
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg JSONLMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Log but don't fail on parse errors for individual lines
			fmt.Fprintf(os.Stderr, "Warning: failed to parse line %d in %s: %v\n", lineNum, filePath, err)
			continue
		}

		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning JSONL file: %w", err)
	}

	return messages, nil
}

// ExtractSessionMetadata extracts session metadata from parsed messages.
func ExtractSessionMetadata(messages []JSONLMessage, jsonlFilePath string) (*SessionMetadata, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages in session")
	}

	metadata := &SessionMetadata{
		JSONLFilePath: jsonlFilePath,
		MessageCount:  len(messages),
	}

	// Get session UUID from first message
	for _, msg := range messages {
		if msg.SessionID != "" {
			metadata.SessionUUID = msg.SessionID
			break
		}
	}

	if metadata.SessionUUID == "" {
		return nil, fmt.Errorf("no session ID found in messages")
	}

	// Extract project path from file path
	// File path format: ~/.claude/projects/{project-encoded-path}/{session-uuid}.jsonl
	projectPath, projectName, err := extractProjectFromPath(jsonlFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract project from path: %w", err)
	}
	metadata.ProjectPath = projectPath
	metadata.ProjectName = projectName

	// Find first and last message timestamps
	for i, msg := range messages {
		if !msg.Timestamp.IsZero() {
			if metadata.FirstMessageAt == nil || msg.Timestamp.Before(*metadata.FirstMessageAt) {
				ts := msg.Timestamp
				metadata.FirstMessageAt = &ts
			}
			if metadata.LastMessageAt == nil || msg.Timestamp.After(*metadata.LastMessageAt) {
				ts := msg.Timestamp
				metadata.LastMessageAt = &ts
			}
		}

		// Get latest non-empty values for CWD, GitBranch, Slug
		if msg.CWD != "" {
			metadata.CWD = msg.CWD
		}
		if msg.GitBranch != "" {
			metadata.GitBranch = msg.GitBranch
		}
		if msg.Slug != "" && i == 0 {
			// Slug is typically in the first message
			metadata.Slug = msg.Slug
		}
	}

	return metadata, nil
}

func extractProjectFromPath(jsonlFilePath string) (string, string, error) {
	// Get the parent directory
	parentDir := filepath.Dir(jsonlFilePath)

	// Check if this is a subagent session
	// Path format for subagents: ~/.claude/projects/{encoded-project-path}/{parent-session-uuid}/subagents/agent-{id}.jsonl
	// Path format for regular sessions: ~/.claude/projects/{encoded-project-path}/{session-uuid}.jsonl
	if filepath.Base(parentDir) == "subagents" {
		// This is a subagent, go up two more levels to get the project directory
		// parentDir is "subagents", go to parent session dir, then to project dir
		parentSessionDir := filepath.Dir(parentDir)
		parentDir = filepath.Dir(parentSessionDir)
	}

	// Get the directory name (encoded project path)
	encodedPath := filepath.Base(parentDir)

	// Decode the path: replace - with / and remove leading dash
	if !strings.HasPrefix(encodedPath, "-") {
		return "", "", fmt.Errorf("invalid project directory format: %s", encodedPath)
	}

	// Remove leading dash and replace remaining dashes with slashes
	decodedPath := "/" + strings.ReplaceAll(encodedPath[1:], "-", "/")

	// Extract project name (last component of path)
	projectName := filepath.Base(decodedPath)

	return decodedPath, projectName, nil
}

// ParseJSONLFileFromOffset reads JSONL file starting from a specific line offset.
// Useful for reading only new messages added to a file.
func ParseJSONLFileFromOffset(filePath string, lineOffset int) ([]JSONLMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file: %w", err)
	}
	defer file.Close()

	var messages []JSONLMessage
	scanner := bufio.NewScanner(file)

	// Increase buffer size for large messages
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++

		// Skip lines before offset
		if lineNum <= lineOffset {
			continue
		}

		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg JSONLMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse line %d in %s: %v\n", lineNum, filePath, err)
			continue
		}

		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning JSONL file: %w", err)
	}

	return messages, nil
}

// CountJSONLLines counts the number of non-empty lines in a JSONL file.
// Useful for detecting when new messages have been added.
func CountJSONLLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error scanning file: %w", err)
	}

	return count, nil
}
