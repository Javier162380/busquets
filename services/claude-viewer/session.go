package claudeviewer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

type sessionManager struct {
	claudeCLI    *ClaudeCLI
	watchManager *SessionWatchManager
	watchChan    chan SessionWatchResult
}

// initSessionManager initializes the session manager components.
func (s *Service) initSessionManager() error {
	cli, err := NewClaudeCLI()
	if err != nil {
		// Claude CLI not available - log but don't fail
		fmt.Fprintf(os.Stderr, "Warning: Claude CLI not available: %v\n", err)
	}

	s.sessionMgr = &sessionManager{
		claudeCLI:    cli,
		watchManager: NewSessionWatchManager(),
		watchChan:    make(chan SessionWatchResult, 100),
	}

	return nil
}

// DiscoverSessions scans ~/.claude/projects/ and imports all Claude sessions.
func (s *Service) DiscoverSessions(ctx context.Context) (int, error) {
	sessionFiles, err := DiscoverAllSessions()
	if err != nil {
		return 0, fmt.Errorf("failed to discover sessions: %w", err)
	}

	importedCount := 0
	for _, jsonlPath := range sessionFiles {
		if err := s.ImportSession(ctx, jsonlPath); err != nil {
			continue
		}
		importedCount++
	}

	return importedCount, nil
}

// ImportSession parses a JSONL file and creates/updates the session in the database.
func (s *Service) ImportSession(ctx context.Context, jsonlPath string) error {
	// Parse JSONL file
	messages, err := ParseJSONLFile(jsonlPath)
	if err != nil {
		return fmt.Errorf("failed to parse JSONL file: %w", err)
	}

	if len(messages) == 0 {
		return fmt.Errorf("no messages found in JSONL file")
	}

	// Extract session metadata
	metadata, err := ExtractSessionMetadata(messages, jsonlPath)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Check if session already exists
	existing, err := s.db.GetSessionByUUID(ctx, metadata.SessionUUID)
	now := s.nowProvider.Now()

	switch {
	case errors.Is(err, dto.ErrNotFound):
		// Create new session
		sessionID, err := s.db.InsertSession(ctx, dto.InsertSessionParams{
			SessionUUID:    metadata.SessionUUID,
			ProjectPath:    metadata.ProjectPath,
			ProjectName:    metadata.ProjectName,
			JSONLFilePath:  metadata.JSONLFilePath,
			PlanID:         nil, // TODO: Try to match plan by title/content
			Status:         "active",
			MessageCount:   int64(metadata.MessageCount),
			FirstMessageAt: metadata.FirstMessageAt,
			LastMessageAt:  metadata.LastMessageAt,
			CreatedAt:      now,
			UpdatedAt:      now,
			CWD:            new(metadata.CWD),
			GitBranch:      new(metadata.GitBranch),
			Slug:           new(metadata.Slug),
		})
		if err != nil {
			return fmt.Errorf("failed to insert session: %w", err)
		}

		// Insert messages
		if err := s.insertSessionMessages(ctx, sessionID, messages); err != nil {
			return fmt.Errorf("failed to insert messages: %w", err)
		}
	case err != nil:
		return fmt.Errorf("failed to import session: %w", err)
	default:
		// Update existing session
		if err := s.db.UpdateSession(ctx, dto.UpdateSessionParams{
			ID:             existing.ID,
			Status:         existing.Status,
			MessageCount:   int64(metadata.MessageCount),
			FirstMessageAt: metadata.FirstMessageAt,
			LastMessageAt:  metadata.LastMessageAt,
			UpdatedAt:      now,
			CWD:            new(metadata.CWD),
			GitBranch:      new(metadata.GitBranch),
		}); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Re-import all messages (simple approach - could optimize later)
		if err := s.db.DeleteSessionMessages(ctx, existing.ID); err != nil {
			return fmt.Errorf("failed to delete old messages: %w", err)
		}

		if err := s.insertSessionMessages(ctx, existing.ID, messages); err != nil {
			return fmt.Errorf("failed to insert messages: %w", err)
		}
	}

	return nil
}

// insertSessionMessages inserts all messages from a session into the database.
func (s *Service) insertSessionMessages(ctx context.Context, sessionID int64, messages []JSONLMessage) error {
	now := s.nowProvider.Now()

	for _, msg := range messages {
		var content *string
		var role *string
		if msg.Message != nil {
			contentStr := msg.Message.GetContentAsString()
			content = &contentStr
			role = &msg.Message.Role
		}

		_, err := s.db.InsertSessionMessage(ctx, dto.InsertSessionMessageParams{
			SessionID:      sessionID,
			MessageUUID:    msg.UUID,
			ParentUUID:     new(msg.ParentUUID),
			MessageType:    msg.Type,
			MessageSubtype: new(msg.Subtype),
			Content:        content,
			Role:           role,
			Timestamp:      msg.Timestamp,
			CWD:            new(msg.CWD),
			GitBranch:      new(msg.GitBranch),
			IsMeta:         msg.IsMeta,
			IsSidechain:    msg.IsSidechain,
			CreatedAt:      now,
		})
		if err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}

		// Insert todos if present
		for _, todo := range msg.Todos {
			if err := s.db.InsertSessionTodo(ctx, dto.InsertSessionTodoParams{
				SessionID:  sessionID,
				MessageID:  nil, // Link to message if needed
				Content:    todo.Content,
				Status:     todo.Status,
				ActiveForm: new(todo.ActiveForm),
				CreatedAt:  now,
				UpdatedAt:  now,
			}); err != nil {
				return fmt.Errorf("failed to insert todo: %w", err)
			}
		}
	}

	return nil
}

