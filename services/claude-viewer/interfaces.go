package claudeviewer

import (
	"context"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// PlanService defines plan CRUD operations.
type PlanService interface {
	ListAllPlansWithReadingTime(ctx context.Context) ([]PlanSummary, error)
	GetPlanDetailByFileName(ctx context.Context, fileName string) (*PlanDetail, error)
	UpdatePlan(ctx context.Context, req UpdatePlanRequest) (*UpdatePlanResult, error)
	SearchPlansWithReadingTime(ctx context.Context, query string) ([]PlanSummary, error)
	SearchPlansWithTags(ctx context.Context, query string, tags []string, matchAll bool) ([]PlanSummary, error)
	DeleteTag(ctx context.Context, id int64) error
	SetPlanTags(ctx context.Context, fileName string, tagNames []string) error
	GetAllTags(ctx context.Context) ([]dto.Tag, error)
	GetPlanTags(ctx context.Context, fileName string) ([]dto.Tag, error)
	ListUntaggedPlansWithReadingTime(ctx context.Context) ([]PlanSummary, error)
	GetTagPlanCounts(ctx context.Context) (map[string]int, error)
	GetUntaggedPlanCount(ctx context.Context) (int64, error)
	CreateTag(ctx context.Context, name string, description, color *string) (dto.Tag, error)
	SearchVersions(ctx context.Context, planName, query string) ([]PlanVersionDetail, error)
	BuildTagPlanMap(ctx context.Context) (map[string][]PlanSummary, error)
	SavePlanLocal(ctx context.Context, req UpdatePlanRequest) (*UpdatePlanResult, error)
	SearchPlansWithPaginationAndReadingTime(ctx context.Context, query string, limit, offset int64) ([]PlanSummary, error)
	GetPlanByFileName(ctx context.Context, fileName string) (*dto.Plan, error)
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
	RSyncPlans(ctx context.Context) (int, error)
	DumpPlans(ctx context.Context) (int, error)
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
	ValidateConnector(ctx context.Context, connectorName string) error
}

// WatchService defines watcher operations.
type WatchService interface {
	StartWatchMode(ctx context.Context, intervalSeconds float64) error
	StopWatchMode(ctx context.Context) error
	GetWatchResultChannel() <-chan WatchResult
	IsWatchModeRunning() bool
	UpdateWatchInterval(intervalSeconds float64)
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
	WatchService
}

// Verify that *Service implements UnifiedService at compile time.
var _ UnifiedService = (*Service)(nil)
