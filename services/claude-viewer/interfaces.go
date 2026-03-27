package claudeviewer

import (
	"context"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"
)

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
	GetPlanVersionHistory(ctx context.Context, planName string, offset, limit int64) ([]repository.PlanVersion, error)
	RestorePlanVersion(ctx context.Context, planName string, versionNumber int64) error
}

// SettingsService defines settings operations.
type SettingsService interface {
	GetStringValue(ctx context.Context, variableName string) (string, bool, error)
	GetBooleanValue(ctx context.Context, variableName string) (bool, bool, error)
	GetNumberValue(ctx context.Context, variableName string) (float64, bool, error)
	GetDateTimeValue(ctx context.Context, variableName string) (time.Time, bool, error)
	SetSetting(ctx context.Context, varName, varType string, values SettingValues) error
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
