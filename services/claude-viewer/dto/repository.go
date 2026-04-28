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
	GetPlanByFileName(ctx context.Context, fileName string) (Plan, error)
	GetPlanByID(ctx context.Context, id int64) (Plan, error)
	InsertPlan(ctx context.Context, params InsertPlanParams) error
	UpdatePlan(ctx context.Context, params UpdatePlanParams) error
	DeletePlan(ctx context.Context, fileName string) error
	ListAllPlans(ctx context.Context) ([]PlanSummary, error)
	ListAllPlansWithPagination(ctx context.Context, params PaginationParams) ([]PlanSummary, error)
	SearchPlans(ctx context.Context, params SearchParams) ([]PlanSummary, error)
	SearchPlansWithPagination(ctx context.Context, params SearchPaginationParams) ([]PlanSummary, error)
	SearchPlansWithTags(ctx context.Context, params SearchParams) ([]PlanSummary, error)

	// Plan version operations
	InsertPlanVersion(ctx context.Context, params InsertPlanVersionParams) error
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
	GetEnabledConnector(ctx context.Context) (Connector, error)
	UpsertConnector(ctx context.Context, params UpsertConnectorParams) error
	SetConnectorEnabled(ctx context.Context, name string) error
	DisableAllConnectors(ctx context.Context) error
	DeleteConnector(ctx context.Context, name string) error

	// Connector setting operations
	GetConnectorSetting(ctx context.Context, connectorName, key string) (ConnectorSetting, error)
	ListConnectorSettings(ctx context.Context, connectorName string) ([]ConnectorSetting, error)
	UpsertConnectorSetting(ctx context.Context, params UpsertConnectorSettingParams) error
	DeleteConnectorSetting(ctx context.Context, connectorName, key string) error
	DeleteAllConnectorSettings(ctx context.Context, connectorName string) error

	// Tag operations
	InsertTag(ctx context.Context, params InsertTagParams) (Tag, error)
	GetTagByName(ctx context.Context, name string) (Tag, error)
	GetTagByID(ctx context.Context, id int64) (Tag, error)
	ListAllTags(ctx context.Context) ([]Tag, error)
	UpdateTag(ctx context.Context, params UpdateTagParams) error
	DeleteTag(ctx context.Context, id int64) error

	// Plan-Tag associations
	AddTagToPlan(ctx context.Context, planID, tagID int64, assignedAt time.Time) error
	RemoveTagFromPlan(ctx context.Context, planID, tagID int64) error
	RemoveAllTagsFromPlan(ctx context.Context, planID int64) error
	GetPlanTags(ctx context.Context, planID int64) ([]Tag, error)
	SetPlanTags(ctx context.Context, planID int64, tagIDs []int64, assignedAt time.Time) error

	// Session operations
	InsertSession(ctx context.Context, params InsertSessionParams) (int64, error)
	UpdateSession(ctx context.Context, params UpdateSessionParams) error
	GetSessionByUUID(ctx context.Context, uuid string) (Session, error)
	GetSessionByID(ctx context.Context, id int64) (Session, error)
	ListAllSessions(ctx context.Context) ([]SessionSummary, error)
	ListSessionsByPlanID(ctx context.Context, planID int64) ([]SessionSummary, error)
	DeleteSession(ctx context.Context, id int64) error
	CountSessions(ctx context.Context) (int64, error)

	// Session message operations
	InsertSessionMessage(ctx context.Context, params InsertSessionMessageParams) (int64, error)
	GetSessionMessages(ctx context.Context, sessionID, limit, offset int64) ([]SessionMessage, error)
	GetSessionMessageCount(ctx context.Context, sessionID int64) (int64, error)
	DeleteSessionMessages(ctx context.Context, sessionID int64) error

	// Session file change operations
	InsertSessionFileChange(ctx context.Context, params InsertSessionFileChangeParams) error
	GetSessionFileChanges(ctx context.Context, sessionID int64) ([]SessionFileChange, error)
	DeleteSessionFileChanges(ctx context.Context, sessionID int64) error

	// Session todo operations
	InsertSessionTodo(ctx context.Context, params InsertSessionTodoParams) error
	UpdateSessionTodo(ctx context.Context, params UpdateSessionTodoParams) error
	GetSessionTodos(ctx context.Context, sessionID int64) ([]SessionTodo, error)
	DeleteSessionTodos(ctx context.Context, sessionID int64) error
}