// ListAllSessions returns all sessions from the database.
func (s *Service) ListAllSessions(ctx context.Context) ([]SessionSummaryInfo, error) {
	sessions, err := s.db.ListAllSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	result := make([]SessionSummaryInfo, len(sessions))
	for i, session := range sessions {
		var lastMessageAt *string
		if session.LastMessageAt != nil {
			formatted := session.LastMessageAt.Format("2006-01-02 15:04:05")
			lastMessageAt = &formatted
		}

		result[i] = SessionSummaryInfo{
			ID:            session.ID,
			SessionUUID:   session.SessionUUID,
			ProjectName:   session.ProjectName,
			Status:        session.Status,
			MessageCount:  session.MessageCount,
			LastMessageAt: lastMessageAt,
			Slug:          session.Slug,
			PlanTitle:     session.PlanTitle,
		}
	}

	return result, nil
}

// GetSessionDetail returns full session details including messages.
func (s *Service) GetSessionDetail(ctx context.Context, sessionUUID string) (*SessionDetailInfo, error) {
	session, err := s.db.GetSessionByUUID(ctx, sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Get messages (limit to last 100 for performance)
	messages, err := s.db.GetSessionMessages(ctx, session.ID, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Get file changes
	fileChanges, err := s.db.GetSessionFileChanges(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file changes: %w", err)
	}

	// Get todos
	todos, err := s.db.GetSessionTodos(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get todos: %w", err)
	}

	// Get associated plan if exists
	var associatedPlan *PlanSummary
	if session.PlanID != nil {
		plan, err := s.db.GetPlanByID(ctx, *session.PlanID)
		if err == nil {
			associatedPlan = &PlanSummary{
				FileName:   plan.FileName,
				Title:      plan.Title,
				ModifiedAt: plan.ModifiedAt,
				FileSize:   plan.FileSize,
			}
		}
	}

	// Convert to info types
	messageInfos := make([]SessionMessageInfo, len(messages))
	for i, msg := range messages {
		messageInfos[i] = SessionMessageInfo{
			MessageUUID: msg.MessageUUID,
			MessageType: msg.MessageType,
			Role:        msg.Role,
			Content:     msg.Content,
			Timestamp:   msg.Timestamp.Format("2006-01-02 15:04:05"),
		}
	}

	fileChangeInfos := make([]SessionFileChangeInfo, len(fileChanges))
	for i, fc := range fileChanges {
		fileChangeInfos[i] = SessionFileChangeInfo{
			FilePath:   fc.FilePath,
			ChangeType: fc.ChangeType,
			DetectedAt: fc.DetectedAt.Format("2006-01-02 15:04:05"),
		}
	}

	todoInfos := make([]SessionTodoInfo, len(todos))
	for i, todo := range todos {
		todoInfos[i] = SessionTodoInfo{
			Content:    todo.Content,
			Status:     todo.Status,
			ActiveForm: todo.ActiveForm,
		}
	}

	var planTitle *string
	if session.PlanID != nil && associatedPlan != nil {
		planTitle = &associatedPlan.Title
	}

	return &SessionDetailInfo{
		Session: SessionInfo{
			SessionUUID:   session.SessionUUID,
			ProjectName:   session.ProjectName,
			JSONLFilePath: session.JSONLFilePath,
			Status:        session.Status,
			PlanTitle:     planTitle,
		},
		Messages:       messageInfos,
		FileChanges:    fileChangeInfos,
		Todos:          todoInfos,
		AssociatedPlan: associatedPlan,
	}, nil
}

// CreateSessionFromPlan creates a new Claude session initialized with a plan's content.
func (s *Service) CreateSessionFromPlan(ctx context.Context, planFileName, initialPrompt string) (*SessionInfo, error) {
	if s.sessionMgr.claudeCLI == nil {
		return nil, fmt.Errorf("claude CLI not available")
	}

	// Get plan details
	plan, err := s.db.GetPlanByFileName(ctx, planFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	// Determine CWD from plan file path
	cwd := filepath.Dir(plan.FilePath)

	// Start Claude session with plan context
	sessionUUID, err := s.sessionMgr.claudeCLI.StartSessionFromPlan(ctx, cwd, plan.Title, plan.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to start Claude session: %w", err)
	}

	// Wait a moment for JSONL file to be created
	time.Sleep(1 * time.Second)

	// Get JSONL path
	jsonlPath, err := GetSessionJSONLPath(sessionUUID, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get JSONL path: %w", err)
	}

	// Import the session
	if err := s.ImportSession(ctx, jsonlPath); err != nil {
		return nil, fmt.Errorf("failed to import new session: %w", err)
	}

	// Link session to plan
	session, err := s.db.GetSessionByUUID(ctx, sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created session: %w", err)
	}

	// TODO: Update session's plan_id once UpdateSessionParams supports it
	_ = plan.ID
	if err := s.db.UpdateSession(ctx, dto.UpdateSessionParams{
		ID:             session.ID,
		Status:         session.Status,
		MessageCount:   session.MessageCount,
		FirstMessageAt: session.FirstMessageAt,
		LastMessageAt:  session.LastMessageAt,
		UpdatedAt:      s.nowProvider.Now(),
		CWD:            session.CWD,
		GitBranch:      session.GitBranch,
	}); err != nil {
		return nil, fmt.Errorf("failed to link session to plan: %w", err)
	}

	return &SessionInfo{
		SessionUUID:   sessionUUID,
		ProjectName:   session.ProjectName,
		JSONLFilePath: jsonlPath,
		Status:        "active",
		PlanTitle:     &plan.Title,
	}, nil
}

// CreateStandaloneSession creates a new standalone Claude session.
func (s *Service) CreateStandaloneSession(ctx context.Context, projectPath, initialMessage string) (*SessionInfo, error) {
	if s.sessionMgr.claudeCLI == nil {
		return nil, fmt.Errorf("claude CLI not available")
	}

	// Start Claude session
	sessionUUID, err := s.sessionMgr.claudeCLI.StartSession(ctx, projectPath, initialMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to start Claude session: %w", err)
	}

	// Wait for JSONL file creation
	time.Sleep(1 * time.Second)

	// Get JSONL path
	jsonlPath, err := GetSessionJSONLPath(sessionUUID, projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get JSONL path: %w", err)
	}

	// Import the session
	if err := s.ImportSession(ctx, jsonlPath); err != nil {
		return nil, fmt.Errorf("failed to import new session: %w", err)
	}

	session, err := s.db.GetSessionByUUID(ctx, sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created session: %w", err)
	}

	return &SessionInfo{
		SessionUUID:   sessionUUID,
		ProjectName:   session.ProjectName,
		JSONLFilePath: jsonlPath,
		Status:        "active",
		PlanTitle:     nil,
	}, nil
}

// DeleteSession deletes a session from the database.
func (s *Service) DeleteSession(ctx context.Context, sessionUUID string) error {
	session, err := s.db.GetSessionByUUID(ctx, sessionUUID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	return s.db.DeleteSession(ctx, session.ID)
}

// SendMessage sends a message to a Claude session.
func (s *Service) SendMessage(ctx context.Context, sessionUUID, message string) error {
	if s.sessionMgr.claudeCLI == nil {
		return fmt.Errorf("claude CLI not available")
	}

	_, err := s.sessionMgr.claudeCLI.SendMessage(ctx, sessionUUID, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// The session watcher will detect and import new messages automatically
	return nil
}

// StartSessionWatch starts monitoring a session for updates.
func (s *Service) StartSessionWatch(ctx context.Context, sessionUUID string) error {
	session, err := s.db.GetSessionByUUID(ctx, sessionUUID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	resultChan, err := s.sessionMgr.watchManager.StartWatching(sessionUUID, session.JSONLFilePath, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to start watching: %w", err)
	}

	// Forward results to the unified watch channel
	go func() {
		for result := range resultChan {
			s.sessionMgr.watchChan <- result
		}
	}()

	return nil
}

// StopSessionWatch stops monitoring a session.
func (s *Service) StopSessionWatch(ctx context.Context, sessionUUID string) error {
	return s.sessionMgr.watchManager.StopWatching(sessionUUID)
}

// GetSessionWatchResultChannel returns the channel that receives session watch results.
func (s *Service) GetSessionWatchResultChannel() <-chan SessionWatchResult {
	if s.sessionMgr == nil {
		return nil
	}
	return s.sessionMgr.watchChan
}

// GetSessionFileChanges returns file changes tracked for a session.
func (s *Service) GetSessionFileChanges(ctx context.Context, sessionUUID string) ([]SessionFileChangeInfo, error) {
	session, err := s.db.GetSessionByUUID(ctx, sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	changes, err := s.db.GetSessionFileChanges(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file changes: %w", err)
	}

	result := make([]SessionFileChangeInfo, len(changes))
	for i, fc := range changes {
		result[i] = SessionFileChangeInfo{
			FilePath:   fc.FilePath,
			ChangeType: fc.ChangeType,
			DetectedAt: fc.DetectedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return result, nil
}
