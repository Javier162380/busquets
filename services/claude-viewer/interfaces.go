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

// UnifiedService combines all service interfaces for use by HTTP and TUI.
type UnifiedService interface {
	PlanService
	VersionService
	SettingsService
	SyncService
	ConnectorService
}

// Verify that *Service implements UnifiedService at compile time.
var _ UnifiedService = (*Service)(nil)
