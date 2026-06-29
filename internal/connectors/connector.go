// Package connectors provides interfaces and types for external channel connectors.
package connectors

import (
	"context"
	"time"

	planviewer "github.com/Javier162380/claude-plan-viewer"
)

// TransmitPayload carries content to push to an external channel (e.g. Telegram).
type TransmitPayload struct {
	Title   string
	Content string
}

// SummaryPayload carries content to summarise via an LLM.
type SummaryPayload struct {
	Title   string
	Content string
}

// DiffVersion is a minimal version snapshot for diff analysis.
// Avoids importing the service layer into this package.
type DiffVersion struct {
	VersionNumber int64
	Content       string
	CreatedAt     time.Time
}

// DiffPayload carries two version snapshots for change analysis.
type DiffPayload struct {
	PlanName string
	From     DiffVersion
	To       DiffVersion
}

// ConnectorRequest is the single input for Execute.
// Exactly one payload field is non-nil, selected by Role.
type ConnectorRequest struct {
	Role     planviewer.ConnectorRole
	Transmit *TransmitPayload
	Summary  *SummaryPayload
	Diff     *DiffPayload
}

// ConnectorResult carries the role that produced it alongside the output,
// since result fields differ by role.
type ConnectorResult struct {
	Role      planviewer.ConnectorRole
	Text      *string // set by generative connectors (Ollama)
	MessageID *string // set by messaging connectors (Telegram)
}

// Connector defines the interface for external channel connectors.
//
//go:generate mockgen -package connectors_test -destination ./test/connector_stub.go . Connector
type Connector interface {
	// Name returns the unique identifier for this connector.
	Name() string

	// DisplayName returns a human-readable name for UI display.
	DisplayName() string

	// SupportedRoles returns the roles this connector can handle.
	SupportedRoles() []planviewer.ConnectorRole

	// Execute dispatches the request to the connector.
	Execute(ctx context.Context, req ConnectorRequest) (*ConnectorResult, error)

	// Validate checks if the connector is properly configured.
	Validate() error

	// RequiredSettings returns the list of setting keys this connector needs.
	RequiredSettings() []SettingDefinition
}

// SettingGetter provides access to connector settings.
type SettingGetter interface {
	GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error)
}

// ConfigurableConnector can load config from a setting getter.
type ConfigurableConnector interface {
	Connector
	LoadConfig(ctx context.Context, getter SettingGetter) error
}

// SettingDefinition describes a configuration setting for a connector.
type SettingDefinition struct {
	Key         string
	DisplayName string
	Description string
	Required    bool
	Sensitive   bool
}

// ConnectorStatus represents the status of a connector.
type ConnectorStatus struct {
	Name        string
	DisplayName string
	Role        *planviewer.ConnectorRole // nil if not assigned to any slot
	Configured  bool
}

// ConnectorRole re-exports the root package type so connector implementations
// don't need to import the root package directly.
type ConnectorRole = planviewer.ConnectorRole

const (
	ConnectorRoleTransmit = planviewer.ConnectorRoleTransmit
	ConnectorRoleSummary  = planviewer.ConnectorRoleSummary
	ConnectorRoleDiff     = planviewer.ConnectorRoleDiff
)
