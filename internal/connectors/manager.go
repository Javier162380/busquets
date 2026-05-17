package connectors

import (
	"context"
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// Manager orchestrates connector operations.
type Manager struct {
	registry *Registry
	db       dto.Repository
}

// NewManager creates a new connector manager.
func NewManager(registry *Registry, db dto.Repository) *Manager {
	return &Manager{
		registry: registry,
		db:       db,
	}
}

// GetEnabledConnector returns the transmit connector, if any.
func (m *Manager) GetEnabledConnector(ctx context.Context) (Connector, error) {
	name, err := m.db.GetConnectorForRole(ctx, dto.ConnectorRoleTransmit)
	if dto.IsNotFound(err) || name == "" {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transmit connector: %w", err)
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
		return dto.ErrNotFound
	}

	if err := m.db.UpsertConnector(ctx, dto.UpsertConnectorParams{
		Name:        name,
		DisplayName: connector.DisplayName(),
		Enabled:     false,
	}); err != nil {
		return fmt.Errorf("failed to upsert connector: %w", err)
	}

	if err := m.db.SetConnectorForRole(ctx, name, dto.ConnectorRoleTransmit); err != nil {
		return fmt.Errorf("failed to set transmit connector: %w", err)
	}

	return nil
}

// DisableConnector clears the transmit connector slot.
func (m *Manager) DisableConnector(ctx context.Context) error {
	return m.db.ClearConnectorForRole(ctx, dto.ConnectorRoleTransmit)
}

// SetSummaryConnector assigns a connector to the summary slot.
func (m *Manager) SetSummaryConnector(ctx context.Context, name string) error {
	connector, ok := m.registry.Get(name)
	if !ok {
		return dto.ErrNotFound
	}

	if err := m.db.UpsertConnector(ctx, dto.UpsertConnectorParams{
		Name:        name,
		DisplayName: connector.DisplayName(),
		Enabled:     false,
	}); err != nil {
		return fmt.Errorf("failed to upsert connector: %w", err)
	}

	if err := m.db.SetConnectorForRole(ctx, name, dto.ConnectorRoleSummary); err != nil {
		return fmt.Errorf("failed to set summary connector: %w", err)
	}

	return nil
}

// SetConnectorSetting saves a setting for a specific connector.
func (m *Manager) SetConnectorSetting(ctx context.Context, connectorName, key, value string, isSecret bool) error {
	return m.db.UpsertConnectorSetting(ctx, dto.UpsertConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
		SettingValue:  value,
		IsSecret:      isSecret,
	})
}

// GetConnectorSetting retrieves a setting for a specific connector.
func (m *Manager) GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error) {
	setting, err := m.db.GetConnectorSetting(ctx, connectorName, key)
	if dto.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return setting.SettingValue, true, nil
}

// Send sends content through the enabled connector.
func (m *Manager) Send(ctx context.Context, title, content string) (*SendResult, error) {
	connector, err := m.GetEnabledConnector(ctx)
	if err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, dto.ErrNoConnectorEnabled
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

	transmitName, _ := m.db.GetConnectorForRole(ctx, dto.ConnectorRoleTransmit)
	summaryName, _ := m.db.GetConnectorForRole(ctx, dto.ConnectorRoleSummary)

	for i, c := range connectors {
		var role *dto.ConnectorRole
		switch c.Name() {
		case transmitName:
			r := dto.ConnectorRoleTransmit
			role = &r
		case summaryName:
			r := dto.ConnectorRoleSummary
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
		return dto.ErrNotFound
	}

	return m.db.UpsertConnector(ctx, dto.UpsertConnectorParams{
		Name:        name,
		DisplayName: connector.DisplayName(),
		Enabled:     false,
	})
}

// GenerateSummary invokes the summary connector and returns its generated text response.
func (m *Manager) GenerateSummary(ctx context.Context, title, content string) (string, error) {
	name, err := m.db.GetConnectorForRole(ctx, dto.ConnectorRoleSummary)
	if dto.IsNotFound(err) || name == "" {
		return "", dto.ErrNoSummarizerConfigured
	}
	if err != nil {
		return "", fmt.Errorf("failed to read summarizer setting: %w", err)
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
		return "", dto.ErrConnectorResponseEmpty
	}
	return *result.Response, nil
}

// GetConnectorRequiredSettings returns the required settings for a connector.
func (m *Manager) GetConnectorRequiredSettings(connectorName string) ([]SettingDefinition, error) {
	connector, ok := m.registry.Get(connectorName)
	if !ok {
		return nil, dto.ErrNotFound
	}
	return connector.RequiredSettings(), nil
}

// ValidateConnector validates a connector's configuration.
func (m *Manager) ValidateConnector(ctx context.Context, connectorName string) error {
	connector, ok := m.registry.Get(connectorName)
	if !ok {
		return dto.ErrNotFound
	}

	// Load config if configurable
	if cfg, ok := connector.(ConfigurableConnector); ok {
		if err := cfg.LoadConfig(ctx, m); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	return connector.Validate()
}
