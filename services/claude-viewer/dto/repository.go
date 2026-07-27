package dto

import (
	"context"
	"time"
)

// Repository defines the interface for data persistence operations.
// Both SQLite and PostgreSQL implementations must satisfy this interface.
type Repository interface {
	// Plan operations
	CountPlans(ctx context.Context) (int64, error)
	GetPlanByFileName(ctx context.Context, fileName, syncSource string) (Plan, error)
	// InsertPlan inserts a new plan row and, while the transaction is open,
	// invokes writeFile with the newly assigned id so the caller can write
	// the id-keyed mirror file, then persists the path writeFile returns —
	// all atomically. If writeFile fails, the insert rolls back with it.
	InsertPlan(ctx context.Context, params InsertPlanParams, writeFile func(id int64) (filePath string, err error)) (int64, error)
	UpdatePlan(ctx context.Context, params UpdatePlanParams) error
	// UpdatePlanFilePath runs writeFile inside a transaction and only sets
	// file_path if it succeeds — same atomicity guarantee as InsertPlan.
	UpdatePlanFilePath(ctx context.Context, id int64, filePath string, writeFile func() error) error
	UpdatePlanVersionFilePath(ctx context.Context, id int64, filePath string, writeFile func() error) error
	InsertPlanWithTags(ctx context.Context, params InsertPlanWithTagsParams) error
	UpdatePlanWithTags(ctx context.Context, params UpdatePlanWithTagsParams) error
	DeletePlan(ctx context.Context, fileName, syncSource string) error
	RenamePlanFile(ctx context.Context, params RenamePlanFileParams, renameFiles func() error) error
	ListAllPlans(ctx context.Context, sortCol, sortDir string, wpm int) ([]PlanSummary, error)
	// ListAllPlansFull and ListPlanVersionsAll are used only by MigrateStorageLayout.
	ListAllPlansFull(ctx context.Context) ([]Plan, error)
	ListPlanVersionsAll(ctx context.Context, planID int64) ([]PlanVersion, error)
	ListAllPlansWithPagination(ctx context.Context, params PaginationParams) ([]PlanSummary, error)
	SearchPlans(ctx context.Context, params SearchParams) ([]PlanSummary, error)
	SearchPlansWithPagination(ctx context.Context, params SearchPaginationParams) ([]PlanSummary, error)
	SearchPlansWithTags(ctx context.Context, params SearchParams) ([]PlanSummary, error)

	// Plan version operations
	InsertPlanVersion(ctx context.Context, params InsertPlanVersionParams) error
	RestorePlanVersion(ctx context.Context, params RestorePlanVersionParams) error
	GetPlanVersionHistory(ctx context.Context, params VersionHistoryParams) ([]PlanVersion, error)
	GetPlanVersionByNumber(ctx context.Context, planID, versionNumber int64) (PlanVersion, error)
	GetLatestVersionNumber(ctx context.Context, planID int64) (int64, error)
	GetVersionCount(ctx context.Context, planID int64) (int64, error)
	DeleteVersionsOlderThan(ctx context.Context, params DeleteVersionsParams) error
	SearchVersionsByContent(ctx context.Context, params SearchVersionsParams) ([]PlanVersion, error)

	// Setting operations
	GetSettingByName(ctx context.Context, name string) (Setting, error)
	UpsertSetting(ctx context.Context, params UpsertSettingParams) error
	DeleteSetting(ctx context.Context, name string) error

	// Connector operations
	GetConnectorByName(ctx context.Context, name string) (Connector, error)
	ListConnectors(ctx context.Context) ([]Connector, error)
	GetConnectorForRole(ctx context.Context, role ConnectorRole) (string, bool, error)
	SetConnectorForRole(ctx context.Context, name string, role ConnectorRole) error
	ClearConnectorForRole(ctx context.Context, role ConnectorRole) error
	UpsertConnector(ctx context.Context, name, displayName string, enabled bool) error
	DeleteConnector(ctx context.Context, name string) error

	// Connector setting operations
	GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error)
	ListConnectorSettings(ctx context.Context, connectorName string) ([]ConnectorSetting, error)
	UpsertConnectorSetting(ctx context.Context, connectorName, key, value string, isSecret bool) error
	DeleteConnectorSetting(ctx context.Context, connectorName, key string) error
	DeleteAllConnectorSettings(ctx context.Context, connectorName string) error

	// Tag operations
	InsertTag(ctx context.Context, params InsertTagParams) (Tag, error)
	GetTagByName(ctx context.Context, name string) (Tag, error)
	GetTagByID(ctx context.Context, id int64) (Tag, error)
	ListAllTags(ctx context.Context) ([]Tag, error)
	UpdateTag(ctx context.Context, params UpdateTagParams) error
	DeleteTag(ctx context.Context, id int64) error

	// Comment operations
	InsertComment(ctx context.Context, params InsertCommentParams) (Comment, error)
	GetPlanComments(ctx context.Context, planID int64) ([]Comment, error)
	DeleteComment(ctx context.Context, id int64) error
	GetPlanCommentCounts(ctx context.Context) (map[int64]int, error)

	// Plan-Tag associations
	AddTagToPlan(ctx context.Context, planID, tagID int64, assignedAt time.Time) error
	RemoveTagFromPlan(ctx context.Context, planID, tagID int64) error
	RemoveAllTagsFromPlan(ctx context.Context, planID int64) error
	GetPlanTags(ctx context.Context, planID int64) ([]Tag, error)
	SetPlanTags(ctx context.Context, planID int64, tagIDs []int64, assignedAt time.Time) error
	GetTagPlanCounts(ctx context.Context) (map[string]int, error)
	GetUntaggedPlanCount(ctx context.Context) (int64, error)
	ListUntaggedPlans(ctx context.Context, sortCol, sortDir string, wpm int) ([]PlanSummary, error)
	ListPlansWithTags(ctx context.Context) ([]PlanSummary, error)
}
