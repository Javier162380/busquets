package connectors

import (
	"context"
	"fmt"

	planviewer "github.com/Javier162380/claude-plan-viewer"
)

// Manager orchestrates connector operations.
type Manager struct {
	registry *Registry
	db       planviewer.Store
}

// NewManager creates a new connector manager.
func NewManager(registry *Registry, db planviewer.Store) *Manager {
	return &Manager{
		registry: registry,
		db:       db,
	}
}

// GetEnabledConnector returns the transmit connector, if any.
func (m *Manager) GetEnabledConnector(ctx context.Context) (Connector, error) {
	name, found, err := m.db.GetConnectorForRole(ctx, planviewer.ConnectorRoleTransmit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transmit connector: %w", err)
	}
	if !found || name == "" {
		return nil, nil
	}

	connector, ok := m.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("transmit connector %q not found in registry", name)
	}

	if cfg, ok := connector.(ConfigurableConnector); ok {
		if err := cfg.LoadConfig(ctx, m); err != nil {
			return nil, fmt.Errorf("failed to load config for %q: %w", name, err)
		}
	}

	return connector, nil
}

// EnableConnector sets a connector as the transmit connector.
func (m *Manager) EnableConnector(ctx context.Context, name string) error {
	connector, ok := m.registry.Get(name)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}

	if err := m.db.UpsertConnector(ctx, name, connector.DisplayName(), false); err != nil {
		return fmt.Errorf("failed to upsert connector: %w", err)
	}

	if err := m.db.SetConnectorForRole(ctx, name, planviewer.ConnectorRoleTransmit); err != nil {
		return fmt.Errorf("failed to set transmit connector: %w", err)
	}

	return nil
}

// DisableConnector clears the transmit connector slot.
func (m *Manager) DisableConnector(ctx context.Context) error {
	return m.db.ClearConnectorForRole(ctx, planviewer.ConnectorRoleTransmit)
}

// SetSummaryConnector assigns a connector to the summary slot.
func (m *Manager) SetSummaryConnector(ctx context.Context, name string) error {
	connector, ok := m.registry.Get(name)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}

	if err := m.db.UpsertConnector(ctx, name, connector.DisplayName(), false); err != nil {
		return fmt.Errorf("failed to upsert connector: %w", err)
	}

	if err := m.db.SetConnectorForRole(ctx, name, planviewer.ConnectorRoleSummary); err != nil {
		return fmt.Errorf("failed to set summary connector: %w", err)
	}

	return nil
}

// SetConnectorSetting saves a setting for a specific connector.
func (m *Manager) SetConnectorSetting(ctx context.Context, connectorName, key, value string, isSecret bool) error {
	return m.db.UpsertConnectorSetting(ctx, connectorName, key, value, isSecret)
}

// GetConnectorSetting retrieves a setting for a specific connector.
func (m *Manager) GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error) {
	return m.db.GetConnectorSetting(ctx, connectorName, key)
}

// Send sends content through the enabled connector.
func (m *Manager) Send(ctx context.Context, title, content string) (*SendResult, error) {
	connector, err := m.GetEnabledConnector(ctx)
	if err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, planviewer.ErrNoConnectorEnabled
	}

	if err := connector.Validate(); err != nil {
		return nil, fmt.Errorf("connector validation failed: %w", err)
	}

	return connector.Send(ctx, title, content)
}

// ListAvailable returns all registered connectors with their status.
func (m *Manager) ListAvailable(ctx context.Context) ([]ConnectorStatus, error) {
	connectors := m.registry.All()
	statuses := make([]ConnectorStatus, len(connectors))

	transmitName, _, err := m.db.GetConnectorForRole(ctx, planviewer.ConnectorRoleTransmit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transmit connector role: %w", err)
	}
	summaryName, _, err := m.db.GetConnectorForRole(ctx, planviewer.ConnectorRoleSummary)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary connector role: %w", err)
	}

	for i, c := range connectors {
		var role *planviewer.ConnectorRole
		switch c.Name() {
		case transmitName:
			r := planviewer.ConnectorRoleTransmit
			role = &r
		case summaryName:
			r := planviewer.ConnectorRoleSummary
			role = &r
		}
		statuses[i] = ConnectorStatus{
			Name:        c.Name(),
			DisplayName: c.DisplayName(),
			Role:        role,
			Configured:  m.isConfigured(ctx, c),
		}
	}
	return statuses, nil
}

// isConfigured checks if all required settings are present for a connector.
func (m *Manager) isConfigured(ctx context.Context, c Connector) bool {
	for _, def := range c.RequiredSettings() {
		if def.Required {
			_, exists, _ := m.GetConnectorSetting(ctx, c.Name(), def.Key)
			if !exists {
				return false
			}
		}
	}
	return true
}

// EnsureConnectorExists ensures a connector record exists in the database.
func (m *Manager) EnsureConnectorExists(ctx context.Context, name string) error {
	connector, ok := m.registry.Get(name)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}

	return m.db.UpsertConnector(ctx, name, connector.DisplayName(), false)
}

// GenerateSummary invokes the summary connector and returns its generated text response.
func (m *Manager) GenerateSummary(ctx context.Context, title, content string) (string, error) {
	name, found, err := m.db.GetConnectorForRole(ctx, planviewer.ConnectorRoleSummary)
	if err != nil {
		return "", fmt.Errorf("failed to read summarizer setting: %w", err)
	}
	if !found || name == "" {
		return "", planviewer.ErrNoSummarizerConfigured
	}
	connector, ok := m.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("summarizer connector %q not registered", name)
	}

	if cfg, ok := connector.(ConfigurableConnector); ok {
		if err := cfg.LoadConfig(ctx, m); err != nil {
			return "", fmt.Errorf("failed to load summarizer config: %w", err)
		}
	}

	if err := connector.Validate(); err != nil {
		return "", fmt.Errorf("summarizer not configured: %w", err)
	}

	result, err := connector.Send(ctx, title, content)
	if err != nil {
		return "", err
	}
	if result.Response == nil {
		return "", planviewer.ErrConnectorResponseEmpty
	}
	return *result.Response, nil
}

// GetConnectorRequiredSettings returns the required settings for a connector.
func (m *Manager) GetConnectorRequiredSettings(connectorName string) ([]SettingDefinition, error) {
	connector, ok := m.registry.Get(connectorName)
	if !ok {
		return nil, planviewer.ErrConnectorNotFound
	}
	return connector.RequiredSettings(), nil
}

// ValidateConnector validates a connector's configuration.
func (m *Manager) ValidateConnector(ctx context.Context, connectorName string) error {
	connector, ok := m.registry.Get(connectorName)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}

	// Load config if configurable
	if cfg, ok := connector.(ConfigurableConnector); ok {
		if err := cfg.LoadConfig(ctx, m); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	return connector.Validate()
}
