// Package connectors provides interfaces and types for external channel connectors.
package connectors

import "context"

// SendResult contains the result of a send/generate operation.
type SendResult struct {
	Success   bool
	MessageID *string // non-nil for messaging connectors (e.g. Telegram)
	Error     error
	Response  *string // non-nil for generative connectors (e.g. Ollama, LM Studio)
}

// Connector defines the interface for external channel connectors.
//
//go:generate mockgen -package connectors_test -destination ./test/connector_stub.go . Connector
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
	Enabled     bool
	Configured  bool
}
