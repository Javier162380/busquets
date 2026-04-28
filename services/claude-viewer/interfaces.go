package claudeviewer

import "context"

// PlanService defines plan CRUD operations.
type PlanService interface {
	ListAllPlansWithReadingTime(ctx context.Context) ([]PlanSummary, error)
	GetPlanDetailByFileName(ctx context.Context, fileName string) (*PlanDetail, error)
	UpdatePlan(ctx context.Context, req UpdatePlanRequest) (*UpdatePlanResult, error)
	SearchPlansWithReadingTime(ctx context.Context, query string) ([]PlanSummary, error)
}

// VersionService defines version control operations.
type VersionService interface {
	SavePlanVersion(ctx context.Context, planName, content string) error
	GetPlanVersionHistory(ctx context.Context, planName string, offset, limit int64) ([]PlanVersionDetail, error)
	GetPlanVersion(ctx context.Context, planName string, versionNumber int64) (*PlanVersionDetail, error)
	RestorePlanVersion(ctx context.Context, planName string, versionNumber int64) error
}

// SettingsService defines settings operations.
type SettingsService interface {
	GetSetting(ctx context.Context, variableName string) (Setting, bool, error)
	SetSetting(ctx context.Context, varName string, values SettingValues) error
}

// SyncService defines synchronization operations.
type SyncService interface {
	SyncPlans(ctx context.Context) (int, error)
}

// ConnectorService defines connector operations.
type ConnectorService interface {
	SendToConnector(ctx context.Context, planFileName string) error
	GetEnabledConnector(ctx context.Context) (*ConnectorInfo, error)
	ListConnectors(ctx context.Context) ([]ConnectorInfo, error)
	EnableConnector(ctx context.Context, name string) error
	DisableConnector(ctx context.Context) error
	ConfigureConnector(ctx context.Context, connectorName, key, value string, isSecret bool) error
	GetConnectorSettings(ctx context.Context, connectorName string) ([]ConnectorSettingInfo, error)
}

// ConnectorInfo represents connector status.
type ConnectorInfo struct {
	Name        string
	DisplayName string
	Enabled     bool
	Configured  bool
}

// ConnectorSettingInfo represents a connector setting with its current value.
type ConnectorSettingInfo struct {
	Key         string
	DisplayName string
	Description string
	Value       string
	Required    bool
	Sensitive   bool
}

// SessionService defines session management operations.
type SessionService interface {
	// Discovery and import
	DiscoverSessions(ctx context.Context) (int, error)
	ImportSession(ctx context.Context, jsonlPath string) error

	// Session CRUD
	ListAllSessions(ctx context.Context) ([]SessionSummaryInfo, error)
	GetSessionDetail(ctx context.Context, sessionUUID string) (*SessionDetailInfo, error)
	CreateSessionFromPlan(ctx context.Context, planFileName, initialPrompt string) (*SessionInfo, error)
	CreateStandaloneSession(ctx context.Context, projectPath, initialMessage string) (*SessionInfo, error)
	DeleteSession(ctx context.Context, sessionUUID string) error

	// Communication
	SendMessage(ctx context.Context, sessionUUID, message string) error

	// Monitoring
	StartSessionWatch(ctx context.Context, sessionUUID string) error
	StopSessionWatch(ctx context.Context, sessionUUID string) error
	GetSessionWatchResultChannel() <-chan SessionWatchResult

	// File tracking
	GetSessionFileChanges(ctx context.Context, sessionUUID string) ([]SessionFileChangeInfo, error)
}

// SessionInfo represents session information.
type SessionInfo struct {
	SessionUUID   string
	ProjectName   string
	JSONLFilePath string
	Status        string
	PlanTitle     *string
}

// SessionSummaryInfo represents a session summary for listing.
type SessionSummaryInfo struct {
	ID            int64
	SessionUUID   string
	ProjectName   string
	Status        string
	MessageCount  int64
	LastMessageAt *string // Formatted timestamp
	Slug          *string
	PlanTitle     *string
}

// SessionDetailInfo represents a session with full details.
type SessionDetailInfo struct {
	Session        SessionInfo
	Messages       []SessionMessageInfo
	FileChanges    []SessionFileChangeInfo
	Todos          []SessionTodoInfo
	AssociatedPlan *PlanSummary
}

// SessionMessageInfo represents a message in a session.
type SessionMessageInfo struct {
	MessageUUID string
	MessageType string
	Role        *string
	Content     *string
	Timestamp   string // Formatted timestamp
}

// SessionFileChangeInfo represents a file change tracked during a session.
type SessionFileChangeInfo struct {
	FilePath   string
	ChangeType string
	DetectedAt string // Formatted timestamp
}

// SessionTodoInfo represents a todo item from a session.
type SessionTodoInfo struct {
	Content    string
	Status     string
	ActiveForm *string
}

// UnifiedService combines all service interfaces for use by HTTP and TUI.
type UnifiedService interface {
	PlanService
	VersionService
	SettingsService
	SyncService
	ConnectorService
	SessionService
}

// Verify that *Service implements UnifiedService at compile time.
var _ UnifiedService = (*Service)(nil)
