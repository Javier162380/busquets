// Package connectors provides interfaces and types for external channel connectors.
package connectors

import (
	"context"

	"github.com/Javier162380/claude-plan-viewer/internal/secrets"
)

// SendResult contains the result of a send operation.
type SendResult struct {
	Success   bool
	MessageID string
	Error     error
}

// Connector defines the interface for external channel connectors.
type Connector interface {
	// Name returns the unique identifier for this connector.
	Name() string

	// DisplayName returns a human-readable name for UI display.
	DisplayName() string

	// Send transmits content to the external channel.
	Send(ctx context.Context, title, content string) (*SendResult, error)

	// Validate checks if the connector is properly configured.
	Validate() error

	// RequiredSettings returns the list of setting keys this connector needs.
	RequiredSettings() []SettingDefinition
}

// ConfigurableConnector can load config from a secrets store.
type ConfigurableConnector interface {
	Connector
	LoadConfig(ctx context.Context, store secrets.Store) error
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
	Enabled     bool
	Configured  bool
}
