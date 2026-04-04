package connectors

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/internal/secrets"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"
)

// Manager orchestrates connector operations.
type Manager struct {
	registry *Registry
	db       *repository.Queries
	secrets  secrets.Store
}

// NewManager creates a new connector manager.
func NewManager(registry *Registry, db *repository.Queries, secrets secrets.Store) *Manager {
	return &Manager{
		registry: registry,
		db:       db,
		secrets:  secrets,
	}
}

// GetEnabledConnector returns the currently enabled connector, if any.
func (m *Manager) GetEnabledConnector(ctx context.Context) (Connector, error) {
	row, err := m.db.GetEnabledConnector(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled connector: %w", err)
	}

	connector, ok := m.registry.Get(row.Name)
	if !ok {
		return nil, fmt.Errorf("enabled connector %q not found in registry", row.Name)
	}

	// Load config from secrets if supported
	if cfg, ok := connector.(ConfigurableConnector); ok {
		if err := cfg.LoadConfig(ctx, m.secrets); err != nil {
			return nil, fmt.Errorf("failed to load config for %q: %w", row.Name, err)
		}
	}

	return connector, nil
}

// EnableConnector enables a specific connector by name (disables all others).
func (m *Manager) EnableConnector(ctx context.Context, name string) error {
	if _, ok := m.registry.Get(name); !ok {
		return fmt.Errorf("connector %q not registered", name)
	}

	// First ensure the connector exists in DB
	connector, _ := m.registry.Get(name)
	err := m.db.UpsertConnector(ctx, repository.UpsertConnectorParams{
		Name:        name,
		DisplayName: connector.DisplayName(),
		Enabled:     false,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert connector: %w", err)
	}

	// Disable all connectors
	if err := m.db.DisableAllConnectors(ctx); err != nil {
		return fmt.Errorf("failed to disable connectors: %w", err)
	}

	// Enable the specified one
	if err := m.db.SetConnectorEnabled(ctx, name); err != nil {
		return fmt.Errorf("failed to enable connector: %w", err)
	}

	return nil
}

// DisableConnector disables all connectors.
func (m *Manager) DisableConnector(ctx context.Context) error {
	return m.db.DisableAllConnectors(ctx)
}

// SetConnectorSetting saves a setting for a specific connector.
func (m *Manager) SetConnectorSetting(ctx context.Context, connectorName, key, value string, isSecret bool) error {
	return m.db.UpsertConnectorSetting(ctx, repository.UpsertConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
		SettingValue:  value,
		IsSecret:      isSecret,
	})
}

// GetConnectorSetting retrieves a setting for a specific connector.
func (m *Manager) GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error) {
	setting, err := m.db.GetConnectorSetting(ctx, repository.GetConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
	})
	if errors.Is(err, sql.ErrNoRows) {
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
		return nil, fmt.Errorf("no connector enabled")
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

	enabledName := ""
	if row, err := m.db.GetEnabledConnector(ctx); err == nil {
		enabledName = row.Name
	}

	for i, c := range connectors {
		statuses[i] = ConnectorStatus{
			Name:        c.Name(),
			DisplayName: c.DisplayName(),
			Enabled:     c.Name() == enabledName,
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
		return fmt.Errorf("connector %q not registered", name)
	}

	return m.db.UpsertConnector(ctx, repository.UpsertConnectorParams{
		Name:        name,
		DisplayName: connector.DisplayName(),
		Enabled:     false,
	})
}

// GetConnectorRequiredSettings returns the required settings for a connector.
func (m *Manager) GetConnectorRequiredSettings(connectorName string) ([]SettingDefinition, error) {
	connector, ok := m.registry.Get(connectorName)
	if !ok {
		return nil, fmt.Errorf("connector %q not found", connectorName)
	}
	return connector.RequiredSettings(), nil
}
