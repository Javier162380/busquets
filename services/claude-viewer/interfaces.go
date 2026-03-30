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

// UnifiedService combines all service interfaces for use by HTTP and TUI.
type UnifiedService interface {
	PlanService
	VersionService
	SettingsService
	SyncService
}

// Verify that *Service implements UnifiedService at compile time.
var _ UnifiedService = (*Service)(nil)
