package dto

import "context"

// Repository defines the interface for data persistence operations.
// Both SQLite and PostgreSQL implementations must satisfy this interface.
type Repository interface {
	// Plan operations
	CountPlans(ctx context.Context) (int64, error)
	GetPlanByFileName(ctx context.Context, fileName string) (Plan, error)
	InsertPlan(ctx context.Context, params InsertPlanParams) error
	UpdatePlan(ctx context.Context, params UpdatePlanParams) error
	DeletePlan(ctx context.Context, fileName string) error
	ListAllPlans(ctx context.Context) ([]PlanSummary, error)
	ListAllPlansWithPagination(ctx context.Context, params PaginationParams) ([]PlanSummary, error)
	SearchPlans(ctx context.Context, params SearchParams) ([]PlanSummary, error)
	SearchPlansWithPagination(ctx context.Context, params SearchPaginationParams) ([]PlanSummary, error)

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
}
